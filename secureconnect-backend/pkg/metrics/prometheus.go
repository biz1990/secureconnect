package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	// Registry is the Prometheus registry for all metrics
	Registry *prometheus.Registry
	// initialized tracks whether metrics have been registered
	initialized bool
	// mu protects initialization state
	mu sync.RWMutex

	// HTTP Request Metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge

	// Database Metrics
	dbQueryDuration     *prometheus.HistogramVec
	dbConnectionsActive prometheus.Gauge
	dbConnectionsIdle   prometheus.Gauge
	dbQueryErrorsTotal  *prometheus.CounterVec

	// Redis Metrics
	redisCommandsTotal   *prometheus.CounterVec
	redisCommandDuration *prometheus.HistogramVec
	redisConnections     prometheus.Gauge
	redisErrorsTotal     *prometheus.CounterVec

	// WebSocket Metrics
	websocketConnections   prometheus.Gauge
	websocketMessagesTotal *prometheus.CounterVec
	websocketErrorsTotal   *prometheus.CounterVec

	// Call Metrics
	callsTotal       *prometheus.CounterVec
	callsActive      prometheus.Gauge
	callsDuration    *prometheus.HistogramVec
	callsFailedTotal *prometheus.CounterVec

	// Message Metrics
	messagesTotal         *prometheus.CounterVec
	messagesSentTotal     *prometheus.CounterVec
	messagesReceivedTotal *prometheus.CounterVec

	// Push Notification Metrics
	pushNotificationsTotal  *prometheus.CounterVec
	pushNotificationsFailed *prometheus.CounterVec

	// Email Metrics
	emailsTotal  *prometheus.CounterVec
	emailsFailed *prometheus.CounterVec

	// Auth Metrics
	authAttemptsTotal *prometheus.CounterVec
	authSuccessTotal  *prometheus.CounterVec
	authFailuresTotal *prometheus.CounterVec

	// Rate Limiting Metrics
	rateLimitHitsTotal    *prometheus.CounterVec
	rateLimitBlockedTotal *prometheus.CounterVec
}

var (
	// globalMetrics is the singleton instance of Metrics
	globalMetrics *Metrics
	// globalMetricsOnce ensures thread-safe initialization
	globalMetricsOnce sync.Once
)

// NewMetrics creates and registers all Prometheus metrics
// This is idempotent - calling multiple times returns the same instance
func NewMetrics(serviceName string) *Metrics {
	// Use sync.Once to ensure only one Metrics instance is created
	globalMetricsOnce.Do(func() {
		// Create a custom registry to avoid conflicts with default registry
		registry := prometheus.NewRegistry()

		// Register standard Go collectors (process, runtime, etc.)
		registry.MustRegister(
			prometheus.NewGoCollector(),
			prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		)

		m := &Metrics{
			Registry:    registry,
			initialized: true,

			// HTTP Request Metrics
			httpRequestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "http_requests_total",
					Help:        "Total number of HTTP requests",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"method", "endpoint", "status"},
			),
			httpRequestDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:        "http_request_duration_seconds",
					Help:        "HTTP request latency in seconds",
					ConstLabels: prometheus.Labels{"service": serviceName},
					Buckets:     prometheus.DefBuckets,
				},
				[]string{"method", "endpoint"},
			),
			httpRequestsInFlight: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name:        "http_requests_in_flight",
					Help:        "Number of HTTP requests currently being processed",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
			),

			// Database Metrics
			dbQueryDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:        "db_query_duration_seconds",
					Help:        "Database query latency in seconds",
					ConstLabels: prometheus.Labels{"service": serviceName},
					Buckets:     prometheus.DefBuckets,
				},
				[]string{"operation", "table"},
			),
			dbConnectionsActive: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name:        "db_connections_active",
					Help:        "Number of active database connections",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
			),
			dbConnectionsIdle: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name:        "db_connections_idle",
					Help:        "Number of idle database connections",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
			),
			dbQueryErrorsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "db_query_errors_total",
					Help:        "Total number of database query errors",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"operation", "table", "error"},
			),

			// Redis Metrics
			redisCommandsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "redis_commands_total",
					Help:        "Total number of Redis commands",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"command"},
			),
			redisCommandDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:        "redis_command_duration_seconds",
					Help:        "Redis command latency in seconds",
					ConstLabels: prometheus.Labels{"service": serviceName},
					Buckets:     prometheus.DefBuckets,
				},
				[]string{"command"},
			),
			redisConnections: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name:        "redis_connections",
					Help:        "Number of Redis connections",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
			),
			redisErrorsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "redis_errors_total",
					Help:        "Total number of Redis errors",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"command", "error"},
			),

			// WebSocket Metrics
			websocketConnections: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name:        "websocket_connections",
					Help:        "Number of active WebSocket connections",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
			),
			websocketMessagesTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "websocket_messages_total",
					Help:        "Total number of WebSocket messages",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type", "direction"},
			),
			websocketErrorsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "websocket_errors_total",
					Help:        "Total number of WebSocket errors",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"error"},
			),

			// Call Metrics
			callsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "calls_total",
					Help:        "Total number of calls",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type", "status"},
			),
			callsActive: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name:        "calls_active",
					Help:        "Number of active calls",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
			),
			callsDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:        "calls_duration_seconds",
					Help:        "Call duration in seconds",
					ConstLabels: prometheus.Labels{"service": serviceName},
					Buckets:     []float64{10, 30, 60, 120, 300, 600, 1800, 3600},
				},
				[]string{"type"},
			),
			callsFailedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "calls_failed_total",
					Help:        "Total number of failed calls",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type", "reason"},
			),

			// Message Metrics
			messagesTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "messages_total",
					Help:        "Total number of messages",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type"},
			),
			messagesSentTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "messages_sent_total",
					Help:        "Total number of messages sent",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type"},
			),
			messagesReceivedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "messages_received_total",
					Help:        "Total number of messages received",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type"},
			),

			// Push Notification Metrics
			pushNotificationsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "push_notifications_total",
					Help:        "Total number of push notifications sent",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type", "platform"},
			),
			pushNotificationsFailed: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "push_notifications_failed_total",
					Help:        "Total number of failed push notifications",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type", "platform", "reason"},
			),

			// Email Metrics
			emailsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "emails_total",
					Help:        "Total number of emails sent",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type"},
			),
			emailsFailed: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "emails_failed_total",
					Help:        "Total number of failed emails",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"type", "reason"},
			),

			// Auth Metrics
			authAttemptsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "auth_attempts_total",
					Help:        "Total number of authentication attempts",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"method"},
			),
			authSuccessTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "auth_success_total",
					Help:        "Total number of successful authentications",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"method"},
			),
			authFailuresTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "auth_failures_total",
					Help:        "Total number of authentication failures",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"method", "reason"},
			),

			// Rate Limiting Metrics
			rateLimitHitsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "rate_limit_hits_total",
					Help:        "Total number of rate limit hits",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"endpoint"},
			),
			rateLimitBlockedTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name:        "rate_limit_blocked_total",
					Help:        "Total number of requests blocked by rate limiting",
					ConstLabels: prometheus.Labels{"service": serviceName},
				},
				[]string{"endpoint"},
			),
		}

		// Register all metrics to the custom registry
		// Using MustRegister to panic on registration errors (should only happen on first call)
		registry.MustRegister(
			m.httpRequestsTotal,
			m.httpRequestDuration,
			m.httpRequestsInFlight,
			m.dbQueryDuration,
			m.dbConnectionsActive,
			m.dbConnectionsIdle,
			m.dbQueryErrorsTotal,
			m.redisCommandsTotal,
			m.redisCommandDuration,
			m.redisConnections,
			m.redisErrorsTotal,
			m.websocketConnections,
			m.websocketMessagesTotal,
			m.websocketErrorsTotal,
			m.callsTotal,
			m.callsActive,
			m.callsDuration,
			m.callsFailedTotal,
			m.messagesTotal,
			m.messagesSentTotal,
			m.messagesReceivedTotal,
			m.pushNotificationsTotal,
			m.pushNotificationsFailed,
			m.emailsTotal,
			m.emailsFailed,
			m.authAttemptsTotal,
			m.authSuccessTotal,
			m.authFailuresTotal,
			m.rateLimitHitsTotal,
			m.rateLimitBlockedTotal,
		)

		globalMetrics = m
	})

	return globalMetrics
}

// GetGlobalMetrics returns the global singleton Metrics instance
// Returns nil if NewMetrics has not been called yet
func GetGlobalMetrics() *Metrics {
	return globalMetrics
}

// IsInitialized returns true if metrics have been initialized
func (m *Metrics) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// GetRegistry returns the Prometheus registry
func (m *Metrics) GetRegistry() *prometheus.Registry {
	return m.Registry
}

// HTTP Metrics Methods

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, strconv.Itoa(statusCode)).Inc()
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// IncrementHTTPRequestsInFlight increments the number of in-flight HTTP requests
func (m *Metrics) IncrementHTTPRequestsInFlight() {
	m.httpRequestsInFlight.Inc()
}

// DecrementHTTPRequestsInFlight decrements the number of in-flight HTTP requests
func (m *Metrics) DecrementHTTPRequestsInFlight() {
	m.httpRequestsInFlight.Dec()
}

// Database Metrics Methods

// RecordDBQuery records a database query
func (m *Metrics) RecordDBQuery(operation, table string, duration time.Duration, err error) {
	m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
	if err != nil {
		m.dbQueryErrorsTotal.WithLabelValues(operation, table, err.Error()).Inc()
	}
}

// SetDBConnections sets the number of database connections
func (m *Metrics) SetDBConnections(active, idle int) {
	m.dbConnectionsActive.Set(float64(active))
	m.dbConnectionsIdle.Set(float64(idle))
}

// Redis Metrics Methods

// RecordRedisCommand records a Redis command
func (m *Metrics) RecordRedisCommand(command string, duration time.Duration, err error) {
	m.redisCommandsTotal.WithLabelValues(command).Inc()
	m.redisCommandDuration.WithLabelValues(command).Observe(duration.Seconds())
	if err != nil {
		m.redisErrorsTotal.WithLabelValues(command, err.Error()).Inc()
	}
}

// SetRedisConnections sets the number of Redis connections
func (m *Metrics) SetRedisConnections(count int) {
	m.redisConnections.Set(float64(count))
}

// WebSocket Metrics Methods

// SetWebSocketConnections sets the number of active WebSocket connections
func (m *Metrics) SetWebSocketConnections(count int) {
	m.websocketConnections.Set(float64(count))
}

// RecordWebSocketMessage records a WebSocket message
func (m *Metrics) RecordWebSocketMessage(msgType, direction string) {
	m.websocketMessagesTotal.WithLabelValues(msgType, direction).Inc()
}

// RecordWebSocketError records a WebSocket error
func (m *Metrics) RecordWebSocketError(err string) {
	m.websocketErrorsTotal.WithLabelValues(err).Inc()
}

// Call Metrics Methods

// RecordCall records a call
func (m *Metrics) RecordCall(callType, status string) {
	m.callsTotal.WithLabelValues(callType, status).Inc()
}

// SetActiveCalls sets the number of active calls
func (m *Metrics) SetActiveCalls(count int) {
	m.callsActive.Set(float64(count))
}

// RecordCallDuration records the duration of a call
func (m *Metrics) RecordCallDuration(callType string, duration time.Duration) {
	m.callsDuration.WithLabelValues(callType).Observe(duration.Seconds())
}

// RecordCallFailure records a failed call
func (m *Metrics) RecordCallFailure(callType, reason string) {
	m.callsFailedTotal.WithLabelValues(callType, reason).Inc()
}

// Message Metrics Methods

// RecordMessage records a message
func (m *Metrics) RecordMessage(msgType string) {
	m.messagesTotal.WithLabelValues(msgType).Inc()
}

// RecordMessageSent records a sent message
func (m *Metrics) RecordMessageSent(msgType string) {
	m.messagesSentTotal.WithLabelValues(msgType).Inc()
}

// RecordMessageReceived records a received message
func (m *Metrics) RecordMessageReceived(msgType string) {
	m.messagesReceivedTotal.WithLabelValues(msgType).Inc()
}

// Push Notification Metrics Methods

// RecordPushNotification records a push notification
func (m *Metrics) RecordPushNotification(notifType, platform string) {
	m.pushNotificationsTotal.WithLabelValues(notifType, platform).Inc()
}

// RecordPushNotificationFailure records a failed push notification
func (m *Metrics) RecordPushNotificationFailure(notifType, platform, reason string) {
	m.pushNotificationsFailed.WithLabelValues(notifType, platform, reason).Inc()
}

// Email Metrics Methods

// RecordEmail records an email
func (m *Metrics) RecordEmail(emailType string) {
	m.emailsTotal.WithLabelValues(emailType).Inc()
}

// RecordEmailFailure records a failed email
func (m *Metrics) RecordEmailFailure(emailType, reason string) {
	m.emailsFailed.WithLabelValues(emailType, reason).Inc()
}

// Auth Metrics Methods

// RecordAuthAttempt records an authentication attempt
func (m *Metrics) RecordAuthAttempt(method string) {
	m.authAttemptsTotal.WithLabelValues(method).Inc()
}

// RecordAuthSuccess records a successful authentication
func (m *Metrics) RecordAuthSuccess(method string) {
	m.authSuccessTotal.WithLabelValues(method).Inc()
}

// RecordAuthFailure records an authentication failure
func (m *Metrics) RecordAuthFailure(method, reason string) {
	m.authFailuresTotal.WithLabelValues(method, reason).Inc()
}

// Rate Limiting Metrics Methods

// RecordRateLimitHit records a rate limit hit
func (m *Metrics) RecordRateLimitHit(endpoint string) {
	m.rateLimitHitsTotal.WithLabelValues(endpoint).Inc()
}

// RecordRateLimitBlocked records a request blocked by rate limiting
func (m *Metrics) RecordRateLimitBlocked(endpoint string) {
	m.rateLimitBlockedTotal.WithLabelValues(endpoint).Inc()
}
