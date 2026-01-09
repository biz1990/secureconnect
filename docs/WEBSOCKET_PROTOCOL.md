# SecureConnect WebSocket Protocol Documentation

## Overview
SecureConnect uses WebSocket for real-time bidirectional communication between clients and the chat service.

## Connection

### Endpoint
```
ws://localhost:8082/v1/ws/chat (Development)
wss://chat.secureconnect.io/v1/ws/chat (Production)
```

### Authentication
**Option 1: Query Parameter**
```
ws://localhost:8082/v1/ws/chat?conversation_id=UUID&access_token=JWT
```

**Option 2: Authorization Header**
```javascript
const ws = new WebSocket('ws://localhost:8082/v1/ws/chat?conversation_id=UUID', {
  headers: {
    'Authorization': 'Bearer JWT_TOKEN'
  }
});
```

### Required Parameters
- `conversation_id`: UUID of the conversation
- Authentication (JWT) via query or header

---

## Message Types

### 1. Chat Message
**Client → Server:**
```json
{
  "type": "chat",
  "content": "Hello, World!",
  "is_encrypted": false,
  "message_type": "text",
  "metadata": {}
}
```

**Server → All Clients:**
```json
{
  "type": "chat",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "sender_id": "123e4567-e89b-12d3-a456-426614174000",
  "message_id": "789e4567-e89b-12d3-a456-426614174999",
  "content": "Hello, World!",
  "is_encrypted": false,
  "message_type": "text",
  "metadata": {},
  "timestamp": "2026-01-09T10:30:15Z"
}
```

### 2. Typing Indicator
**Client → Server:**
```json
{
  "type": "typing"
}
```

**Server → Other Clients:**
```json
{
  "type": "typing",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "sender_id": "123e4567-e89b-12d3-a456-426614174000",
  "timestamp": "2026-01-09T10:30:15Z"
}
```

### 3. Read Receipt
**Client → Server:**
```json
{
  "type": "read",
  "message_id": "789e4567-e89b-12d3-a456-426614174999"
}
```

**Server → Other Clients:**
```json
{
  "type": "read",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "sender_id": "123e4567-e89b-12d3-a456-426614174000",
  "message_id": "789e4567-e89b-12d3-a456-426614174999",
  "timestamp": "2026-01-09T10:30:15Z"
}
```

### 4. User Joined (System Message)
**Server → All Clients:**
```json
{
  "type": "user_joined",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "sender_id": "123e4567-e89b-12d3-a456-426614174000",
  "timestamp": "2026-01-09T10:30:15Z"
}
```

### 5. User Left (System Message)
**Server → All Clients:**
```json
{
  "type": "user_left",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "sender_id": "123e4567-e89b-12d3-a456-426614174000",
  "timestamp": "2026-01-09T10:30:15Z"
}
```

---

## Connection Lifecycle

### 1. Establish Connection
```javascript
const conversationId = '550e8400-e29b-41d4-a716-446655440000';
const token = 'eyJhbGc...';

const ws = new WebSocket(
  `ws://localhost:8082/v1/ws/chat?conversation_id=${conversationId}`,
  {
    headers: { 'Authorization': `Bearer ${token}` }
  }
);
```

### 2. Handle Connection Events
```javascript
ws.onopen = () => {
  console.log('🔗 Connected to chat');
  // User join notification sent automatically
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('📨 Received:', message);
  
  switch (message.type) {
    case 'chat':
      displayMessage(message);
      break;
    case 'typing':
      showTypingIndicator(message.sender_id);
      break;
    case 'read':
      markAsRead(message.message_id);
      break;
    case 'user_joined':
      showUserJoined(message.sender_id);
      break;
    case 'user_left':
      showUserLeft(message.sender_id);
      break;
  }
};

ws.onerror = (error) => {
  console.error('❌ WebSocket error:', error);
};

ws.onclose = (event) => {
  console.log('🔌 Disconnected:', event.code, event.reason);
  // Implement reconnection logic
  if (event.code !== 1000) {
    setTimeout(() => reconnect(), 5000);
  }
};
```

### 3. Send Messages
```javascript
// Send chat message
function sendMessage(content, isEncrypted = false) {
  ws.send(JSON.stringify({
    type: 'chat',
    content: content,
    is_encrypted: isEncrypted,
    message_type: 'text'
  }));
}

// Send typing indicator
function sendTyping() {
  ws.send(JSON.stringify({
    type: 'typing'
  }));
}

// Send read receipt
function markAsRead(messageId) {
  ws.send(JSON.stringify({
    type: 'read',
    message_id: messageId
  }));
}
```

### 4. Close Connection
```javascript
ws.close(1000, 'User logged out');
```

---

## Heartbeat / Keep-Alive

The server sends **PING** frames every **54 seconds**.  
Client should respond with **PONG** automatically (handled by browser).

If no PONG received within **60 seconds**, server closes connection.

**Client Implementation:**
```javascript
// Most WebSocket clients handle PING/PONG automatically
// No manual intervention needed

// To detect disconnection:
let heartbeatInterval = setInterval(() => {
  if (ws.readyState === WebSocket.OPEN) {
    console.log('💓 Connection alive');
  } else {
    console.warn('⚠️ Connection lost, reconnecting...');
    reconnect();
  }
}, 60000); // Check every 60s
```

---

## Error Handling

### Connection Errors

**400 Bad Request**
```json
{
  "error": "conversation_id required"
}
```
- Missing `conversation_id` parameter

**401 Unauthorized**
```json
{
  "error": "unauthorized"
}
```
- Invalid or expired JWT token

**500 Internal Server Error**
```json
{
  "error": "invalid user_id"
}
```
- Server-side issue

### WebSocket Close Codes

| Code | Meaning | Action |
|------|---------|--------|
| 1000 | Normal Closure | Don't reconnect |
| 1001 | Going Away | Reconnect after delay |
| 1006 | Abnormal Closure | Reconnect immediately |
| 1008 | Policy Violation | Check authentication |
| 1011 | Internal Error | Reconnect with backoff |

---

## Reconnection Strategy

### Exponential Backoff
```javascript
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;
const baseDelay = 1000; // 1 second

function reconnect() {
  if (reconnectAttempts >= maxReconnectAttempts) {
    console.error('❌ Max reconnection attempts reached');
    return;
  }
  
  const delay = Math.min(
    baseDelay * Math.pow(2, reconnectAttempts),
    30000 // Max 30 seconds
  );
  
  reconnectAttempts++;
  console.log(`🔄 Reconnecting in ${delay}ms (attempt ${reconnectAttempts})`);
  
  setTimeout(() => {
    establishConnection();
  }, delay);
}

function establishConnection() {
  ws = new WebSocket(...);
  
  ws.onopen = () => {
    reconnectAttempts = 0; // Reset on successful connection
    console.log('✅ Reconnected successfully');
  };
  
  ws.onerror = () => {
    reconnect();
  };
}
```

---

## Message Queueing

Queue messages while offline and send when reconnected:

```javascript
const messageQueue = [];

function sendMessage(content) {
  const message = {
    type: 'chat',
    content: content,
    is_encrypted: false,
    message_type: 'text'
  };
  
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(message));
  } else {
    console.warn('⚠️ Connection lost, queueing message');
    messageQueue.push(message);
  }
}

ws.onopen = () => {
  // Send queued messages
  while (messageQueue.length > 0) {
    const msg = messageQueue.shift();
    ws.send(JSON.stringify(msg));
  }
};
```

---

## Security Considerations

### 1. TLS/SSL
**Always use WSS in production:**
```javascript
const ws = new WebSocket('wss://chat.secureconnect.io/v1/ws/chat?...');
```

### 2. Token Refresh
JWT tokens expire. Implement token refresh:
```javascript
let tokenRefreshTimer;

function setupTokenRefresh(expiresIn) {
  // Refresh 5 minutes before expiry
  const refreshTime = (expiresIn - 300) * 1000;
  
  tokenRefreshTimer = setTimeout(async () => {
    const newToken = await refreshAccessToken();
    
    // Reconnect with new token
    ws.close(1000, 'Token refresh');
    establishConnectionWithToken(newToken);
  }, refreshTime);
}
```

### 3. Message Validation
Always validate incoming messages:
```javascript
ws.onmessage = (event) => {
  try {
    const message = JSON.parse(event.data);
    
    // Validate required fields
    if (!message.type || !message.timestamp) {
      console.warn('Invalid message format');
      return;
    }
    
    // Process message
    handleMessage(message);
  } catch (err) {
    console.error('Failed to parse message:', err);
  }
};
```

---

## Complete Example

```javascript
class ChatWebSocket {
  constructor(conversationId, token) {
    this.conversationId = conversationId;
    this.token = token;
    this.ws = null;
    this.messageQueue = [];
    this.reconnectAttempts = 0;
    
    this.connect();
  }
  
  connect() {
    const url = `ws://localhost:8082/v1/ws/chat?conversation_id=${this.conversationId}`;
    
    this.ws = new WebSocket(url);
    
    // Add auth header if needed
    // Note: Some browsers don't support headers in WebSocket constructor
    // In that case, use query parameter: url + &access_token=${this.token}
    
    this.ws.onopen = this.handleOpen.bind(this);
    this.ws.onmessage = this.handleMessage.bind(this);
    this.ws.onerror = this.handleError.bind(this);
    this.ws.onclose = this.handleClose.bind(this);
  }
  
  handleOpen() {
    console.log('✅ Connected');
    this.reconnectAttempts = 0;
    
    // Flush message queue
    while (this.messageQueue.length > 0) {
      this.send(this.messageQueue.shift());
    }
  }
  
  handleMessage(event) {
    const message = JSON.parse(event.data);
    
    switch (message.type) {
      case 'chat':
        this.onChat(message);
        break;
      case 'typing':
        this.onTyping(message);
        break;
      case 'user_joined':
        this.onUserJoined(message);
        break;
      case 'user_left':
        this.onUserLeft(message);
        break;
    }
  }
  
  handleError(error) {
    console.error('❌ WebSocket error:', error);
  }
  
  handleClose(event) {
    console.log('🔌 Disconnected:', event.code);
    
    if (event.code !== 1000) {
      this.reconnect();
    }
  }
  
  reconnect() {
    if (this.reconnectAttempts >= 10) {
      console.error('Max reconnect attempts reached');
      return;
    }
    
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    this.reconnectAttempts++;
    
    setTimeout(() => this.connect(), delay);
  }
  
  send(message) {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      this.messageQueue.push(message);
    }
  }
  
  sendChat(content, isEncrypted = false) {
    this.send({
      type: 'chat',
      content: content,
      is_encrypted: isEncrypted,
      message_type: 'text'
    });
  }
  
  sendTyping() {
    this.send({ type: 'typing' });
  }
  
  close() {
    this.ws.close(1000);
  }
  
  // Callbacks (override these)
  onChat(message) {}
  onTyping(message) {}
  onUserJoined(message) {}
  onUserLeft(message) {}
}

// Usage
const chat = new ChatWebSocket('conversation-uuid', 'jwt-token');

chat.onChat = (message) => {
  console.log('New message:', message.content);
};

chat.sendChat('Hello, WebSocket!');
```

---

## Testing

### Using wscat (CLI tool)
```bash
# Install
npm install -g wscat

# Connect
wscat -c "ws://localhost:8082/v1/ws/chat?conversation_id=UUID" \
  -H "Authorization: Bearer JWT_TOKEN"

# Send message
> {"type":"chat","content":"Hello","is_encrypted":false,"message_type":"text"}

# Receive
< {"type":"chat","conversation_id":"...","content":"Hello",...}
```

### Using Browser Console
```javascript
const ws = new WebSocket('ws://localhost:8082/v1/ws/chat?conversation_id=UUID');

ws.onopen = () => console.log('Connected!');
ws.onmessage = (e) => console.log('Received:', JSON.parse(e.data));

ws.send(JSON.stringify({
  type: 'chat',
  content: 'Test message',
  is_encrypted: false,
  message_type: 'text'
}));
```

---

## Performance Tips

1. **Batch typing indicators**: Don't send on every keystroke
2. **Debounce read receipts**: Send after 1 second delay
3. **Message compression**: Enable WebSocket compression
4. **Connection pooling**: Reuse connections when possible
5. **Limit message size**: Keep messages under 1MB

---

For more information:
- [API Documentation](./API_DOCUMENTATION.md)
- [Security Architecture](./03-security-architecture.md)
- [Deployment Guide](./DEPLOYMENT_GUIDE.md)
