# SecureConnect Backend API Documentation

## Base URL
```
Production: https://api.secureconnect.io/v1
Development: http://localhost:8080/v1
```

## Authentication
All protected endpoints require JWT token in Authorization header:
```
Authorization: Bearer <access_token>
```

---

## 📱 Authentication API

### Register User
```http
POST /auth/register
```

**Request:**
```json
{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "securePassword123",
  "display_name": "John Doe"
}
```

**Response (201):**
```json
{
  "success": true,
  "data": {
    "user": {
      "user_id": "uuid",
      "email": "user@example.com",
      "username": "johndoe",
      "display_name": "John Doe",
      "created_at": "2026-01-09T10:00:00Z"
    },
    "access_token": "eyJhbGc...",
    "refresh_token": "eyJhbGc...",
    "expires_at": "2026-01-09T10:15:00Z"
  }
}
```

### Login
```http
POST /auth/login
```

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securePassword123"
}
```

**Response (200):** Same as register

### Refresh Token
```http
POST /auth/refresh
```

**Request:**
```json
{
  "refresh_token": "eyJhbGc..."
}
```

**Response (200):** New tokens

### Logout
```http
POST /auth/logout
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

### Get Profile
```http
GET /auth/profile
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "email": "user@example.com",
    "username": "johndoe",
    "display_name": "John Doe",
    "status": "online",
    "created_at": "2026-01-09T10:00:00Z"
  }
}
```

---

## 🔐 E2EE Crypto API

### Upload Public Keys
```http
POST /keys/upload
Authorization: Bearer <token>
```

**Request:**
```json
{
  "identity_key": "base64_ed25519_public_key",
  "signed_pre_key": {
    "key_id": 1,
    "public_key": "base64_x25519_public_key",
    "signature": "base64_signature"
  },
  "one_time_pre_keys": [
    {
      "key_id": 1,
      "public_key": "base64_x25519_public_key"
    }
    // ... 20-100 keys
  ]
}
```

**Response (201):**
```json
{
  "success": true,
  "data": {
    "message": "Keys uploaded successfully",
    "one_time_keys": 20
  }
}
```

### Get Pre-Key Bundle
```http
GET /keys/:user_id
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "identity_key": "base64_key",
    "signed_pre_key": {
      "key_id": 1,
      "public_key": "base64_key",
      "signature": "base64_signature"
    },
    "one_time_pre_key": {
      "key_id": 5,
      "public_key": "base64_key"
    }
  }
}
```

### Rotate Signed Pre-Key
```http
POST /keys/rotate
Authorization: Bearer <token>
```

**Request:**
```json
{
  "new_signed_pre_key": {
    "key_id": 2,
    "public_key": "base64_key",
    "signature": "base64_signature"
  },
  "new_one_time_keys": [...]
}
```

---

## 💬 Conversation API

### Create Conversation
```http
POST /conversations
Authorization: Bearer <token>
```

**Request:**
```json
{
  "title": "Project Discussion",
  "type": "direct",  // "direct" or "group"
  "participant_ids": ["uuid1", "uuid2"],
  "is_e2ee_enabled": true
}
```

**Response (201):**
```json
{
  "success": true,
  "data": {
    "conversation_id": "uuid",
    "title": "Project Discussion",
    "type": "direct",
    "created_by": "uuid",
    "created_at": "2026-01-09T10:00:00Z"
  }
}
```

### List Conversations
```http
GET /conversations?limit=20&offset=0
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "conversations": [
      {
        "conversation_id": "uuid",
        "title": "Project Discussion",
        "type": "direct",
        "created_at": "2026-01-09T10:00:00Z"
      }
    ],
    "limit": 20,
    "offset": 0
  }
}
```

### Update E2EE Settings
```http
PUT /conversations/:id/settings
Authorization: Bearer <token>
```

**Request:**
```json
{
  "is_e2ee_enabled": false  // Opt-out encryption
}
```

### Add Participants
```http
POST /conversations/:id/participants
Authorization: Bearer <token>
```

**Request:**
```json
{
  "user_ids": ["uuid3", "uuid4"]
}
```

---

## 💬 Messaging API

### Send Message
```http
POST /messages
Authorization: Bearer <token>
```

**Request:**
```json
{
  "conversation_id": "uuid",
  "content": "Hello, World!",  // Or encrypted payload
  "is_encrypted": false,
  "message_type": "text",  // text, image, video, file
  "metadata": {
    "ai_summary": "..."  // Only if is_encrypted=false
  }
}
```

**Response (201):**
```json
{
  "success": true,
  "data": {
    "message_id": "uuid",
    "conversation_id": "uuid",
    "sender_id": "uuid",
    "content": "Hello, World!",
    "is_encrypted": false,
    "message_type": "text",
    "created_at": "2026-01-09T10:00:00Z"
  }
}
```

### Get Messages
```http
GET /messages?conversation_id=uuid&limit=20&page_state=base64
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "messages": [...],
    "next_page_state": "base64_encoded",
    "has_more": true
  }
}
```

### Update Presence
```http
POST /presence
Authorization: Bearer <token>
```

**Request:**
```json
{
  "online": true
}
```

---

## 📁 Storage API

### Request Upload URL
```http
POST /storage/upload-url
Authorization: Bearer <token>
```

**Request:**
```json
{
  "file_name": "document.pdf",
  "file_size": 1048576,
  "content_type": "application/pdf",
  "is_encrypted": true
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "file_id": "uuid",
    "upload_url": "https://minio.../presigned-url",
    "expires_at": "2026-01-09T10:15:00Z"
  }
}
```

**Client then uploads directly to MinIO:**
```bash
curl -X PUT "$UPLOAD_URL" \
  -H "Content-Type: application/pdf" \
  --data-binary @file.pdf
```

### Complete Upload
```http
POST /storage/upload-complete
Authorization: Bearer <token>
```

**Request:**
```json
{
  "file_id": "uuid"
}
```

### Request Download URL
```http
GET /storage/download-url/:file_id
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "download_url": "https://minio.../presigned-url"
  }
}
```

### Get Storage Quota
```http
GET /storage/quota
Authorization: Bearer <token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "used": 5368709120,      // 5GB in bytes
    "total": 10737418240,    // 10GB in bytes
    "available": 5368709120,
    "usage_percentage": 50.0
  }
}
```

### Delete File
```http
DELETE /storage/files/:file_id
Authorization: Bearer <token>
```

---

## 🌐 WebSocket Chat API

### Connect
```
ws://localhost:8082/v1/ws/chat?conversation_id=<uuid>
Authorization: Bearer <token> (in query or header)
```

### Message Format

**Client → Server:**
```json
{
  "type": "chat",           // chat, typing, read
  "content": "Hello!",
  "is_encrypted": false,
  "message_type": "text"
}
```

**Server → Client:**
```json
{
  "type": "chat",
  "conversation_id": "uuid",
  "sender_id": "uuid",
  "message_id": "uuid",
  "content": "Hello!",
  "is_encrypted": false,
  "message_type": "text",
  "timestamp": "2026-01-09T10:00:00Z"
}
```

**System Messages:**
```json
{
  "type": "user_joined",    // or "user_left"
  "conversation_id": "uuid",
  "sender_id": "uuid",
  "timestamp": "2026-01-09T10:00:00Z"
}
```

**Typing Indicator:**
```json
{
  "type": "typing",
  "conversation_id": "uuid",
  "sender_id": "uuid"
}
```

**Read Receipt:**
```json
{
  "type": "read",
  "conversation_id": "uuid",
  "sender_id": "uuid",
  "message_id": "uuid"
}
```

---

## 📊 Error Responses

**400 Bad Request:**
```json
{
  "success": false,
  "error": "Validation error: email is required"
}
```

**401 Unauthorized:**
```json
{
  "success": false,
  "error": "Invalid or expired token"
}
```

**404 Not Found:**
```json
{
  "success": false,
  "error": "Resource not found"
}
```

**429 Too Many Requests:**
```json
{
  "success": false,
  "error": "Rate limit exceeded. Try again in 60 seconds"
}
```

**500 Internal Server Error:**
```json
{
  "success": false,
  "error": "Internal server error"
}
```

---

## 🔧 Rate Limits

- **Default**: 100 requests per minute per user/IP
- **WebSocket**: Unlimited messages (within reasonable usage)

---

## 📝 Notes

1. **E2EE Flow**: Client handles all encryption/decryption. Server only stores ciphertext.
2. **File Upload**: Two-step process (request URL → upload to MinIO → mark complete)
3. **Pagination**: Use cursor-based for messages, offset-based for lists
4. **Timestamps**: Always UTC in ISO 8601 format
5. **UUIDs**: All IDs are UUID v4
