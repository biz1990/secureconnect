package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/logger"
)

// SignalingHub manages WebRTC signaling connections
type SignalingHub struct {
	// Registered clients per call
	calls map[uuid.UUID]map[*SignalingClient]bool

	// Cancel functions for call subscriptions
	subscriptionCancels map[uuid.UUID]context.CancelFunc

	// Redis client for Pub/Sub
	redisClient *redis.Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Channels
	register   chan *SignalingClient
	unregister chan *SignalingClient
	broadcast  chan *SignalingMessage

	// Concurrency limit: maxConnections is the maximum number of concurrent WebSocket connections
	maxConnections int
	// Semaphore for limiting concurrent connections
	semaphore chan struct{}
}

// SignalingClient represents a WebSocket client for signaling
type SignalingClient struct {
	hub    *SignalingHub
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
	callID uuid.UUID
	ctx    context.Context
	cancel context.CancelFunc
}

// SignalingMessage types
const (
	SignalTypeOffer     = "offer"
	SignalTypeAnswer    = "answer"
	SignalTypeICE       = "ice_candidate"
	SignalTypeJoin      = "join"
	SignalTypeLeave     = "leave"
	SignalTypeMuteAudio = "mute_audio"
	SignalTypeMuteVideo = "mute_video"
)

// SignalingMessage represents a WebRTC signaling message
type SignalingMessage struct {
	Type      string                 `json:"type"`
	CallID    uuid.UUID              `json:"call_id"`
	SenderID  uuid.UUID              `json:"sender_id,omitempty"`
	TargetID  uuid.UUID              `json:"target_id,omitempty"` // For 1-1 signaling
	SDP       string                 `json:"sdp,omitempty"`       // For offer/answer
	Candidate map[string]interface{} `json:"candidate,omitempty"` // For ICE
	Muted     bool                   `json:"muted,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

var signalingUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Reject empty origins - require explicit origin for security
			return false
		}

		// Check if origin is in allowed list
		allowedOrigins := GetAllowedOrigins()
		for allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

// NewSignalingHub creates a new signaling hub
func NewSignalingHub(redisClient *redis.Client) *SignalingHub {
	// Default max connections: 1000 (configurable via environment if needed)
	maxConns := 1000
	if val := os.Getenv("WS_MAX_SIGNALING_CONNECTIONS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			maxConns = n
		}
	}

	hub := &SignalingHub{
		calls:               make(map[uuid.UUID]map[*SignalingClient]bool),
		subscriptionCancels: make(map[uuid.UUID]context.CancelFunc),
		redisClient:         redisClient,
		register:            make(chan *SignalingClient),
		unregister:          make(chan *SignalingClient),
		broadcast:           make(chan *SignalingMessage, 256),
		maxConnections:      maxConns,
		semaphore:           make(chan struct{}, maxConns),
	}

	go hub.run()

	return hub
}

// run handles hub operations
func (h *SignalingHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.calls[client.callID] == nil {
				h.calls[client.callID] = make(map[*SignalingClient]bool)

				// Create cancelable context for subscription
				ctx, cancel := context.WithCancel(context.Background())
				h.subscriptionCancels[client.callID] = cancel

				// Subscribe to Redis channel for this call
				go h.subscribeToCall(ctx, client.callID)
			}
			h.calls[client.callID][client] = true
			h.mu.Unlock()

			// Notify others that user joined
			h.broadcast <- &SignalingMessage{
				Type:      SignalTypeJoin,
				CallID:    client.callID,
				SenderID:  client.userID,
				Timestamp: time.Now(),
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.calls[client.callID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					client.cancel() // Cancel client context

					// Notify others that user left
					h.broadcast <- &SignalingMessage{
						Type:      SignalTypeLeave,
						CallID:    client.callID,
						SenderID:  client.userID,
						Timestamp: time.Now(),
					}

					// Clean up empty calls
					if len(clients) == 0 {
						// Cancel Redis subscription
						if cancel, ok := h.subscriptionCancels[client.callID]; ok {
							cancel()
							delete(h.subscriptionCancels, client.callID)
						}
						delete(h.calls, client.callID)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.calls[message.CallID]; ok {
				messageJSON, _ := json.Marshal(message)

				// If targeted message (e.g., offer to specific user)
				if message.TargetID != uuid.Nil {
					for client := range clients {
						if client.userID == message.TargetID {
							select {
							case client.send <- messageJSON:
							default:
								close(client.send)
								delete(clients, client)
							}
							break
						}
					}
				} else {
					// Broadcast to all except sender
					for client := range clients {
						if client.userID != message.SenderID {
							select {
							case client.send <- messageJSON:
							default:
								close(client.send)
								delete(clients, client)
							}
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// subscribeToCall subscribes to Redis Pub/Sub for a call
func (h *SignalingHub) subscribeToCall(ctx context.Context, callID uuid.UUID) {
	channel := fmt.Sprintf("call:%s", callID)

	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		logger.Error("Failed to subscribe to Redis channel",
			zap.String("call_id", callID.String()),
			zap.Error(err))
		return
	}

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			// Parse message from Redis
			var signalMsg SignalingMessage
			if err := json.Unmarshal([]byte(msg.Payload), &signalMsg); err != nil {
				logger.Warn("Failed to unmarshal Redis message",
					zap.String("call_id", callID.String()),
					zap.Error(err))
				continue
			}

			// Broadcast to WebSocket clients
			h.broadcast <- &signalMsg
		}
	}
}

// ServeWS handles WebSocket requests for signaling
func (h *SignalingHub) ServeWS(c *gin.Context) {
	// Acquire semaphore to limit concurrent connections
	select {
	case h.semaphore <- struct{}{}:
		// Successfully acquired, continue
		defer func() {
			<-h.semaphore // Release semaphore when connection closes
		}()
	default:
		// No available slots, reject connection
		logger.Warn("WebSocket connection rejected: max connections reached",
			zap.Int("max_connections", h.maxConnections))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Server at capacity, please try again later"})
		return
	}

	// Get call ID from query params
	callIDStr := c.Query("call_id")
	if callIDStr == "" {
		c.JSON(400, gin.H{"error": "call_id required"})
		return
	}

	callID, err := uuid.Parse(callIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid call_id"})
		return
	}

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		c.JSON(500, gin.H{"error": "invalid user_id"})
		return
	}

	// Upgrade to WebSocket
	conn, err := signalingUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warn("WebSocket upgrade failed",
			zap.String("call_id", callID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return
	}

	// Create cancelable context for this client
	ctx, cancel := context.WithCancel(context.Background())
	client := &SignalingClient{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		callID: callID,
		ctx:    ctx,
		cancel: cancel,
	}

	client.hub.register <- client

	// Start goroutines for read/write
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from WebSocket
func (c *SignalingClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(constants.WebSocketPingInterval))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(constants.WebSocketPingInterval))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Debug("WebSocket connection closed",
					zap.String("call_id", c.callID.String()),
					zap.String("user_id", c.userID.String()),
					zap.Error(err))
			}
			break
		}

		// Parse message
		var msg SignalingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warn("Invalid message format from WebSocket",
				zap.String("call_id", c.callID.String()),
				zap.String("user_id", c.userID.String()),
				zap.Error(err))
			continue
		}

		// Set metadata
		msg.SenderID = c.userID
		msg.CallID = c.callID
		msg.Timestamp = time.Now()

		// Broadcast to hub
		c.hub.broadcast <- &msg
	}
}

// writePump writes messages to WebSocket
func (c *SignalingClient) writePump() {
	ticker := time.NewTicker(constants.WebSocketPingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
