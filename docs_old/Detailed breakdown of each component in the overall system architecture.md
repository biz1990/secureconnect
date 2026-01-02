## 1. API Gateway Layer

**API Gateway (Kong/AWS API Gateway/Traefik)**
- Routing requests đến các microservices
- Rate limiting và throttling
- Request/Response transformation
- API versioning (v1, v2)
- CORS handling
- SSL/TLS termination

**WebSocket Gateway**
- Persistent connections cho real-time messaging
- Connection pooling
- Heartbeat/ping-pong mechanism
- Auto-reconnection handling

## 2. Core Services Chi Tiết

### **Auth Service**
```
Responsibilities:
- User registration/login
- JWT token generation & validation
- OAuth2 integration (Google, Facebook, Apple)
- 2FA/MFA
- Session management
- Password reset

Database: PostgreSQL
Cache: Redis (sessions, tokens)
Tech: Node.js/Go
```

### **User Service**
```
Responsibilities:
- User profile CRUD
- Settings management
- Contact/Friend management
- Blocking users
- User search

Database: PostgreSQL (sharded by user_id)
Cache: Redis (user profiles)
API: RESTful + gRPC (internal)
```

### **Messaging Service**
```
Responsibilities:
- Send/receive messages (1-1, group)
- Message delivery status
- Read receipts
- Typing indicators
- Message reactions
- Message history

Database: Cassandra (partition by conversation_id)
Message Queue: Kafka
Protocol: WebSocket + gRPC
Encryption: Client-side E2EE
```

### **Presence Service**
```
Responsibilities:
- Online/offline status
- Last seen
- Currently typing
- Custom status

Database: Redis (in-memory)
Protocol: WebSocket pub/sub
TTL: Auto-expire after 5 minutes
```

### **Call Service**
```
Responsibilities:
- WebRTC signaling (SDP exchange)
- Call initiation/termination
- Call state management
- Call history
- Call quality monitoring

Database: PostgreSQL (call logs)
Real-time: WebSocket
Media: Mediasoup/Janus
```

### **Notification Service**
```
Responsibilities:
- Push notifications (FCM, APNs)
- Email notifications
- SMS notifications
- In-app notifications
- Notification preferences

Message Queue: Kafka consumer
Database: MongoDB (notification history)
Third-party: Twilio, SendGrid, Firebase
```

## 3. AI Services Module

### **AI Gateway**
```
- Load balancing cho AI requests
- Request queuing
- Response caching
- Model versioning
```

### **AI Modules:**
```
1. Chatbot Service
   - NLU/Intent recognition
   - Context management
   - Multi-turn conversations

2. Translation Service
   - Real-time message translation
   - 100+ languages support
   - API: Google Translate/DeepL

3. Speech-to-Text
   - Voice message transcription
   - Multi-language support
   - API: Whisper/Google Speech

4. Smart Reply
   - Context-aware suggestions
   - ML model (BERT-based)
   - Personalized responses

5. Sentiment Analysis
   - Message emotion detection
   - Customer service insights
```

## 4. Database Schema Design

### **User Database (PostgreSQL)**
```sql
Tables:
- users (id, email, username, password_hash, created_at)
- user_profiles (user_id, display_name, avatar_url, bio)
- user_settings (user_id, theme, language, notification_prefs)
- contacts (user_id, contact_user_id, status, created_at)
- blocked_users (user_id, blocked_user_id)

Sharding Strategy: Hash sharding by user_id
Replication: Master-slave (read replicas)
```

### **Message Database (Cassandra)**
```
Table: messages
Partition Key: conversation_id
Clustering Key: timestamp (DESC)
Columns: 
- message_id (UUID)
- sender_id
- content_encrypted
- message_type (text/image/video/file)
- metadata (JSON)
- delivery_status
- created_at

TTL: Optional (auto-delete after X days)
Replication Factor: 3
```

### **Cache Layer (Redis)**
```
Data Types:
- Strings: user sessions, JWT tokens
- Hash: user profiles, settings
- Sets: online users, typing users
- Sorted Sets: message queues
- Pub/Sub: real-time events

Clusters: Redis Cluster mode
Persistence: RDB + AOF
```

## 5. Communication Patterns

**Synchronous (REST/gRPC):**
- Client → API Gateway → Services
- Service-to-service: gRPC (faster, typed)

**Asynchronous (Event-Driven):**
```
Kafka Topics:
- message.sent
- message.delivered
- message.read
- user.online
- user.offline
- call.started
- call.ended
- notification.triggered

Pattern: Event Sourcing + CQRS
```

## 6. Service Deployment

**Container Strategy:**
```yaml
Each service:
- Dockerfile
- Kubernetes Deployment
- Horizontal Pod Autoscaler
- Service (ClusterIP/LoadBalancer)
- ConfigMap/Secret
- Health checks (liveness/readiness)

Example: Messaging Service
- Min replicas: 3
- Max replicas: 20
- CPU threshold: 70%
- Memory threshold: 80%
```

**Multi-Region Deployment:**
```
Regions: Asia-Pacific, Europe, US
- Active-active setup
- Data locality compliance (GDPR)
- Edge locations for media
- Cross-region replication (eventual consistency)
```

## 7. Security Implementation

**E2E Encryption Flow:**
```
1. Key Exchange (Signal Protocol)
   - Identity keys (long-term)
   - Signed pre-keys
   - One-time pre-keys

2. Message Encryption
   - Client encrypts with recipient's public key
   - Server stores encrypted blob
   - Only recipient can decrypt

3. Group Messaging
   - Sender keys
   - Double ratchet algorithm
```
