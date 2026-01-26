# SecureConnect - Comprehensive Architectural Analysis

**Analysis Date:** 2025-01-10  
**Codebase Version:** Current  
**Analysis Scope:** Full Backend System (Go Microservices)

---

## 1. System Summary

### 1.1 What This System Is

SecureConnect is a **secure, real-time communication SaaS platform** that provides three core capabilities:
- **Instant Messaging** (Chat with E2EE support)
- **Video/Audio Calling** (WebRTC-based with SFU signaling)
- **Cloud File Storage** (Secure file sharing)

### 1.2 Primary Purpose

The system enables secure, real-time communication between users with a unique **Hybrid Security Model** that allows users to choose between:
- **Secure Mode (Default):** End-to-End Encryption (E2EE) using Signal Protocol - server cannot read messages
- **Intelligent Mode (Optional):** Opt-out of E2EE to enable server-side AI features (transcription, sentiment analysis, recording)

### 1.3 Target Users

- **Individual Users:** Privacy-conscious users seeking secure messaging
- **Enterprise/Business Users:** Organizations needing recording, AI-powered features, and team collaboration
- **Developers:** Teams integrating communication features into their applications

---

## 2. Technology Stack

### 2.1 Programming Languages

| Language | Purpose | Version |
|-----------|---------|----------|
| **Go (Golang)** | Backend microservices | 1.23 |

### 2.2 Frameworks & Libraries

| Category | Technology | Purpose |
|----------|-------------|---------|
| **Web Framework** | Gin (github.com/gin-gonic/gin) | HTTP routing & middleware |
| **WebSocket** | Gorilla WebSocket (github.com/gorilla/websocket) | Real-time communication |
| **Cryptography** | golang.org/x/crypto | Password hashing, encryption primitives |
| **UUID** | github.com/google/uuid | Unique identifiers |
| **JWT** | github.com/golang-jwt/jwt/v5 | Authentication tokens |
| **Logging** | go.uber.org/zap | Structured logging |
| **Testing** | github.com/stretchr/testify | Unit and integration tests |

### 2.3 Database Drivers

| Database | Driver | Purpose |
|----------|---------|---------|
| **CockroachDB** | github.com/jackc/pgx/v5 | PostgreSQL-compatible SQL driver |
| **Cassandra** | github.com/gocql/gocql | NoSQL wide-column store driver |
| **Redis** | github.com/redis/go-redis/v9 | In-memory cache and pub/sub |
| **MinIO/S3** | github.com/minio/minio-go/v7 | Object storage client |

### 2.4 Infrastructure Dependencies

| Component | Technology | Purpose |
|-----------|-------------|---------|
| **Containerization** | Docker | Application packaging |
| **Orchestration** | Kubernetes (K8s) | Deployment and scaling |
| **Load Balancer** | Nginx | Reverse proxy and TLS termination |
| **CI/CD** | GitHub Actions | Automated deployment pipeline |

---

## 3. Architecture Overview

### 3.1 Architecture Style

**Microservices Architecture** with the following characteristics:
- **Service Decomposition:** 5 independent microservices (API Gateway, Auth, Chat, Video, Storage)
- **Service Communication:** RESTful HTTP for synchronous calls, WebSocket for real-time
- **API Gateway Pattern:** Single entry point with routing, rate limiting, and authentication
- **Polyglot Persistence:** Multiple database types optimized for different data patterns

### 3.2 Logical Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                     Presentation Layer                         │
│  (HTTP Handlers, WebSocket Handlers, Middleware)              │
└────────────────────┬────────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────────┐
│                    Service Layer                              │
│  (Business Logic, Domain Models, Use Cases)                   │
└────────────────────┬────────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────────┐
│                  Repository Layer                              │
│  (Data Access Abstractions, Database Operations)                 │
└────────────────────┬────────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────────┐
│                   Data Layer                                  │
│  (CockroachDB, Cassandra, Redis, MinIO)                     │
└─────────────────────────────────────────────────────────────────┘
```

### 3.3 Component Diagram

```
                    ┌──────────────┐
                    │   Clients    │
                    │ (Flutter)    │
                    └──────┬───────┘
                           │ HTTPS/WSS
                    ┌──────▼────────┐
                    │  API Gateway  │
                    │  (Port 8080)  │
                    └───┬───────┬───┘
                        │       │
        ┌───────────────┼───────┼──────────────┐
        │               │       │              │
┌───────▼──────┐ ┌───▼───┐ ┌─▼────────┐ ┌─▼──────────┐
│ Auth Service  │ │ Chat   │ │ Video    │ │ Storage    │
│ (Port 8081)  │ │Service │ │ Service  │ │ Service    │
│              │ │(8082)  │ │ (8083)   │ │ (8084)    │
└───────┬──────┘ └───┬───┘ └─┬────────┘ └─┬──────────┘
        │              │         │             │
        │              │         │             │
┌───────▼──────────────▼─────────▼─────────────▼──────┐
│                  Data Layer                            │
│  ┌──────────┐ ┌──────────┐ ┌──────┐ ┌────────┐ │
│  │CockroachDB│ │Cassandra │ │Redis │ │ MinIO  │ │
│  │          │ │          │ │      │ │        │ │
│  └──────────┘ └──────────┘ └──────┘ └────────┘ │
└─────────────────────────────────────────────────────┘
```

---

## 4. Project Structure Breakdown

```
secureconnect-backend/
├── cmd/                          # Service entry points
│   ├── api-gateway/              # API Gateway main.go
│   ├── auth-service/              # Authentication service
│   ├── chat-service/             # Chat messaging service
│   ├── video-service/            # Video calling service
│   └── storage-service/          # File storage service
│
├── internal/                     # Private application code
│   ├── auth/                    # Auth domain logic
│   ├── chat/                    # Chat domain logic
│   ├── video/                   # Video domain logic
│   ├── crypto/                  # Cryptography utilities
│   ├── config/                   # Configuration management
│   ├── database/                 # Database connection setup
│   │   ├── cassandra.go          # Cassandra connection
│   │   ├── cockroachdb.go        # CockroachDB connection
│   │   └── redis.go             # Redis connection
│   ├── domain/                   # Domain entities
│   │   ├── user.go               # User entity
│   │   ├── conversation.go       # Conversation entity
│   │   ├── message.go            # Message entity
│   │   ├── call.go               # Call entity
│   │   ├── file.go               # File entity
│   │   └── keys.go              # E2EE key entities
│   ├── handler/                  # HTTP/WS handlers
│   │   ├── http/                 # HTTP handlers
│   │   │   ├── auth/            # Auth endpoints
│   │   │   ├── chat/            # Chat endpoints
│   │   │   ├── video/           # Video endpoints
│   │   │   ├── storage/         # Storage endpoints
│   │   │   ├── keys/            # Key management endpoints
│   │   │   └── crypto/          # Crypto endpoints
│   │   └── ws/                   # WebSocket handlers
│   │       ├── chat_handler.go   # Chat WebSocket hub
│   │       └── signaling_handler.go # Video signaling hub
│   ├── middleware/               # HTTP middleware
│   │   ├── auth.go              # JWT authentication
│   │   ├── cors.go              # CORS handling
│   │   ├── logger.go            # Request logging
│   │   ├── ratelimit.go         # Rate limiting
│   │   └── recovery.go          # Panic recovery
│   ├── models/                   # Data models
│   │   └── user.go              # User model
│   ├── repository/               # Data access layer
│   │   ├── cassandra/           # Cassandra repositories
│   │   │   └── message_repo.go  # Message storage
│   │   ├── cockroach/           # CockroachDB repositories
│   │   │   ├── user_repo.go     # User CRUD
│   │   │   ├── conversation_repo.go # Conversation CRUD
│   │   │   ├── file_repo.go     # File metadata
│   │   │   ├── keys_repo.go     # E2EE keys
│   │   │   └── call_repo.go     # Call records
│   │   └── redis/               # Redis repositories
│   │       ├── directory_repo.go  # Email/username lookup
│   │       ├── presence_repo.go   # Online status
│   │       └── session_repo.go   # Session management
│   └── service/                  # Business logic layer
│       ├── auth/                  # Auth service
│       ├── chat/                  # Chat service
│       ├── video/                 # Video service
│       ├── storage/               # Storage service
│       ├── crypto/                # Crypto service
│       └── conversation/          # Conversation service
│
├── pkg/                         # Public library code
│   ├── jwt/                     # JWT token management
│   ├── logger/                   # Logging utilities
│   ├── response/                 # HTTP response helpers
│   └── database/                # Database config structs
│
├── configs/                      # Configuration files
│   └── nginx.conf               # Nginx reverse proxy config
│
├── scripts/                      # Utility scripts
│   ├── build.sh                  # Build script
│   ├── init-db.sh                # Database initialization
│   ├── cassandra-init.cql        # Cassandra schema
│   ├── cassandra-schema.cql       # Cassandra schema
│   ├── cockroach-init.sql         # CockroachDB schema
│   └── calls-schema.sql          # Call tables schema
│
├── deployments/                  # Deployment configurations
│   ├── docker/                   # Docker-related configs
│   └── k8s/                     # Kubernetes manifests
│       ├── staging/               # Staging environment
│       └── production/            # Production environment
│
├── test/                        # Test files
│   └── integration/             # Integration tests
│
├── logs/                        # Application logs
│   └── logger.go                # Logger setup
│
├── .github/                     # GitHub configurations
│   └── workflows/               # CI/CD workflows
│       └── ci-cd.yml            # GitHub Actions pipeline
│
├── docker-compose.yml             # Local development setup
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── Makefile                     # Build automation
├── .env.example                 # Environment variables template
└── README.md                    # Project documentation
```

---

## 5. Core Modules & Components

### 5.1 API Gateway Module

**Responsibility:** Single entry point for all client requests, routing, and cross-cutting concerns

**Key Files:**
- [`cmd/api-gateway/main.go`](secureconnect-backend/cmd/api-gateway/main.go:1)
- [`internal/middleware/auth.go`](secureconnect-backend/internal/middleware/auth.go:1)
- [`internal/middleware/ratelimit.go`](secureconnect-backend/internal/middleware/ratelimit.go:1)

**Dependencies:**
- Redis (for rate limiting)
- JWT Manager (for authentication)
- All downstream services (via HTTP proxy)

**Features:**
- Reverse proxy to microservices
- JWT authentication
- Rate limiting (100 requests/minute)
- Request logging
- CORS handling
- Health check endpoint

---

### 5.2 Auth Service Module

**Responsibility:** User authentication, registration, session management, and E2EE key storage

**Key Files:**
- [`cmd/auth-service/main.go`](secureconnect-backend/cmd/auth-service/main.go:1)
- [`internal/service/auth/service.go`](secureconnect-backend/internal/service/auth/service.go:1)
- [`internal/handler/http/auth/handler.go`](secureconnect-backend/internal/handler/http/auth/handler.go:1)
- [`internal/repository/cockroach/user_repo.go`](secureconnect-backend/internal/repository/cockroach/user_repo.go:1)
- [`internal/repository/cockroach/keys_repo.go`](secureconnect-backend/internal/repository/cockroach/keys_repo.go:1)

**Dependencies:**
- CockroachDB (users, keys)
- Redis (sessions, directory)

**Features:**
- User registration (email/username/password)
- User login with JWT tokens
- Token refresh mechanism
- Session management
- E2EE key storage (Identity, Signed Pre-Keys, One-Time Pre-Keys)
- Password hashing with bcrypt

---

### 5.3 Chat Service Module

**Responsibility:** Real-time messaging, presence tracking, and message persistence

**Key Files:**
- [`cmd/chat-service/main.go`](secureconnect-backend/cmd/chat-service/main.go:1)
- [`internal/service/chat/service.go`](secureconnect-backend/internal/service/chat/service.go:1)
- [`internal/handler/http/chat/handler.go`](secureconnect-backend/internal/handler/http/chat/handler.go:1)
- [`internal/handler/ws/chat_handler.go`](secureconnect-backend/internal/handler/ws/chat_handler.go:1)
- [`internal/repository/cassandra/message_repo.go`](secureconnect-backend/internal/repository/cassandra/message_repo.go:1)

**Dependencies:**
- Cassandra (messages)
- Redis (presence, pub/sub)

**Features:**
- Send and receive messages
- Message history with pagination
- Real-time message delivery via WebSocket
- User presence (online/offline status)
- Typing indicators
- Message encryption flag (E2EE support)

---

### 5.4 Video Service Module

**Responsibility:** Video/audio call management and WebRTC signaling

**Key Files:**
- [`cmd/video-service/main.go`](secureconnect-backend/cmd/video-service/main.go:1)
- [`internal/service/video/service.go`](secureconnect-backend/internal/service/video/service.go:1)
- [`internal/handler/http/video/handler.go`](secureconnect-backend/internal/handler/http/video/handler.go:1)
- [`internal/handler/ws/signaling_handler.go`](secureconnect-backend/internal/handler/ws/signaling_handler.go:1)
- [`internal/repository/cockroach/call_repo.go`](secureconnect-backend/internal/repository/cockroach/call_repo.go:1)

**Dependencies:**
- CockroachDB (call logs)
- Redis (optional, for signaling coordination)

**Features:**
- Call initiation
- Call joining/leaving
- Call status tracking
- WebRTC signaling via WebSocket (offer/answer/ICE candidates)
- Call participant management
- Call history

**Note:** SFU (Selective Forwarding Unit) logic is planned but not yet fully implemented (TODO comments in code)

---

### 5.5 Storage Service Module

**Responsibility:** File upload, download, and metadata management

**Key Files:**
- [`cmd/storage-service/main.go`](secureconnect-backend/cmd/storage-service/main.go:1)
- [`internal/service/storage/service.go`](secureconnect-backend/internal/service/storage/service.go:1)
- [`internal/handler/http/storage/handler.go`](secureconnect-backend/internal/handler/http/storage/handler.go:1)
- [`internal/repository/cockroach/file_repo.go`](secureconnect-backend/internal/repository/cockroach/file_repo.go:1)

**Dependencies:**
- CockroachDB (file metadata)
- MinIO (object storage)

**Features:**
- Generate presigned upload URLs
- Generate presigned download URLs
- File metadata management
- Storage quota tracking
- File deletion
- Client-side encryption support

---

## 6. Feature Map

### 6.1 Authentication & Authorization

| Feature | Modules Involved |
|---------|-----------------|
| User Registration | Auth Service, API Gateway |
| User Login | Auth Service, API Gateway |
| Token Refresh | Auth Service, API Gateway |
| Session Management | Auth Service, Redis |
| JWT Authentication | All Services (via middleware) |

### 6.2 Messaging

| Feature | Modules Involved |
|---------|-----------------|
| Send Message | Chat Service, Cassandra, Redis |
| Receive Message (Real-time) | Chat Service, WebSocket Hub |
| Message History | Chat Service, Cassandra |
| Presence Tracking | Chat Service, Redis |
| Typing Indicators | Chat Service, WebSocket Hub |
| E2EE Support | Chat Service, Auth Service (Keys) |

### 6.3 Video Calling

| Feature | Modules Involved |
|---------|-----------------|
| Initiate Call | Video Service, CockroachDB |
| Join Call | Video Service, WebSocket Signaling |
| WebRTC Signaling | Video Service, WebSocket Hub |
| Call Status | Video Service, CockroachDB |
| Call History | Video Service, CockroachDB |

### 6.4 File Storage

| Feature | Modules Involved |
|---------|-----------------|
| Upload File | Storage Service, MinIO, CockroachDB |
| Download File | Storage Service, MinIO |
| File Metadata | Storage Service, CockroachDB |
| Storage Quota | Storage Service, CockroachDB |
| Delete File | Storage Service, MinIO, CockroachDB |

---

## 7. Services & Runtime Processes

### 7.1 Long-Running Services

| Service | Port | Purpose |
|---------|-------|---------|
| **API Gateway** | 8080 | Request routing, authentication, rate limiting |
| **Auth Service** | 8081 | User authentication and key management |
| **Chat Service** | 8082 | Messaging and presence |
| **Video Service** | 8083 | Call management and signaling |
| **Storage Service** | 8084 | File operations |

### 7.2 Background Processes

| Process | Location | Purpose |
|---------|-----------|---------|
| **WebSocket Hub (Chat)** | [`internal/handler/ws/chat_handler.go`](secureconnect-backend/internal/handler/ws/chat_handler.go:1) | Manages chat WebSocket connections and broadcasts |
| **WebSocket Hub (Signaling)** | [`internal/handler/ws/signaling_handler.go`](secureconnect-backend/internal/handler/ws/signaling_handler.go:1) | Manages video signaling WebSocket connections |
| **Redis Pub/Sub Subscriber** | Chat Hub | Subscribes to Redis channels for message delivery |

### 7.3 Integration Points

| External Service | Integration Method |
|-----------------|-------------------|
| **CockroachDB** | pgx/v5 driver (connection pooling) |
| **Cassandra** | gocql driver (session management) |
| **Redis** | go-redis/v9 (pub/sub, caching) |
| **MinIO** | minio-go/v7 (S3-compatible API) |

---

## 8. Data Architecture

### 8.1 Storage Systems

| Database | Data Stored | Access Pattern |
|----------|-------------|----------------|
| **CockroachDB** | Users, Conversations, Files, E2EE Keys, Calls, Subscriptions | ACID transactions, complex queries |
| **Cassandra** | Messages, Call Logs, User Activity | Time-series, write-heavy, partitioned |
| **Redis** | Sessions, Directory (email→user_id), Presence, Pub/Sub channels | Fast lookups, real-time pub/sub |
| **MinIO** | File content (images, videos, documents) | Object storage with presigned URLs |

### 8.2 Main Entities

#### User Entity
- **Table:** `users` (CockroachDB)
- **Fields:** user_id, email, username, password_hash, display_name, avatar_url, status, created_at, updated_at
- **Indexes:** email, username, status, created_at

#### Message Entity
- **Table:** `messages` (Cassandra)
- **Fields:** conversation_id, bucket, message_id, sender_id, content, is_encrypted, message_type, metadata, created_at
- **Partition Key:** (conversation_id, bucket)
- **Clustering:** created_at DESC, message_id DESC

#### Conversation Entity
- **Tables:** `conversations`, `conversation_participants`, `conversation_settings` (CockroachDB)
- **Fields:** conversation_id, type, name, created_by, created_at, updated_at
- **Settings:** is_e2ee_enabled, ai_enabled, recording_enabled

#### Call Entity
- **Tables:** `calls`, `call_participants` (CockroachDB)
- **Fields:** call_id, conversation_id, caller_id, call_type, status, started_at, ended_at, duration

#### File Entity
- **Table:** `files` (CockroachDB)
- **Fields:** file_id, user_id, file_name, file_size, content_type, minio_object_key, is_encrypted, status

#### E2EE Key Entities
- **Tables:** `identity_keys`, `signed_pre_keys`, `one_time_pre_keys` (CockroachDB)
- **Purpose:** Signal Protocol key management for E2EE

### 8.3 Data Flow

```
Client Request
    ↓
API Gateway (Auth + Rate Limit)
    ↓
Microservice (Business Logic)
    ↓
Repository Layer (Data Access)
    ↓
Database (CockroachDB/Cassandra/Redis) or MinIO
```

**Message Flow Example:**
1. Client sends message via POST /messages
2. Chat Service validates and saves to Cassandra
3. Chat Service publishes to Redis Pub/Sub channel
4. WebSocket Hub receives from Redis and broadcasts to connected clients

---

## 9. System Flow

### 9.1 Application Startup Flow

```
For each service (api-gateway, auth-service, chat-service, video-service, storage-service):

1. Load environment variables
2. Initialize database connections (CockroachDB, Cassandra, Redis)
3. Initialize JWT Manager (Auth, Gateway, Chat, Video, Storage)
4. Initialize Repositories
5. Initialize Services
6. Initialize Handlers
7. Setup Gin Router with middleware
8. Register routes
9. Start HTTP server on configured port
```

### 9.2 Request Lifecycle (HTTP)

```
1. Client sends HTTP request with JWT token
2. API Gateway receives request
3. Rate limiter checks IP/user quota
4. Auth middleware validates JWT
5. Request is proxied to appropriate microservice
6. Microservice handler processes request
7. Service layer executes business logic
8. Repository layer accesses data
9. Response flows back through the chain
10. Client receives response
```

### 9.3 WebSocket Connection Flow

```
Chat WebSocket:
1. Client connects to /v1/ws/chat?conversation_id=xxx&token=jwt
2. Auth middleware validates JWT
3. Connection is added to Chat Hub
4. Client can send/receive messages
5. Hub broadcasts messages to all clients in conversation
6. Redis Pub/Sub enables cross-instance message delivery

Signaling WebSocket:
1. Client connects to /v1/ws/signaling?call_id=xxx&token=jwt
2. Auth middleware validates JWT
3. Connection is added to Signaling Hub
4. SDP offers/answers and ICE candidates are exchanged
5. Hub forwards messages between participants
```

---

## 10. Observations & Architectural Notes

### 10.1 Architectural Patterns Detected

1. **Microservices Pattern:** Clear separation of concerns across 5 independent services
2. **Repository Pattern:** Data access abstraction with interfaces
3. **Service Layer Pattern:** Business logic separated from handlers
4. **Middleware Pattern:** Cross-cutting concerns (auth, logging, rate limiting)
5. **Hub Pattern:** WebSocket connection management with broadcast channels
6. **Pub/Sub Pattern:** Redis-based message distribution for real-time features

### 10.2 Design Intentions Inferred

1. **Scalability:** Microservices can scale independently; Cassandra partitioning for time-series data
2. **Security by Default:** E2EE enabled by default; JWT authentication on all protected routes
3. **High Performance:** Go for concurrency; Redis for low-latency lookups; Cassandra for write-heavy workloads
4. **Flexibility:** Hybrid security model allows trade-off between privacy and AI features
5. **Observability:** Structured logging with Zap; health check endpoints on all services

### 10.3 Areas of Complexity

1. **Polyglot Persistence:** Managing 4 different storage systems increases operational complexity
2. **E2EE Key Management:** Signal Protocol implementation requires careful key rotation and storage
3. **WebSocket State Management:** Maintaining connection state across service instances requires Redis coordination
4. **Data Consistency:** Eventual consistency in Cassandra vs strong consistency in CockroachDB
5. **Service Discovery:** Docker DNS used in compose; K8s service discovery in production

### 10.4 Coupling Observations

1. **Loose Coupling:** Services communicate via HTTP; no direct database access between services
2. **Tight Coupling (within service):** Handlers depend on Services; Services depend on Repositories
3. **Shared Libraries:** `pkg/` directory contains shared JWT, logger, and response utilities
4. **Configuration Coupling:** All services share similar environment variable patterns

### 10.5 Security Considerations

1. **Authentication:** JWT-based with access/refresh token pattern
2. **Encryption:** Passwords hashed with bcrypt; E2EE using Signal Protocol (planned)
3. **Transport:** TLS required in production (enforced in K8s config)
4. **Authorization:** Role-based access control (RBAC) mentioned in docs
5. **Rate Limiting:** Redis-based token bucket implementation

### 10.6 Potential Limitations

1. **SFU Not Complete:** Video service has TODO comments indicating SFU logic is not fully implemented
2. **AI Service Missing:** Documentation mentions AI service wrapper but no implementation found
3. **Billing Service Missing:** Subscription table exists but no billing service implementation
4. **Conversation Service:** Handler exists but not fully integrated in main.go files
5. **File Upload Complete:** Storage service has placeholder for upload-complete endpoint

### 10.7 Testing Coverage

1. **Unit Tests:** Found for auth, chat, video, storage services
2. **Integration Tests:** Test directory exists with integration test script
3. **Test Utilities:** Mock generation with mockgen mentioned in Makefile

---

## Appendix A: API Endpoints Summary

### Auth Service (Port 8081)
- `POST /v1/auth/register` - User registration
- `POST /v1/auth/login` - User login
- `POST /v1/auth/refresh` - Refresh access token
- `POST /v1/auth/logout` - User logout (protected)
- `GET /v1/auth/profile` - Get user profile (protected)

### Chat Service (Port 8082)
- `POST /v1/messages` - Send message (protected)
- `GET /v1/messages` - Get messages (protected)
- `POST /v1/presence` - Update presence (protected)
- `GET /v1/ws/chat` - WebSocket chat endpoint (protected)

### Video Service (Port 8083)
- `POST /v1/calls/initiate` - Initiate call (protected)
- `POST /v1/calls/:id/end` - End call (protected)
- `POST /v1/calls/:id/join` - Join call (protected)
- `GET /v1/calls/:id` - Get call status (protected)
- `GET /v1/calls/ws/signaling` - WebSocket signaling endpoint (protected)

### Storage Service (Port 8084)
- `POST /v1/storage/upload-url` - Generate upload URL (protected)
- `POST /v1/storage/upload-complete` - Mark upload complete (protected, not implemented)
- `GET /v1/storage/download-url/:file_id` - Generate download URL (protected)
- `DELETE /v1/storage/files/:file_id` - Delete file (protected)
- `GET /v1/storage/quota` - Get storage quota (protected, not implemented)

---

**Analysis Completed:** 2025-01-10  
**Total Files Analyzed:** 50+  
**Lines of Code:** ~15,000+ (backend only)
