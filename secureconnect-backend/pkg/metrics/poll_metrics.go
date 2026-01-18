package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Poll metrics for monitoring poll lifecycle and voting
var (
	// Poll lifecycle metrics
	PollsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "polls_created_total",
		Help: "Total number of polls created",
	}, []string{"poll_type", "conversation_id"})

	PollsClosedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "polls_closed_total",
		Help: "Total number of polls closed",
	}, []string{"conversation_id"})

	PollsDeletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "polls_deleted_total",
		Help: "Total number of polls deleted",
	}, []string{"conversation_id"})

	// Vote metrics
	VotesCastTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "votes_cast_total",
		Help: "Total number of votes cast",
	}, []string{"poll_type", "conversation_id"})

	VotesChangedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "votes_changed_total",
		Help: "Total number of votes changed",
	}, []string{"poll_type", "conversation_id"})

	// Poll type metrics
	PollsByType = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "polls_by_type",
		Help: "Current number of active polls by type",
	}, []string{"poll_type", "conversation_id"})

	// Poll status metrics
	PollsActiveTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "polls_active_total",
		Help: "Current number of active (not closed) polls",
	}, []string{"conversation_id"})

	PollsExpiredTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "polls_expired_total",
		Help: "Total number of polls that have expired",
	}, []string{"conversation_id"})

	// Poll option metrics
	PollOptionsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_options_created_total",
		Help: "Total number of poll options created",
	}, []string{"conversation_id"})

	// WebSocket metrics for polls
	PollWebSocketConnectionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_websocket_connection_total",
		Help: "Total number of poll WebSocket connections",
	}, []string{"status"})

	PollWebSocketDisconnectionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_websocket_disconnection_total",
		Help: "Total number of poll WebSocket disconnections",
	}, []string{"reason"})

	PollWebSocketConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "poll_websocket_connections_active",
		Help: "Current number of active poll WebSocket connections",
	})

	PollWebSocketMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_websocket_messages_total",
		Help: "Total number of poll WebSocket messages",
	}, []string{"type", "direction"})

	// Event publishing metrics
	PollEventPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_event_published_total",
		Help: "Total number of poll events published to Redis",
	}, []string{"event_type", "status"})

	// Authorization metrics
	PollCreateUnauthorizedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "poll_create_unauthorized_total",
		Help: "Total number of poll creation attempts rejected due to unauthorized access",
	})

	PollVoteUnauthorizedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "poll_vote_unauthorized_total",
		Help: "Total number of vote attempts rejected due to unauthorized access",
	})

	PollCloseUnauthorizedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "poll_close_unauthorized_total",
		Help: "Total number of poll close attempts rejected due to unauthorized access",
	})

	PollDeleteUnauthorizedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "poll_delete_unauthorized_total",
		Help: "Total number of poll delete attempts rejected due to unauthorized access",
	})

	// Error metrics
	PollCreateErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_create_error_total",
		Help: "Total number of poll creation errors",
	}, []string{"error_type"})

	PollVoteErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_vote_error_total",
		Help: "Total number of vote errors",
	}, []string{"error_type"})

	PollCloseErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "poll_close_error_total",
		Help: "Total number of poll close errors",
	}, []string{"error_type"})

	// Latency metrics
	PollCreationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "poll_creation_duration_seconds",
		Help:    "Time taken to create a poll",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"poll_type"})

	PollVoteDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "poll_vote_duration_seconds",
		Help:    "Time taken to cast a vote",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"poll_type"})

	PollRetrievalDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "poll_retrieval_duration_seconds",
		Help:    "Time taken to retrieve poll data",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"operation"})
)

// RecordPollCreated records a poll creation
func RecordPollCreated(pollType, conversationID string) {
	PollsCreatedTotal.WithLabelValues(pollType, conversationID).Inc()
}

// RecordPollClosed records a poll closure
func RecordPollClosed(conversationID string) {
	PollsClosedTotal.WithLabelValues(conversationID).Inc()
}

// RecordPollDeleted records a poll deletion
func RecordPollDeleted(conversationID string) {
	PollsDeletedTotal.WithLabelValues(conversationID).Inc()
}

// RecordVoteCast records a vote being cast
func RecordVoteCast(pollType, conversationID string) {
	VotesCastTotal.WithLabelValues(pollType, conversationID).Inc()
}

// RecordVoteChanged records a vote being changed
func RecordVoteChanged(pollType, conversationID string) {
	VotesChangedTotal.WithLabelValues(pollType, conversationID).Inc()
}

// RecordPollWebSocketConnection records a poll WebSocket connection
func RecordPollWebSocketConnection(status string) {
	PollWebSocketConnectionTotal.WithLabelValues(status).Inc()
}

// RecordPollWebSocketDisconnection records a poll WebSocket disconnection
func RecordPollWebSocketDisconnection(reason string) {
	PollWebSocketDisconnectionTotal.WithLabelValues(reason).Inc()
}

// SetPollWebSocketConnectionsActive sets the number of active poll WebSocket connections
func SetPollWebSocketConnectionsActive(count int) {
	PollWebSocketConnectionsActive.Set(float64(count))
}

// RecordPollWebSocketMessage records a poll WebSocket message
func RecordPollWebSocketMessage(msgType, direction string) {
	PollWebSocketMessagesTotal.WithLabelValues(msgType, direction).Inc()
}

// RecordPollEventPublished records a poll event published to Redis
func RecordPollEventPublished(eventType, status string) {
	PollEventPublishedTotal.WithLabelValues(eventType, status).Inc()
}

// RecordPollCreateUnauthorized records an unauthorized poll creation attempt
func RecordPollCreateUnauthorized() {
	PollCreateUnauthorizedTotal.Inc()
}

// RecordPollVoteUnauthorized records an unauthorized vote attempt
func RecordPollVoteUnauthorized() {
	PollVoteUnauthorizedTotal.Inc()
}

// RecordPollCloseUnauthorized records an unauthorized poll close attempt
func RecordPollCloseUnauthorized() {
	PollCloseUnauthorizedTotal.Inc()
}

// RecordPollDeleteUnauthorized records an unauthorized poll delete attempt
func RecordPollDeleteUnauthorized() {
	PollDeleteUnauthorizedTotal.Inc()
}

// RecordPollCreateError records a poll creation error
func RecordPollCreateError(errorType string) {
	PollCreateErrorTotal.WithLabelValues(errorType).Inc()
}

// RecordPollVoteError records a vote error
func RecordPollVoteError(errorType string) {
	PollVoteErrorTotal.WithLabelValues(errorType).Inc()
}

// RecordPollCloseError records a poll close error
func RecordPollCloseError(errorType string) {
	PollCloseErrorTotal.WithLabelValues(errorType).Inc()
}

// RecordPollCreationDuration records the duration of poll creation
func RecordPollCreationDuration(pollType string, duration float64) {
	PollCreationDuration.WithLabelValues(pollType).Observe(duration)
}

// RecordPollVoteDuration records the duration of voting
func RecordPollVoteDuration(pollType string, duration float64) {
	PollVoteDuration.WithLabelValues(pollType).Observe(duration)
}

// RecordPollRetrievalDuration records the duration of poll data retrieval
func RecordPollRetrievalDuration(operation string, duration float64) {
	PollRetrievalDuration.WithLabelValues(operation).Observe(duration)
}
