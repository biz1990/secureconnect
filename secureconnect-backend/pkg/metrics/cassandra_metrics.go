package metrics

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Cassandra metrics for monitoring query performance and reliability
var (
	// Query metrics
	CassandraQueryTimeoutTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_query_timeout_total",
		Help: "Total number of Cassandra query timeouts",
	}, []string{"operation", "table"})

	CassandraQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cassandra_query_duration_seconds",
		Help:    "Cassandra query latency in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"operation", "table"})

	CassandraQueryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_query_total",
		Help: "Total number of Cassandra queries executed",
	}, []string{"operation", "table", "status"})

	// Retry metrics
	CassandraQueryRetryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_query_retry_total",
		Help: "Total number of Cassandra query retries",
	}, []string{"operation", "table", "reason"})

	CassandraQueryRetryExhaustedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_query_retry_exhausted_total",
		Help: "Total number of Cassandra queries that exhausted all retries",
	}, []string{"operation", "table"})

	// Connection metrics
	CassandraConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cassandra_connections_active",
		Help: "Current number of active Cassandra connections",
	})

	CassandraConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cassandra_connections_idle",
		Help: "Current number of idle Cassandra connections",
	})

	// Error metrics
	CassandraQueryErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_query_error_total",
		Help: "Total number of Cassandra query errors",
	}, []string{"operation", "table", "error_type"})

	CassandraWriteErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_write_error_total",
		Help: "Total number of Cassandra write errors",
	}, []string{"table", "error_type"})

	CassandraReadErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cassandra_read_error_total",
		Help: "Total number of Cassandra read errors",
	}, []string{"table", "error_type"})

	// CockroachDB connection pool metrics
	DBConnectionsInUse = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_in_use",
		Help: "Current number of database connections in use",
	})

	DBConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_idle",
		Help: "Current number of idle database connections",
	})

	DBConnectionAcquireTimeoutTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "db_connection_acquire_timeout_total",
		Help: "Total number of database connection acquisition timeouts",
	})

	DBConnectionAcquireTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "db_connection_acquire_total",
		Help: "Total number of database connection acquisitions",
	})

	DBConnectionAcquireDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "db_connection_acquire_duration_seconds",
		Help:    "Database connection acquisition latency in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	// Request timeout metrics
	RequestTimeoutTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "request_timeout_total",
		Help: "Total number of request timeouts",
	})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"method", "path", "status"})

	RequestTimeoutDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_timeout_duration_seconds",
		Help:    "Request timeout duration in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"method", "path"})

	RequestInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "request_in_flight",
		Help: "Current number of in-flight requests",
	})

	// Redis fallback metrics
	RedisFallbackHitTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "redis_fallback_hits_total",
		Help: "Total number of Redis fallback cache hits",
	})

	RedisUnavailableTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "redis_unavailable_total",
		Help: "Total number of times Redis was unavailable",
	})

	RedisAvailableGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "redis_available",
		Help: "Whether Redis is available (1) or unavailable (0)",
	})
)

// requestInFlightCount tracks in-flight requests atomically
// This allows us to both update the Prometheus gauge AND read the value
var requestInFlightCount int64

// RecordCassandraQueryTimeout records a Cassandra query timeout
func RecordCassandraQueryTimeout(operation, table string) {
	CassandraQueryTimeoutTotal.WithLabelValues(operation, table).Inc()
}

// RecordCassandraQueryDuration records the duration of a Cassandra query
func RecordCassandraQueryDuration(operation, table string, duration float64) {
	CassandraQueryDuration.WithLabelValues(operation, table).Observe(duration)
}

// RecordCassandraQuery records a Cassandra query execution
func RecordCassandraQuery(operation, table, status string) {
	CassandraQueryTotal.WithLabelValues(operation, table, status).Inc()
}

// RecordCassandraQueryRetry records a Cassandra query retry
func RecordCassandraQueryRetry(operation, table, reason string) {
	CassandraQueryRetryTotal.WithLabelValues(operation, table, reason).Inc()
}

// RecordCassandraQueryRetryExhausted records when all retries are exhausted
func RecordCassandraQueryRetryExhausted(operation, table string) {
	CassandraQueryRetryExhaustedTotal.WithLabelValues(operation, table).Inc()
}

// SetCassandraConnectionsActive sets the number of active Cassandra connections
func SetCassandraConnectionsActive(count int) {
	CassandraConnectionsActive.Set(float64(count))
}

// SetCassandraConnectionsIdle sets the number of idle Cassandra connections
func SetCassandraConnectionsIdle(count int) {
	CassandraConnectionsIdle.Set(float64(count))
}

// RecordCassandraQueryError records a Cassandra query error
func RecordCassandraQueryError(operation, table, errorType string) {
	CassandraQueryErrorTotal.WithLabelValues(operation, table, errorType).Inc()
}

// RecordCassandraWriteError records a Cassandra write error
func RecordCassandraWriteError(table, errorType string) {
	CassandraWriteErrorTotal.WithLabelValues(table, errorType).Inc()
}

// RecordCassandraReadError records a Cassandra read error
func RecordCassandraReadError(table, errorType string) {
	CassandraReadErrorTotal.WithLabelValues(table, errorType).Inc()
}

// RecordDBConnectionsInUse sets the number of database connections in use
func RecordDBConnectionsInUse(count int) {
	DBConnectionsInUse.Set(float64(count))
}

// RecordDBConnectionsIdle sets the number of idle database connections
func RecordDBConnectionsIdle(count int) {
	DBConnectionsIdle.Set(float64(count))
}

// RecordDBConnectionAcquireTimeout records a database connection acquisition timeout
func RecordDBConnectionAcquireTimeout() {
	DBConnectionAcquireTimeoutTotal.Inc()
}

// RecordDBConnectionAcquire records a database connection acquisition
func RecordDBConnectionAcquire() {
	DBConnectionAcquireTotal.Inc()
}

// RecordDBConnectionAcquireDuration records database connection acquisition latency
func RecordDBConnectionAcquireDuration(duration float64) {
	DBConnectionAcquireDuration.Observe(duration)
}

// RecordRequestTimeout records a request timeout
func RecordRequestTimeout(timeout time.Duration, duration time.Duration, method, path string) {
	RequestTimeoutTotal.Inc()
	RequestTimeoutDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordRequestDuration records a request duration
func RecordRequestDuration(duration time.Duration, method, path, status string) {
	RequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
}

// RecordRequestStart records the start of a request
func RecordRequestStart() {
	RequestInFlight.Inc()
	atomic.AddInt64(&requestInFlightCount, 1)
}

// RecordRequestEnd records the end of a request
func RecordRequestEnd() {
	RequestInFlight.Dec()
	atomic.AddInt64(&requestInFlightCount, -1)
}

// GetRequestInFlight returns the current number of in-flight requests
func GetRequestInFlight() float64 {
	return float64(atomic.LoadInt64(&requestInFlightCount))
}

// RecordRedisFallbackHit records a Redis fallback cache hit
func RecordRedisFallbackHit() {
	RedisFallbackHitTotal.Inc()
}

// RecordRedisUnavailable records when Redis is unavailable
func RecordRedisUnavailable() {
	RedisUnavailableTotal.Inc()
	RedisAvailableGauge.Set(0)
}

// RecordRedisAvailable records when Redis is available
func RecordRedisAvailable(available bool) {
	if available {
		RedisAvailableGauge.Set(1)
	} else {
		RedisAvailableGauge.Set(0)
	}
}
