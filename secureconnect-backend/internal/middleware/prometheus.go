package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
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
// This handler always returns HTTP 200 if the process is alive, even if metrics collection fails
func MetricsHandler(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add panic recovery to ensure metrics endpoint always returns HTTP 200 if process is alive
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace for debugging
				log.Printf("PANIC in metrics handler: %v\nStack:\n%s", r, debug.Stack())
				// Return HTTP 200 even on panic to indicate the process is alive
				c.JSON(http.StatusOK, gin.H{
					"status": "metrics_collection_error",
					"error":  fmt.Sprintf("%v", r),
				})
				c.Abort()
			}
		}()

		// Check if metrics are initialized
		if m == nil {
			log.Println("Metrics instance is nil")
			c.JSON(http.StatusOK, gin.H{
				"status": "metrics_not_initialized",
				"error":  "metrics instance is nil",
			})
			return
		}

		// Check if registry is initialized
		registry := m.GetRegistry()
		if registry == nil {
			log.Println("Metrics registry is nil")
			c.JSON(http.StatusOK, gin.H{
				"status": "registry_not_initialized",
				"error":  "metrics registry is nil",
			})
			return
		}

		// Set proper content type for Prometheus metrics
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Create handler for the custom registry
		handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})

		// Serve metrics - this will return HTTP 200 with metrics in Prometheus format
		// Use defer to ensure we catch any errors during metric serving
		defer func() {
			if err := recover(); err != nil {
				// Already handled by outer defer, but ensure HTTP 200
				log.Printf("Recovering from metrics serve panic: %v", err)
			}
		}()

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
