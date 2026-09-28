package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/redis/rueidis"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
)

const (
	IdempotencyKeyHeader     = "Idempotency-Key"
	IdempotentReplayedHeader = "Idempotent-Replayed"
)

const (
	idempotencyTTL           = 24 * time.Hour
	idempotencyKeyPrefix     = "labp:panel:idempotency:"
	idempotencyMaxKeyLen     = 256
	idempotencyMaxBodyBytes  = 1 << 20
	idempotencyRedisTimeout  = 1 * time.Second
	idempotencyJSONContent   = "application/json"
	idempotencyAnonymousUser = "anon"
)

var errIdempotencyNotFound = errors.New("idempotency record not found")

type idempotencyRecord struct {
	RequestHash string `json:"request_hash"`
	Status      int    `json:"status"`
	Body        []byte `json:"body"`
	ContentType string `json:"content_type,omitempty"`
}

type idempotencyBackend interface {
	get(ctx context.Context, key string) (string, error)
	setNX(ctx context.Context, key, value string, ttl time.Duration) error
}

type redisIdempotencyBackend struct {
	client *rueidis.Client
}

func (b *redisIdempotencyBackend) get(ctx context.Context, key string) (string, error) {
	raw, err := (*b.client).Do(ctx, (*b.client).B().Get().Key(key).Build()).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return "", errIdempotencyNotFound
		}
		return "", err
	}
	return raw, nil
}

func (b *redisIdempotencyBackend) setNX(ctx context.Context, key, value string, ttl time.Duration) error {
	err := (*b.client).Do(ctx, (*b.client).B().Set().Key(key).Value(value).Nx().Ex(ttl).Build()).Error()
	if err != nil && rueidis.IsRedisNil(err) {
		return nil
	}
	return err
}

type memoryIdempotencyBackend struct {
	mu      sync.Mutex
	records map[string]memoryIdempotencyEntry
}

type memoryIdempotencyEntry struct {
	value     string
	expiresAt time.Time
}

func newMemoryIdempotencyBackend() *memoryIdempotencyBackend {
	return &memoryIdempotencyBackend{records: make(map[string]memoryIdempotencyEntry)}
}

func (b *memoryIdempotencyBackend) get(_ context.Context, key string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.records[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", errIdempotencyNotFound
	}
	return entry.value, nil
}

func (b *memoryIdempotencyBackend) setNX(_ context.Context, key, value string, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if entry, ok := b.records[key]; ok && time.Now().Before(entry.expiresAt) {
		return nil
	}
	b.records[key] = memoryIdempotencyEntry{value: value, expiresAt: time.Now().Add(ttl)}
	return nil
}

func IdempotencyGin(redis *rueidis.Client, log *zap.Logger) gin.HandlerFunc {
	if log == nil {
		log = zap.NewNop()
	}
	var backend idempotencyBackend
	if redis != nil {
		backend = &redisIdempotencyBackend{client: redis}
	}
	return newIdempotencyHandler(backend, log)
}

func newIdempotencyHandler(backend idempotencyBackend, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
		default:
			c.Next()
			return
		}

		key := strings.TrimSpace(c.GetHeader(IdempotencyKeyHeader))
		if key == "" {
			c.Next()
			return
		}
		if len(key) > idempotencyMaxKeyLen {
			api.Error(c.Writer, http.StatusBadRequest, api.BadRequest, "idempotency key is too long")
			c.Abort()
			return
		}
		if backend == nil {
			c.Next()
			return
		}

		if contentType := c.ContentType(); contentType != "" && contentType != idempotencyJSONContent {
			c.Next()
			return
		}

		body, tooLarge := peekIdempotencyBody(c)
		if tooLarge {
			c.Next()
			return
		}

		sum := sha256.Sum256(body)
		bodyHash := hex.EncodeToString(sum[:])
		redisKey := idempotencyRedisKey(CallerIDGin(c), c.Request.Method, idempotencyRoute(c), key)

		ctx, cancel := context.WithTimeout(c.Request.Context(), idempotencyRedisTimeout)
		raw, err := backend.get(ctx, redisKey)
		cancel()
		if err == nil {
			var rec idempotencyRecord
			if uerr := json.Unmarshal([]byte(raw), &rec); uerr == nil && rec.Status != 0 {
				if rec.RequestHash != bodyHash {
					logger.SetErrorType(c, logger.ErrorTypeConflict)
					api.Error(c.Writer, http.StatusConflict, api.Conflict, "idempotency key already used with a different request body")
					c.Abort()
					return
				}
				if rec.ContentType != "" {
					c.Writer.Header().Set("Content-Type", rec.ContentType)
				}
				c.Writer.Header().Set(IdempotentReplayedHeader, "true")
				c.Writer.WriteHeader(rec.Status)
				_, _ = c.Writer.Write(rec.Body)
				c.Abort()
				return
			}
		} else if !errors.Is(err, errIdempotencyNotFound) {
			log.Warn(
				"idempotency lookup failed; proceeding without deduplication",
				zap.String(logger.FieldCacheOperation, "idempotency_get"),
				zap.String(logger.FieldCacheStatus, "error"),
				zap.Error(err),
			)
			c.Next()
			return
		}

		capture := &idempotencyCaptureWriter{ResponseWriter: c.Writer}
		c.Writer = capture
		c.Next()

		status := capture.status
		if status == 0 || status < 200 || status > 299 {
			return
		}
		if capture.body.Len() > idempotencyMaxBodyBytes {
			return
		}

		rec := idempotencyRecord{
			RequestHash: bodyHash,
			Status:      status,
			Body:        append([]byte(nil), capture.body.Bytes()...),
			ContentType: capture.Header().Get("Content-Type"),
		}
		payload, merr := json.Marshal(rec)
		if merr != nil {
			log.Warn(
				"idempotency store skipped; marshal failed",
				zap.String(logger.FieldCacheOperation, "idempotency_set"),
				zap.String(logger.FieldCacheStatus, "marshal_error"),
				zap.Error(merr),
			)
			return
		}

		storeCtx, storeCancel := context.WithTimeout(context.Background(), idempotencyRedisTimeout)
		defer storeCancel()
		if serr := backend.setNX(storeCtx, redisKey, string(payload), idempotencyTTL); serr != nil {
			log.Warn(
				"idempotency store failed",
				zap.String(logger.FieldCacheOperation, "idempotency_set"),
				zap.String(logger.FieldCacheStatus, "error"),
				zap.Error(serr),
			)
		}
	}
}

func idempotencyRedisKey(caller, method, route, key string) string {
	if caller == "" {
		caller = idempotencyAnonymousUser
	}
	return idempotencyKeyPrefix + caller + ":" + method + ":" + route + ":" + key
}

func idempotencyRoute(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return route
	}
	return c.Request.URL.Path
}

func peekIdempotencyBody(c *gin.Context) ([]byte, bool) {
	if c.Request.Body == nil {
		return nil, false
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, idempotencyMaxBodyBytes+1))
	if err != nil {
		c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), c.Request.Body))
		return nil, true
	}
	if len(raw) > idempotencyMaxBodyBytes {
		c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), c.Request.Body))
		return nil, true
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	return raw, false
}

type idempotencyCaptureWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *idempotencyCaptureWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *idempotencyCaptureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *idempotencyCaptureWriter) WriteString(s string) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
