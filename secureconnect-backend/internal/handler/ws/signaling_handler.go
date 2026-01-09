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
)

// SignalingHub manages WebRTC signaling connections
type SignalingHub struct {
	// Registered clients per call
	calls map[uuid.UUID]map[*SignalingClient]bool
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// Channels
	register   chan *SignalingClient
	unregister chan *SignalingClient
	broadcast  chan *SignalingMessage
}

// SignalingClient represents a WebSocket client for signaling
type SignalingClient struct {
	hub    *SignalingHub
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
	callID uuid.UUID
}

// SignalingMessage types
const (
	SignalTypeOffer      = "offer"
	SignalTypeAnswer     = "answer"
	SignalTypeICE        = "ice_candidate"
	SignalTypeJoin       = "join"
	SignalTypeLeave      = "leave"
	SignalTypeMuteAudio  = "mute_audio"
	SignalTypeMuteVideo  = "mute_video"
)

// SignalingMessage represents a WebRTC signaling message
type SignalingMessage struct {
	Type       string                 `json:"type"`
	CallID     uuid.UUID              `json:"call_id"`
	SenderID   uuid.UUID              `json:"sender_id,omitempty"`
	TargetID   uuid.UUID              `json:"target_id,omitempty"` // For 1-1 signaling
	SDP        string                 `json:"sdp,omitempty"`       // For offer/answer
	Candidate  map[string]interface{} `json:"candidate,omitempty"` // For ICE
	Muted      bool                   `json:"muted,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

var signalingUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev
	},
}

// NewSignalingHub creates a new signaling hub
func NewSignalingHub() *SignalingHub {
	hub := &SignalingHub{
		calls:      make(map[uuid.UUID]map[*SignalingClient]bool),
		register:   make(chan *SignalingClient),
		unregister: make(chan *SignalingClient),
		broadcast:  make(chan *SignalingMessage, 256),
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
					
					// Notify others that user left
					h.broadcast <- &SignalingMessage{
						Type:      SignalTypeLeave,
						CallID:    client.callID,
						SenderID:  client.userID,
						Timestamp: time.Now(),
					}
					
					// Clean up empty calls
					if len(clients) == 0 {
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

// ServeWS handles WebSocket requests for signaling
func (h *SignalingHub) ServeWS(c *gin.Context) {
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
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	client := &SignalingClient{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		callID: callID,
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
		var msg SignalingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
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
