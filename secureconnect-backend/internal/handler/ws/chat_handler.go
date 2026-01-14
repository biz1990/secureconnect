package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// ChatHub manages WebSocket connections for chat
type ChatHub struct {
	// Registered clients per conversation
	conversations map[uuid.UUID]map[*Client]bool

	// Cancel functions for conversation subscriptions
	subscriptionCancels map[uuid.UUID]context.CancelFunc

	// Redis client for Pub/Sub
	redisClient *redis.Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Channels
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message
}

// Client represents a WebSocket client
type Client struct {
	hub            *ChatHub
	conn           *websocket.Conn
	send           chan []byte
	userID         uuid.UUID
	conversationID uuid.UUID
	ctx            context.Context
	cancel         context.CancelFunc
}

// Message types
const (
	MessageTypeChat       = "chat"
	MessageTypeTyping     = "typing"
	MessageTypeRead       = "read"
	MessageTypeUserJoined = "user_joined"
	MessageTypeUserLeft   = "user_left"
)

// Message represents a WebSocket message
type Message struct {
	Type           string                 `json:"type"`
	ConversationID uuid.UUID              `json:"conversation_id"`
	SenderID       uuid.UUID              `json:"sender_id,omitempty"`
	MessageID      uuid.UUID              `json:"message_id,omitempty"`
	Content        string                 `json:"content,omitempty"`
	IsEncrypted    bool                   `json:"is_encrypted,omitempty"`
	MessageType    string                 `json:"message_type,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// GetAllowedOrigins returns allowed WebSocket origins from environment or defaults
func GetAllowedOrigins() map[string]bool {
	allowedOrigins := map[string]bool{
		"http://localhost:3000": true,
		"http://localhost:8080": true,
		"http://127.0.0.1:3000": true,
		"http://127.0.0.1:8080": true,
	}
	return allowedOrigins
}

var upgrader = websocket.Upgrader{
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

// NewChatHub creates a new chat hub
func NewChatHub(redisClient *redis.Client) *ChatHub {
	hub := &ChatHub{
		conversations:       make(map[uuid.UUID]map[*Client]bool),
		subscriptionCancels: make(map[uuid.UUID]context.CancelFunc),
		redisClient:         redisClient,
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		broadcast:           make(chan *Message, 256),
	}

	go hub.run()

	return hub
}

// run handles hub operations
func (h *ChatHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.conversations[client.conversationID] == nil {
				h.conversations[client.conversationID] = make(map[*Client]bool)

				// Create cancelable context for subscription
				ctx, cancel := context.WithCancel(context.Background())
				h.subscriptionCancels[client.conversationID] = cancel

				// Subscribe to Redis channel for this conversation
				go h.subscribeToConversation(ctx, client.conversationID)
			}
			h.conversations[client.conversationID][client] = true
			h.mu.Unlock()

			// Notify others that user joined
			h.broadcast <- &Message{
				Type:           MessageTypeUserJoined,
				ConversationID: client.conversationID,
				SenderID:       client.userID,
				Timestamp:      time.Now(),
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.conversations[client.conversationID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					client.cancel() // Cancel client context

					// Notify others that user left
					h.broadcast <- &Message{
						Type:           MessageTypeUserLeft,
						ConversationID: client.conversationID,
						SenderID:       client.userID,
						Timestamp:      time.Now(),
					}

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
			if clients, ok := h.conversations[message.ConversationID]; ok {
				messageJSON, _ := json.Marshal(message)
				for client := range clients {
					select {
					case client.send <- messageJSON:
					default:
						close(client.send)
						delete(clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// subscribeToConversation subscribes to Redis Pub/Sub for a conversation
func (h *ChatHub) subscribeToConversation(ctx context.Context, conversationID uuid.UUID) {
	channel := fmt.Sprintf("chat:%s", conversationID)

	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	// Wait for confirmation that subscription is created before receiving messages
	// This prevents a race condition (optional but good practice)
	if _, err := pubsub.Receive(ctx); err != nil {
		logger.Error("Failed to subscribe to Redis channel",
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
			var chatMsg Message
			if err := json.Unmarshal([]byte(msg.Payload), &chatMsg); err != nil {
				logger.Warn("Failed to unmarshal Redis message",
					zap.String("conversation_id", conversationID.String()),
					zap.Error(err))
				continue
			}

			// Broadcast to WebSocket clients
			h.broadcast <- &chatMsg
		}
	}
}

// ServeWS handles WebSocket requests
func (h *ChatHub) ServeWS(c *gin.Context) {
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

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Warn("WebSocket upgrade failed",
			zap.String("conversation_id", conversationID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return
	}

	// Create cancelable context for this client's subscription interest
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		hub:            h,
		conn:           conn,
		send:           make(chan []byte, 256),
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
func (c *Client) readPump() {
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
					zap.String("conversation_id", c.conversationID.String()),
					zap.String("user_id", c.userID.String()),
					zap.Error(err))
			}
			break
		}

		// Parse message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warn("Invalid message format from WebSocket",
				zap.String("conversation_id", c.conversationID.String()),
				zap.String("user_id", c.userID.String()),
				zap.Error(err))
			continue
		}

		// Set metadata
		msg.SenderID = c.userID
		msg.ConversationID = c.conversationID
		msg.Timestamp = time.Now()

		// Broadcast to hub
		c.hub.broadcast <- &msg
	}
}

// writePump writes messages to WebSocket
func (c *Client) writePump() {
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
