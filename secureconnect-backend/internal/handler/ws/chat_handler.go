package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// ChatHub manages WebSocket connections for chat
type ChatHub struct {
	// Registered clients per conversation
	conversations map[uuid.UUID]map[*Client]bool
	
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
}

// Message types
const (
	MessageTypeChat          = "chat"
	MessageTypeTyping        = "typing"
	MessageTypeRead          = "read"
	MessageTypeUserJoined    = "user_joined"
	MessageTypeUserLeft      = "user_left"
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev, restrict in production
	},
}

// NewChatHub creates a new chat hub
func NewChatHub(redisClient *redis.Client) *ChatHub {
	hub := &ChatHub{
		conversations: make(map[uuid.UUID]map[*Client]bool),
		redisClient:   redisClient,
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan *Message, 256),
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
				
				// Subscribe to Redis channel for this conversation
				go h.subscribeToConversation(client.conversationID)
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
					
					// Notify others that user left
					h.broadcast <- &Message{
						Type:           MessageTypeUserLeft,
						ConversationID: client.conversationID,
						SenderID:       client.userID,
						Timestamp:      time.Now(),
					}
					
					// Clean up empty conversations
					if len(clients) == 0 {
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
func (h *ChatHub) subscribeToConversation(conversationID uuid.UUID) {
	ctx := context.Background()
	channel := fmt.Sprintf("chat:%s", conversationID)
	
	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	
	for msg := range ch {
		// Parse message from Redis
		var chatMsg Message
		if err := json.Unmarshal([]byte(msg.Payload), &chatMsg); err != nil {
			log.Printf("Failed to unmarshal Redis message: %v", err)
			continue
		}
		
		// Broadcast to WebSocket clients
		h.broadcast <- &chatMsg
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
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	client := &Client{
		hub:            h,
		conn:           conn,
		send:           make(chan []byte, 256),
		userID:         userID,
		conversationID: conversationID,
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
	
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		// Parse message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
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
	ticker := time.NewTicker(54 * time.Second)
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
