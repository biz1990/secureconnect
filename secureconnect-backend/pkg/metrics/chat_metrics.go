package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Chat metrics for monitoring message lifecycle and real-time delivery
var (
	// Message lifecycle metrics
	ChatMessageCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_message_created_total",
		Help: "Total number of messages created",
	}, []string{"message_type", "is_encrypted"})

	ChatMessagePersistedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_message_persisted_total",
		Help: "Total number of messages persisted to Cassandra",
	}, []string{"status"})

	ChatMessagePublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_message_published_total",
		Help: "Total number of messages published to Redis",
	}, []string{"status"})

	ChatMessageDeliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chat_message_delivery_duration_seconds",
		Help:    "Time taken to deliver a message",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"step"}) // "persist", "publish", "notify"

	// Authorization metrics
	ChatMessageSendUnauthorizedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chat_message_send_unauthorized_total",
		Help: "Total number of messages rejected due to unauthorized access",
	})

	ChatWebSocketConnectionUnauthorizedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chat_websocket_connection_unauthorized_total",
		Help: "Total number of rejected WebSocket connections",
	})

	// Concurrency and goroutine safety metrics
	ChatBroadcastPanicTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chat_broadcast_panic_total",
		Help: "Total number of panics during broadcast",
	})

	ChatRedisSubscriptionActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chat_redis_subscription_active",
		Help: "Current number of active Redis subscriptions",
	})

	// Channel capacity metrics
	ChatBroadcastChannelLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chat_broadcast_channel_length",
		Help: "Current length of broadcast channel",
	})

	ChatClientSendChannelLength = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "chat_client_send_channel_length",
		Help: "Current length of client send channel",
	}, []string{"conversation_id", "user_id"})

	ChatClientMessageDroppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_client_message_dropped_total",
		Help: "Total number of messages dropped to clients",
	}, []string{"reason"})

	// Redis metrics
	ChatRedisPublishSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chat_redis_publish_success_total",
		Help: "Total number of successful Redis publishes",
	})

	ChatRedisPublishErrorTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "chat_redis_publish_error_total",
		Help: "Total number of failed Redis publishes",
	})

	// WebSocket lifecycle metrics
	ChatWebSocketConnectionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_websocket_connection_total",
		Help: "Total number of WebSocket connections",
	}, []string{"status"})

	ChatWebSocketDisconnectionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_websocket_disconnection_total",
		Help: "Total number of WebSocket disconnections",
	}, []string{"reason"})

	// Conversation metrics
	ChatConversationParticipantsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "chat_conversation_participants_total",
		Help: "Current number of participants per conversation",
	}, []string{"conversation_id"})

	// WebSocket connection metrics
	ChatWebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chat_websocket_connections",
		Help: "Current number of active WebSocket connections",
	})

	// WebSocket message metrics
	ChatWebSocketMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_websocket_messages_total",
		Help: "Total number of WebSocket messages",
	}, []string{"direction"}) // "in" for received, "out" for sent

	// WebSocket error metrics
	ChatWebSocketErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_websocket_errors_total",
		Help: "Total number of WebSocket errors",
	}, []string{"error_type"})
)
