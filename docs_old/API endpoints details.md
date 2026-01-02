# API Endpoints Specification

## Base URL Structure
```
Production: https://api.yourdomain.com/v1
Staging: https://api-staging.yourdomain.com/v1
WebSocket: wss://ws.yourdomain.com
```

## Authentication
All authenticated endpoints require:
```
Header: Authorization: Bearer {JWT_TOKEN}
```

---

## 1. AUTH SERVICE

### 1.1 Register User
```http
POST /auth/register
Content-Type: application/json

Request:
{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "SecurePass123!",
  "full_name": "John Doe",
  "phone": "+84123456789"
}

Response (201):
{
  "success": true,
  "data": {
    "user_id": "usr_1234567890",
    "email": "user@example.com",
    "username": "johndoe",
    "created_at": "2025-01-15T10:30:00Z"
  },
  "message": "User registered successfully. Please verify your email."
}

Errors:
- 400: Invalid input
- 409: User already exists
```

### 1.2 Login
```http
POST /auth/login
Content-Type: application/json

Request:
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "device_id": "device_xyz123"
}

Response (200):
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "token_type": "Bearer",
    "user": {
      "user_id": "usr_1234567890",
      "email": "user@example.com",
      "username": "johndoe",
      "avatar_url": "https://cdn.example.com/avatars/user.jpg"
    }
  }
}

Errors:
- 401: Invalid credentials
- 403: Account suspended
- 429: Too many login attempts
```

### 1.3 Refresh Token
```http
POST /auth/refresh
Content-Type: application/json

Request:
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}

Response (200):
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600
  }
}
```

### 1.4 Logout
```http
POST /auth/logout
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "message": "Logged out successfully"
}
```

### 1.5 Enable 2FA
```http
POST /auth/2fa/enable
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "qr_code": "data:image/png;base64,...",
    "secret": "JBSWY3DPEHPK3PXP",
    "backup_codes": ["12345678", "87654321", ...]
  }
}
```

### 1.6 Verify 2FA
```http
POST /auth/2fa/verify
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "code": "123456"
}

Response (200):
{
  "success": true,
  "message": "2FA verified successfully"
}
```

---

## 2. USER SERVICE

### 2.1 Get User Profile
```http
GET /users/{user_id}
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "user_id": "usr_1234567890",
    "username": "johndoe",
    "full_name": "John Doe",
    "email": "user@example.com",
    "phone": "+84123456789",
    "avatar_url": "https://cdn.example.com/avatars/user.jpg",
    "bio": "Software Developer",
    "status": "online",
    "last_seen": "2025-01-15T10:30:00Z",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### 2.2 Update Profile
```http
PUT /users/me
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "full_name": "John Smith",
  "bio": "Senior Software Developer",
  "avatar_url": "https://cdn.example.com/avatars/new.jpg",
  "status_message": "Working remotely"
}

Response (200):
{
  "success": true,
  "data": {
    "user_id": "usr_1234567890",
    "full_name": "John Smith",
    "bio": "Senior Software Developer",
    "updated_at": "2025-01-15T10:30:00Z"
  }
}
```

### 2.3 Search Users
```http
GET /users/search?q={query}&limit=20&offset=0
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "users": [
      {
        "user_id": "usr_9876543210",
        "username": "janedoe",
        "full_name": "Jane Doe",
        "avatar_url": "https://cdn.example.com/avatars/jane.jpg",
        "status": "online"
      }
    ],
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

### 2.4 Get Contacts
```http
GET /users/me/contacts?status=accepted&limit=50&offset=0
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "contacts": [
      {
        "user_id": "usr_9876543210",
        "username": "janedoe",
        "full_name": "Jane Doe",
        "avatar_url": "https://cdn.example.com/avatars/jane.jpg",
        "status": "online",
        "last_seen": "2025-01-15T10:30:00Z",
        "added_at": "2024-06-01T00:00:00Z"
      }
    ],
    "total": 1
  }
}
```

### 2.5 Add Contact
```http
POST /users/me/contacts
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "user_id": "usr_9876543210",
  "message": "Hi! Let's connect"
}

Response (201):
{
  "success": true,
  "data": {
    "contact_id": "cnt_12345",
    "status": "pending",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### 2.6 Block User
```http
POST /users/me/blocked
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "user_id": "usr_9876543210"
}

Response (200):
{
  "success": true,
  "message": "User blocked successfully"
}
```

---

## 3. MESSAGING SERVICE

### 3.1 Get Conversations
```http
GET /conversations?limit=50&offset=0
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "conversations": [
      {
        "conversation_id": "conv_abc123",
        "type": "direct", // direct | group
        "name": "Jane Doe",
        "avatar_url": "https://cdn.example.com/avatars/jane.jpg",
        "participants": [
          {
            "user_id": "usr_9876543210",
            "username": "janedoe",
            "full_name": "Jane Doe"
          }
        ],
        "last_message": {
          "message_id": "msg_xyz789",
          "sender_id": "usr_9876543210",
          "content": "Hello! How are you?",
          "message_type": "text",
          "timestamp": "2025-01-15T10:30:00Z",
          "is_read": false
        },
        "unread_count": 3,
        "is_pinned": false,
        "is_muted": false,
        "updated_at": "2025-01-15T10:30:00Z"
      }
    ],
    "total": 1
  }
}
```

### 3.2 Create Conversation
```http
POST /conversations
Authorization: Bearer {token}
Content-Type: application/json

Request (Direct):
{
  "type": "direct",
  "participant_id": "usr_9876543210"
}

Request (Group):
{
  "type": "group",
  "name": "Project Team",
  "participant_ids": ["usr_9876543210", "usr_1111111111"],
  "avatar_url": "https://cdn.example.com/group.jpg"
}

Response (201):
{
  "success": true,
  "data": {
    "conversation_id": "conv_abc123",
    "type": "group",
    "name": "Project Team",
    "participants": [...],
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### 3.3 Get Messages
```http
GET /conversations/{conversation_id}/messages?limit=50&before={message_id}
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "messages": [
      {
        "message_id": "msg_xyz789",
        "conversation_id": "conv_abc123",
        "sender_id": "usr_9876543210",
        "content_encrypted": "base64_encrypted_content",
        "message_type": "text", // text | image | video | audio | file
        "metadata": {
          "file_name": "document.pdf",
          "file_size": 1024000,
          "mime_type": "application/pdf",
          "thumbnail_url": "https://cdn.example.com/thumb.jpg"
        },
        "reply_to_message_id": null,
        "reactions": {
          "❤️": ["usr_1234567890"],
          "👍": ["usr_9876543210"]
        },
        "delivery_status": {
          "sent": true,
          "delivered": true,
          "read": false,
          "delivered_to": ["usr_9876543210"],
          "read_by": []
        },
        "is_edited": false,
        "is_deleted": false,
        "created_at": "2025-01-15T10:30:00Z",
        "updated_at": "2025-01-15T10:30:00Z"
      }
    ],
    "has_more": true,
    "next_cursor": "msg_abc456"
  }
}
```

### 3.4 Send Message
```http
POST /conversations/{conversation_id}/messages
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "content_encrypted": "base64_encrypted_content",
  "message_type": "text",
  "reply_to_message_id": "msg_abc123",
  "metadata": {},
  "temp_id": "temp_12345" // Client-generated ID for deduplication
}

Response (201):
{
  "success": true,
  "data": {
    "message_id": "msg_xyz789",
    "conversation_id": "conv_abc123",
    "sender_id": "usr_1234567890",
    "temp_id": "temp_12345",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### 3.5 Edit Message
```http
PUT /messages/{message_id}
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "content_encrypted": "base64_new_encrypted_content"
}

Response (200):
{
  "success": true,
  "data": {
    "message_id": "msg_xyz789",
    "is_edited": true,
    "updated_at": "2025-01-15T10:35:00Z"
  }
}
```

### 3.6 Delete Message
```http
DELETE /messages/{message_id}?delete_for=me
Authorization: Bearer {token}

Query params:
- delete_for: "me" | "everyone"

Response (200):
{
  "success": true,
  "message": "Message deleted successfully"
}
```

### 3.7 React to Message
```http
POST /messages/{message_id}/reactions
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "emoji": "❤️"
}

Response (200):
{
  "success": true,
  "data": {
    "message_id": "msg_xyz789",
    "reactions": {
      "❤️": ["usr_1234567890", "usr_9876543210"]
    }
  }
}
```

### 3.8 Mark as Read
```http
POST /conversations/{conversation_id}/read
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "message_id": "msg_xyz789"
}

Response (200):
{
  "success": true,
  "message": "Messages marked as read"
}
```

---

## 4. CALL SERVICE

### 4.1 Initiate Call
```http
POST /calls
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "conversation_id": "conv_abc123",
  "call_type": "video", // video | audio
  "participant_ids": ["usr_9876543210"]
}

Response (201):
{
  "success": true,
  "data": {
    "call_id": "call_xyz123",
    "conversation_id": "conv_abc123",
    "call_type": "video",
    "initiator_id": "usr_1234567890",
    "participants": [
      {
        "user_id": "usr_9876543210",
        "status": "ringing"
      }
    ],
    "signaling_url": "wss://signal.example.com/call_xyz123",
    "turn_servers": [
      {
        "urls": "turn:turn.example.com:3478",
        "username": "user123",
        "credential": "pass123"
      }
    ],
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### 4.2 Answer Call
```http
POST /calls/{call_id}/answer
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "sdp_offer": "v=0\r\no=- 123456789 2 IN IP4..."
}

Response (200):
{
  "success": true,
  "data": {
    "call_id": "call_xyz123",
    "sdp_answer": "v=0\r\no=- 987654321 2 IN IP4...",
    "ice_candidates": [...]
  }
}
```

### 4.3 Reject Call
```http
POST /calls/{call_id}/reject
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "message": "Call rejected"
}
```

### 4.4 End Call
```http
POST /calls/{call_id}/end
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "call_id": "call_xyz123",
    "duration": 180, // seconds
    "ended_at": "2025-01-15T10:33:00Z"
  }
}
```

### 4.5 Get Call History
```http
GET /calls/history?limit=50&offset=0
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "calls": [
      {
        "call_id": "call_xyz123",
        "conversation_id": "conv_abc123",
        "call_type": "video",
        "initiator_id": "usr_1234567890",
        "participants": [...],
        "status": "completed", // completed | missed | rejected | failed
        "duration": 180,
        "started_at": "2025-01-15T10:30:00Z",
        "ended_at": "2025-01-15T10:33:00Z"
      }
    ],
    "total": 1
  }
}
```

---

## 5. FILE SERVICE

### 5.1 Upload File
```http
POST /files/upload
Authorization: Bearer {token}
Content-Type: multipart/form-data

Request:
- file: (binary)
- conversation_id: "conv_abc123"
- file_type: "image" // image | video | audio | document

Response (201):
{
  "success": true,
  "data": {
    "file_id": "file_xyz123",
    "file_name": "photo.jpg",
    "file_size": 1024000,
    "mime_type": "image/jpeg",
    "file_url": "https://cdn.example.com/files/xyz123.jpg",
    "thumbnail_url": "https://cdn.example.com/thumbs/xyz123.jpg",
    "expires_at": "2025-01-22T10:30:00Z"
  }
}
```

### 5.2 Download File
```http
GET /files/{file_id}
Authorization: Bearer {token}

Response (200):
- Binary file data
- Headers:
  - Content-Type: image/jpeg
  - Content-Disposition: attachment; filename="photo.jpg"
```

---

## 6. AI SERVICES

### 6.1 Chatbot Query
```http
POST /ai/chatbot
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "message": "What's the weather like?",
  "conversation_context": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi! How can I help?"}
  ]
}

Response (200):
{
  "success": true,
  "data": {
    "response": "I can help you check the weather, but I'll need your location first.",
    "intent": "weather_query",
    "confidence": 0.95,
    "suggestions": ["Share location", "Enter city name"]
  }
}
```

### 6.2 Translate Message
```http
POST /ai/translate
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "text": "Hello, how are you?",
  "source_lang": "en",
  "target_lang": "vi"
}

Response (200):
{
  "success": true,
  "data": {
    "translated_text": "Xin chào, bạn khỏe không?",
    "source_lang": "en",
    "target_lang": "vi",
    "confidence": 0.98
  }
}
```

### 6.3 Speech-to-Text
```http
POST /ai/speech-to-text
Authorization: Bearer {token}
Content-Type: multipart/form-data

Request:
- audio_file: (binary)
- language: "vi"

Response (200):
{
  "success": true,
  "data": {
    "text": "Xin chào, tôi là John",
    "language": "vi",
    "confidence": 0.92,
    "duration": 3.5
  }
}
```

### 6.4 Smart Reply
```http
POST /ai/smart-reply
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "message": "Are you free for a meeting tomorrow?",
  "context": "work"
}

Response (200):
{
  "success": true,
  "data": {
    "suggestions": [
      "Yes, I'm available",
      "What time works for you?",
      "Sorry, I'm busy tomorrow"
    ]
  }
}
```

### 6.5 Sentiment Analysis
```http
POST /ai/sentiment
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "text": "I'm really happy with this project!"
}

Response (200):
{
  "success": true,
  "data": {
    "sentiment": "positive",
    "score": 0.89,
    "emotions": {
      "joy": 0.85,
      "sadness": 0.05,
      "anger": 0.02,
      "fear": 0.03,
      "surprise": 0.05
    }
  }
}
```

---

## 7. NOTIFICATION SERVICE

### 7.1 Get Notifications
```http
GET /notifications?limit=50&offset=0&unread_only=true
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "notifications": [
      {
        "notification_id": "notif_123",
        "type": "new_message", // new_message | call_missed | contact_request | mention
        "title": "New message from Jane",
        "body": "Hello! How are you?",
        "data": {
          "conversation_id": "conv_abc123",
          "message_id": "msg_xyz789"
        },
        "is_read": false,
        "created_at": "2025-01-15T10:30:00Z"
      }
    ],
    "unread_count": 5,
    "total": 10
  }
}
```

### 7.2 Mark Notification as Read
```http
PUT /notifications/{notification_id}/read
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "message": "Notification marked as read"
}
```

### 7.3 Update Push Token
```http
POST /notifications/push-token
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "device_id": "device_xyz123",
  "platform": "ios", // ios | android | web
  "push_token": "fcm_token_abc123..."
}

Response (200):
{
  "success": true,
  "message": "Push token updated successfully"
}
```

---

## 8. PRESENCE SERVICE (WebSocket)

### 8.1 WebSocket Connection
```
wss://ws.yourdomain.com?token={JWT_TOKEN}

On Connect:
Client sends:
{
  "type": "auth",
  "token": "JWT_TOKEN"
}

Server responds:
{
  "type": "auth_success",
  "user_id": "usr_1234567890",
  "connection_id": "conn_xyz123"
}
```

### 8.2 Presence Events
```javascript
// Set status
Client sends:
{
  "type": "presence_update",
  "status": "online", // online | away | busy | offline
  "status_message": "In a meeting"
}

// Typing indicator
Client sends:
{
  "type": "typing",
  "conversation_id": "conv_abc123",
  "is_typing": true
}

// Subscribe to user presence
Client sends:
{
  "type": "subscribe_presence",
  "user_ids": ["usr_9876543210", "usr_1111111111"]
}

Server pushes:
{
  "type": "presence_changed",
  "user_id": "usr_9876543210",
  "status": "online",
  "last_seen": "2025-01-15T10:30:00Z"
}
```

### 8.3 Real-time Message Events
```javascript
// New message received
Server pushes:
{
  "type": "message_received",
  "conversation_id": "conv_abc123",
  "message": {
    "message_id": "msg_xyz789",
    "sender_id": "usr_9876543210",
    "content_encrypted": "...",
    "created_at": "2025-01-15T10:30:00Z"
  }
}

// Message read receipt
Server pushes:
{
  "type": "message_read",
  "conversation_id": "conv_abc123",
  "message_id": "msg_xyz789",
  "read_by": "usr_9876543210",
  "read_at": "2025-01-15T10:31:00Z"
}

// Incoming call
Server pushes:
{
  "type": "incoming_call",
  "call_id": "call_xyz123",
  "caller": {
    "user_id": "usr_9876543210",
    "full_name": "Jane Doe",
    "avatar_url": "..."
  },
  "call_type": "video"
}
```

---

## 9. ANALYTICS SERVICE

### 9.1 Track Event
```http
POST /analytics/events
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "event_name": "message_sent",
  "properties": {
    "conversation_type": "direct",
    "message_type": "text",
    "has_media": false
  },
  "timestamp": "2025-01-15T10:30:00Z"
}

Response (200):
{
  "success": true,
  "message": "Event tracked"
}
```

### 9.2 Get Usage Statistics
```http
GET /analytics/usage?start_date=2025-01-01&end_date=2025-01-31
Authorization: Bearer {token}

Response (200):
{
  "success": true,
  "data": {
    "messages_sent": 1250,
    "messages_received": 980,
    "calls_made": 45,
    "total_call_duration": 5400, // seconds
    "files_shared": 120,
    "storage_used": 524288000 // bytes
  }
}
```

---

## 10. MEETING SERVICE

### 10.1 Schedule Meeting
```http
POST /meetings
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "title": "Project Discussion",
  "description": "Discuss Q1 goals",
  "start_time": "2025-01-20T14:00:00Z",
  "duration": 60, // minutes
  "participant_ids": ["usr_9876543210", "usr_1111111111"],
  "recurrence": {
    "frequency": "weekly", // daily | weekly | monthly
    "interval": 1,
    "end_date": "2025-03-20T14:00:00Z"
  }
}

Response (201):
{
  "success": true,
  "data": {
    "meeting_id": "meet_abc123",
    "title": "Project Discussion",
    "meeting_url": "https://meet.example.com/meet_abc123",
    "calendar_invite_url": "https://calendar.example.com/...",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

---

## Error Response Format

All errors follow this structure:
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input parameters",
    "details": {
      "email": "Invalid email format",
      "password": "Password must be at least 8 characters"
    }
  },
  "request_id": "req_xyz123",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Common Error Codes
- `AUTHENTICATION_FAILED` (401)
- `AUTHORIZATION_FAILED` (403)
- `NOT_FOUND` (404)
- `VALIDATION_ERROR` (400)
- `RATE_LIMIT_EXCEEDED` (429)
- `INTERNAL_SERVER_ERROR` (500)
- `SERVICE_UNAVAILABLE` (503)

---

## Rate Limiting

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1642348800

Limits:
- Authentication endpoints: 5 requests/minute
- Regular API calls: 1000 requests/hour
- File uploads: 100 requests/hour
- WebSocket messages: 100 messages/minute
```

---

## Pagination

All list endpoints support pagination:
```
GET /endpoint?limit=50&offset=0

Response includes:
{
  "data": [...],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 150,
    "has_next": true,
    "has_prev": false
  }
}
```

---

## Versioning

API versioning in URL: `/v1/`, `/v2/`

Header-based versioning (alternative):
```
API-Version: 2025-01-15
```