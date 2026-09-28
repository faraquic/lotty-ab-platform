package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"job", "method", "path", "code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"job", "method", "path"},
	)

	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes.",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"job", "method", "path"},
	)

	SnapshotAge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "snapshot_age_seconds",
			Help: "Age of the current snapshot in seconds.",
		},
	)

	EventsAccepted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "analytics_events_accepted_total",
			Help: "Total number of accepted events.",
		},
		[]string{"job"},
	)

	EventsRejected = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "analytics_events_rejected_total",
			Help: "Total number of rejected events.",
		},
		[]string{"job"},
	)

	EventsDuplicates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "analytics_events_duplicates_total",
			Help: "Total number of duplicate events.",
		},
		[]string{"job"},
	)

	ClickhouseInserts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clickhouse_inserts_total",
			Help: "Total number of ClickHouse inserts.",
		},
		[]string{"job", "table"},
	)

	ClickhouseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "clickhouse_query_duration_seconds",
			Help:    "ClickHouse query duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"job", "query_type"},
	)
)

func Middleware(job string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		code := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		HTTPRequestsTotal.WithLabelValues(job, c.Request.Method, path, code).Inc()
		HTTPRequestDuration.WithLabelValues(job, c.Request.Method, path).Observe(duration)
		HTTPResponseSize.WithLabelValues(job, c.Request.Method, path).Observe(float64(c.Writer.Size()))
	}
}

func MiddlewareFiber(job string) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		code := strconv.Itoa(c.Response().StatusCode())
		path := c.Route().Path
		if path == "" {
			path = "unknown"
		}

		HTTPRequestsTotal.WithLabelValues(job, c.Method(), path, code).Inc()
		HTTPRequestDuration.WithLabelValues(job, c.Method(), path).Observe(duration)
		HTTPResponseSize.WithLabelValues(job, c.Method(), path).Observe(float64(len(c.Response().Body())))

		return err
	}
}
