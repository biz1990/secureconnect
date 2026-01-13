package middleware

import (
	"strconv"
	"time"

	"secureconnect-backend/pkg/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMiddleware is a Gin middleware that records HTTP metrics
type PrometheusMiddleware struct {
	metrics *metrics.Metrics
}

// NewPrometheusMiddleware creates a new Prometheus middleware
func NewPrometheusMiddleware(m *metrics.Metrics) *PrometheusMiddleware {
	return &PrometheusMiddleware{
		metrics: m,
	}
}

// Handler returns the Gin middleware handler
func (p *PrometheusMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Increment in-flight requests
		p.metrics.IncrementHTTPRequestsInFlight()
		defer p.metrics.DecrementHTTPRequestsInFlight()

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Record metrics
		p.metrics.RecordHTTPRequest(
			c.Request.Method,
			c.FullPath(),
			c.Writer.Status(),
			duration,
		)
	}
}

// MetricsHandler returns an HTTP handler for Prometheus metrics endpoint
func MetricsHandler(m *metrics.Metrics) gin.HandlerFunc {
	// Use promhttp.Handler to expose metrics
	handler := promhttp.Handler()

	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// GetMetricsPath returns the path for the metrics endpoint
func GetMetricsPath() string {
	return "/metrics"
}

// GetMetricsLabel returns a label for the metrics
func GetMetricsLabel(serviceName string) prometheus.Labels {
	return prometheus.Labels{"service": serviceName}
}

// HTTPStatusToLabel converts HTTP status code to label
func HTTPStatusToLabel(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return strconv.Itoa(statusCode)
	}
}
