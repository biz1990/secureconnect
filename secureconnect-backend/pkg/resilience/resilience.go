package resilience

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open"
	CircuitBreakerOpen     CircuitBreakerState = "open"
)

// MinIOResilience wraps MinIO operations with resilience patterns
type MinIOResilience struct {
	mu                  sync.RWMutex
	state               CircuitBreakerState
	consecutiveFailures int
	lastFailureTime     time.Time
	halfOpenAttempts    int
	metrics             *minioMetrics
}

// minioMetrics tracks MinIO operation metrics
type minioMetrics struct {
	requestsTotal       *prometheus.CounterVec
	errorsTotal         *prometheus.CounterVec
	circuitBreakerState prometheus.Gauge
}

var (
	minioMetricsInstance *minioMetrics
	minioMetricsOnce     sync.Once
)

// init registers MinIO metrics with Prometheus
func init() {
	minioMetricsOnce.Do(func() {
		minioMetricsInstance = &minioMetrics{
			requestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "minio_requests_total",
					Help: "Total number of MinIO requests",
				},
				[]string{"operation", "status"},
			),
			errorsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "minio_errors_total",
					Help: "Total number of MinIO errors",
				},
				[]string{"operation", "error_type"},
			),
			circuitBreakerState: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "minio_circuit_breaker_state",
				Help: "State of MinIO circuit breaker (0=closed, 1=half_open, 2=open)",
			}),
		}
		prometheus.MustRegister(minioMetricsInstance.requestsTotal)
		prometheus.MustRegister(minioMetricsInstance.errorsTotal)
		prometheus.MustRegister(minioMetricsInstance.circuitBreakerState)
	})
}

// NewMinIOResilience creates a new MinIO resilience wrapper
func NewMinIOResilience() *MinIOResilience {
	return &MinIOResilience{
		state:               CircuitBreakerClosed,
		consecutiveFailures: 0,
		lastFailureTime:     time.Time{},
		halfOpenAttempts:    0,
		metrics:             minioMetricsInstance,
	}
}

// Execute runs an operation with retry, timeout, and circuit breaker
func (r *MinIOResilience) Execute(
	ctx context.Context,
	operation string,
	fn func() error,
) error {
	// Apply timeout context (max 10s)
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var lastErr error
	var attempts int
	initialInterval := 100 * time.Millisecond
	maxInterval := 5 * time.Second
	maxElapsedTime := 30 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxElapsedTime {
		attempts++

		// Check circuit breaker state
		r.mu.RLock()
		state := r.state
		halfOpenAttempts := r.halfOpenAttempts
		r.mu.RUnlock()

		// If circuit is open, reject immediately
		if state == CircuitBreakerOpen {
			logger.Error("MinIO circuit breaker is OPEN - requests blocked",
				zap.String("operation", operation),
			)
			r.metrics.requestsTotal.WithLabelValues(operation, "circuit_breaker_open").Inc()
			return fmt.Errorf("storage service temporarily unavailable due to repeated failures (circuit breaker open)")
		}

		// If circuit is half-open, allow limited requests
		if state == CircuitBreakerHalfOpen {
			halfOpenAttempts++
			if halfOpenAttempts > 3 {
				// Too many half-open attempts, close circuit
				r.mu.Lock()
				r.state = CircuitBreakerClosed
				r.consecutiveFailures = 0
				r.halfOpenAttempts = 0
				r.lastFailureTime = time.Time{}
				r.mu.Unlock()
				logger.Info("MinIO circuit breaker CLOSED - recovered from half-open state",
					zap.String("operation", operation),
				)
				r.metrics.circuitBreakerState.Set(0)
			} else {
				logger.Warn("MinIO circuit breaker HALF-OPEN - allowing request",
					zap.String("operation", operation),
					zap.Int("attempt", halfOpenAttempts),
				)
			}
		}

		// Log retry attempt
		if attempts > 1 {
			logger.Warn("MinIO operation retry",
				zap.String("operation", operation),
				zap.Int("attempt", attempts),
				zap.Error(lastErr),
			)
		}

		// Execute operation
		err := fn()
		lastErr = err

		if err == nil {
			// Success - reset circuit breaker state
			r.mu.Lock()
			if r.state != CircuitBreakerClosed {
				r.state = CircuitBreakerClosed
				r.consecutiveFailures = 0
				r.halfOpenAttempts = 0
				r.lastFailureTime = time.Time{}
				r.metrics.circuitBreakerState.Set(0)
			}
			r.mu.Unlock()

			// Record success metric
			r.metrics.requestsTotal.WithLabelValues(operation, "success").Inc()

			logger.Info("MinIO operation succeeded",
				zap.String("operation", operation),
				zap.Int("attempts", attempts),
			)
			return nil
		}

		// Failure - track consecutive failures
		r.mu.Lock()
		r.consecutiveFailures++
		r.lastFailureTime = time.Now()

		// Record error metric
		r.metrics.errorsTotal.WithLabelValues(operation, classifyError(err)).Inc()
		r.metrics.requestsTotal.WithLabelValues(operation, "failure").Inc()

		// Open circuit after 3 consecutive failures
		if r.consecutiveFailures >= 3 {
			r.state = CircuitBreakerOpen
			r.metrics.circuitBreakerState.Set(2)
			logger.Error("MinIO circuit breaker OPEN - too many consecutive failures",
				zap.String("operation", operation),
				zap.Int("consecutive_failures", r.consecutiveFailures),
			)
		}

		// Half-open after 10 seconds
		if r.consecutiveFailures > 0 && time.Since(r.lastFailureTime) > 10*time.Second {
			r.state = CircuitBreakerHalfOpen
			r.halfOpenAttempts = 0
			r.metrics.circuitBreakerState.Set(1)
			logger.Warn("MinIO circuit breaker HALF-OPEN - cooling down period",
				zap.String("operation", operation),
				zap.Duration("time_since_last_failure", time.Since(r.lastFailureTime)),
			)
		}
		r.mu.Unlock()

		// Backoff before next retry
		backoff := time.Duration(float64(attempts) * float64(initialInterval))
		if backoff > maxInterval {
			backoff = maxInterval
		}

		logger.Info("MinIO operation failed, backing off",
			zap.String("operation", operation),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		// Wait for backoff period
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("MinIO operation timed out after 10s")
		case <-time.After(backoff):
			// Continue to next retry
		}
	}

	return fmt.Errorf("MinIO operation failed after %d attempts: %w", attempts, lastErr)
}

// GetCircuitBreakerState returns the current circuit breaker state
func (r *MinIOResilience) GetCircuitBreakerState() CircuitBreakerState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// classifyError classifies errors for better metrics
func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	// Check for common error types
	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded"):
		return "timeout"
	case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "network unreachable"):
		return "network"
	case strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "dns"):
		return "dns"
	case strings.Contains(errMsg, "bucket not found") || strings.Contains(errMsg, "not found"):
		return "not_found"
	case strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "access denied"):
		return "permission"
	case strings.Contains(errMsg, "circuit breaker"):
		return "circuit_breaker"
	default:
		return "unknown"
	}
}
