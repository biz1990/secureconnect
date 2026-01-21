package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/logger"
)

// PollHub manages WebSocket connections for polls
type PollHub struct {
	// Registered clients per conversation
	conversations map[uuid.UUID]map[*PollClient]bool

	// Cancel functions for conversation subscriptions
	subscriptionCancels map[uuid.UUID]context.CancelFunc

	// Redis client for Pub/Sub
	redisClient *redis.Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Channels
	register   chan *PollClient
	unregister chan *PollClient
	broadcast  chan *PollMessage

	// Concurrency limit: maxConnections is the maximum number of concurrent WebSocket connections
	maxConnections int
	// Semaphore for limiting concurrent connections
	semaphore chan struct{}
}

// PollClient represents a WebSocket client for polls
type PollClient struct {
	hub            *PollHub
	conn           *websocket.Conn
	send           chan []byte
	userID         uuid.UUID
	conversationID uuid.UUID
	ctx            context.Context
	cancel         context.CancelFunc
}

// Poll message types
const (
	PollMessageTypeCreated = "poll_created"
	PollMessageTypeVoted   = "poll_voted"
	PollMessageTypeClosed  = "poll_closed"
)

// PollMessage represents a WebSocket message for polls
type PollMessage struct {
	Type           string                 `json:"type"`
	ConversationID uuid.UUID              `json:"conversation_id"`
	Data           map[string]interface{} `json:"data"`
	Timestamp      time.Time              `json:"timestamp"`
}

// GetPollAllowedOrigins returns allowed WebSocket origins from environment or defaults
func GetPollAllowedOrigins() map[string]bool {
	allowedOrigins := map[string]bool{
		"http://localhost:3000": true,
		"http://localhost:8080": true,
		"http://127.0.0.1:3000": true,
		"http://127.0.0.1:8080": true,
	}

	// Add production origins from environment if set
	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		// Parse comma-separated origins
		for _, origin := range strings.Split(origins, ",") {
			allowedOrigins[strings.TrimSpace(origin)] = true
		}
	}

	return allowedOrigins
}

var pollUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Reject empty origins - require explicit origin for security
			return false
		}

		// Check if origin is in allowed list
		allowedOrigins := GetPollAllowedOrigins()
		for allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

// NewPollHub creates a new poll hub
func NewPollHub(redisClient *redis.Client) *PollHub {
	// Default max connections: 1000 (configurable via environment if needed)
	maxConns := 1000
	if val := os.Getenv("WS_MAX_POLL_CONNECTIONS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			maxConns = n
		}
	}

	hub := &PollHub{
		conversations:       make(map[uuid.UUID]map[*PollClient]bool),
		subscriptionCancels: make(map[uuid.UUID]context.CancelFunc),
		redisClient:         redisClient,
		register:            make(chan *PollClient),
		unregister:          make(chan *PollClient),
		broadcast:           make(chan *PollMessage, 1000),
		maxConnections:      maxConns,
		semaphore:           make(chan struct{}, maxConns),
	}

	go hub.run()

	return hub
}

// run handles hub operations
func (h *PollHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.conversations[client.conversationID] == nil {
				h.conversations[client.conversationID] = make(map[*PollClient]bool)

				// Create cancelable context for subscription
				ctx, cancel := context.WithCancel(context.Background())
				h.subscriptionCancels[client.conversationID] = cancel

				// Subscribe to Redis channel for this conversation
				go h.subscribeToConversation(ctx, client.conversationID)
			}
			h.conversations[client.conversationID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.conversations[client.conversationID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					client.cancel() // Cancel client context

					// Clean up empty conversations
					if len(clients) == 0 {
						// Cancel Redis subscription
						if cancel, ok := h.subscriptionCancels[client.conversationID]; ok {
							cancel()
							delete(h.subscriptionCancels, client.conversationID)
						}
						delete(h.conversations, client.conversationID)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			var clientsToRemove []*PollClient
			if clients, ok := h.conversations[message.ConversationID]; ok {
				messageJSON, _ := json.Marshal(message)
				for client := range clients {
					select {
					case client.send <- messageJSON:
					default:
						// Mark for removal instead of deleting now
						clientsToRemove = append(clientsToRemove, client)
					}
				}
			}
			h.mu.RUnlock()

			// Remove clients outside of read lock
			if len(clientsToRemove) > 0 {
				h.mu.Lock()
				if clients, ok := h.conversations[message.ConversationID]; ok {
					for _, client := range clientsToRemove {
						delete(clients, client)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// subscribeToConversation subscribes to Redis Pub/Sub for a conversation
func (h *PollHub) subscribeToConversation(ctx context.Context, conversationID uuid.UUID) {
	channel := fmt.Sprintf("poll:%s", conversationID)

	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// Wait for confirmation that subscription is created before receiving messages
	if _, err := pubsub.Receive(ctx); err != nil {
		logger.Error("Failed to subscribe to Redis poll channel",
			zap.String("conversation_id", conversationID.String()),
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
			var pollMsg PollMessage
			if err := json.Unmarshal([]byte(msg.Payload), &pollMsg); err != nil {
				logger.Warn("Failed to unmarshal Redis poll message",
					zap.String("conversation_id", conversationID.String()),
					zap.Error(err))
				continue
			}

			// Broadcast to WebSocket clients
			h.broadcast <- &pollMsg
		}
	}
}

// ServePollWS handles WebSocket requests for polls
func (h *PollHub) ServePollWS(c *gin.Context, conversationRepo *cockroach.ConversationRepository) {
	// Acquire semaphore to limit concurrent connections
	select {
	case h.semaphore <- struct{}{}:
		// Successfully acquired, continue
		defer func() {
			<-h.semaphore // Release semaphore when connection closes
		}()
	default:
		// No available slots, reject connection
		logger.Warn("Poll WebSocket connection rejected: max connections reached",
			zap.Int("max_connections", h.maxConnections))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Server at capacity, please try again later"})
		return
	}

	// Get conversation ID from query params
	conversationIDStr := c.Query("conversation_id")
	if conversationIDStr == "" {
		c.JSON(400, gin.H{"error": "conversation_id required"})
		return
	}

	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid conversation_id"})
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

	// Validate user is a participant in conversation
	isParticipant, err := conversationRepo.IsParticipant(c.Request.Context(), conversationID, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to verify conversation membership"})
		return
	}
	if !isParticipant {
		c.JSON(403, gin.H{"error": "unauthorized: not a participant in this conversation"})
		return
	}

	// Upgrade to WebSocket
	conn, err := pollUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warn("Poll WebSocket upgrade failed",
			zap.String("conversation_id", conversationID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return
	}

	// Create cancelable context for this client's subscription interest
	ctx, cancel := context.WithCancel(context.Background())
	client := &PollClient{
		hub:            h,
		conn:           conn,
		send:           make(chan []byte, 1000),
		userID:         userID,
		conversationID: conversationID,
		ctx:            ctx,
		cancel:         cancel,
	}

	client.hub.register <- client

	// Start goroutines for read/write
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from WebSocket
func (c *PollClient) readPump() {
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
				logger.Debug("Poll WebSocket connection closed",
					zap.String("conversation_id", c.conversationID.String()),
					zap.String("user_id", c.userID.String()),
					zap.Error(err))
			}
			break
		}

		// Parse message (poll clients typically don't send messages, just receive)
		var msg PollMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warn("Invalid message format from poll WebSocket",
				zap.String("conversation_id", c.conversationID.String()),
				zap.String("user_id", c.userID.String()),
				zap.Error(err))
			continue
		}

		// Set metadata
		msg.ConversationID = c.conversationID
		msg.Timestamp = time.Now()

		// Broadcast to hub (if needed for future bidirectional communication)
		c.hub.broadcast <- &msg
	}
}

// writePump writes messages to WebSocket
func (c *PollClient) writePump() {
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
