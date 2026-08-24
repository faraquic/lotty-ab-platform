package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appleboy/gin-jwt/v3"
	"github.com/appleboy/gin-jwt/v3/store"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
)

const (
	Realm       = "lotty-panel"
	IdentityKey = "id"
	KeyPrefix   = "lotty:auth:jwt:"
)

type Middleware struct {
	middleware *jwt.GinJWTMiddleware
	svc        *Service
	log        *zap.Logger
}

func NewMiddleware(cfg *config.Config, svc *Service, log *zap.Logger) (*Middleware, error) {
	middleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:         Realm,
		Key:           []byte(cfg.Auth.JWT.SecretKey),
		Timeout:       cfg.Auth.JWT.TTL,
		IdentityKey:   IdentityKey,
		TokenLookup:   "header: Authorization: Bearer",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,

		// tokens are minted by our /login handler (sub = user id),
		// so only validation is delegated to gin-jwt
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			id, _ := strconv.ParseInt(claims["sub"].(string), 10, 64)
			return id
		},

		Unauthorized: func(c *gin.Context, code int, message string) {
			api.Error(c.Writer, code, "UNAUTHORIZED", message)
		},

		RedisConfig: &store.RedisConfig{
			Addr:      cfg.Database.Redis.Address,
			Password:  cfg.Database.Redis.Password,
			DB:        cfg.Database.Redis.DB,
			PoolSize:  cfg.Database.Redis.PoolSize,
			KeyPrefix: KeyPrefix,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Middleware{middleware: middleware, svc: svc, log: log}, nil
}

// Handler verifies the Bearer token via gin-jwt and additionally requires
// the session to be present in Redis (revocation support).
func (m *Middleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.svc.SessionActive(c.Request.Context(), bearerToken(c.Request.Header.Get("Authorization"))) {
			api.Error(c.Writer, http.StatusUnauthorized, "UNAUTHORIZED", "token revoked")
			c.Abort()
			return
		}

		m.middleware.MiddlewareFunc()(c)
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return header[len(prefix):]
	}

	return ""
}
