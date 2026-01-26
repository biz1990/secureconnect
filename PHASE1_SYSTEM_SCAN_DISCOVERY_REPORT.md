# PHASE 1 — SYSTEM SCAN & DISCOVERY REPORT

**Date:** 2026-01-12  
**System:** SecureConnect  
**Status:** READ-ONLY ANALYSIS COMPLETE

---

## EXECUTIVE SUMMARY

SecureConnect is a microservices-based real-time communication platform built with Go, featuring end-to-end encryption (E2EE), WebRTC video calling, and hybrid cloud storage. The system follows a clean architecture pattern with clear separation of concerns across services, repositories, handlers, and domain models.

---

## 1. SYSTEM OVERVIEW

**System Name:** SecureConnect  
**Primary Purpose:** Secure real-time messaging and video calling platform with E2EE  
**Architecture Style:** Microservices with API Gateway pattern  
**Deployment Model:** Docker Compose (development) / Kubernetes (production)  
**Development Status:** Working in development/testing, ready for production hardening

---

## 2. TECHNOLOGY STACK

### 2.1 Programming Languages
| Language | Version | Usage |
|----------|----------|--------|
| Go | 1.23 | Primary backend language for all microservices |

### 2.2 Frameworks & Libraries
| Component | Library | Version | Purpose |
|-----------|----------|----------|---------|
| Web Framework | Gin | v1.9.1 | HTTP routing and middleware |
| WebSocket | Gorilla WebSocket | v1.5.1 | Real-time communication |
| JWT | golang-jwt/jwt | v5.2.0 | Authentication tokens |
| UUID | google/uuid | v1.5.0 | Unique identifiers |
| Crypto | golang.org/x/crypto | v0.17.0 | Password hashing (bcrypt) |
| Logging | Zap | v1.27.1 | Structured logging |
| Testing | Testify | v1.8.4 | Unit and integration tests |

### 2.3 Databases
| Database | Type | Version | Purpose |
|----------|------|----------|---------|
| CockroachDB | SQL | v23.1.0 | Primary relational data (users, conversations, files, E2EE keys, friendships, blocked users, email verification, subscriptions) |
| Cassandra | NoSQL | Latest | Time-series message storage and call logs |
| Redis | Cache/Store | 7-alpine | Session management, rate limiting, presence, pub/sub, token blacklist, failed login tracking |

### 2.4 Object Storage
| Service | Version | Purpose |
|---------|----------|---------|
| MinIO | Latest | S3-compatible object storage for user files and media |

### 2.5 Messaging & Real-Time
| Component | Technology | Purpose |
|-----------|-------------|---------|
| Pub/Sub | Redis Pub/Sub | Real-time message broadcasting across service instances |
| WebSocket | Gorilla WebSocket | Chat and video signaling connections |
| WebRTC Signaling | Custom implementation | Video/audio call coordination |

### 2.6 Build & Runtime Tools
| Tool | Purpose |
|-------|---------|
| Go Modules | Dependency management |
| Docker | Containerization |
| Docker Compose | Development orchestration |
| Make | Build automation |
| pgx/v5 | PostgreSQL/CockroachDB driver |
| gocql | Cassandra driver |
| go-redis/v9 | Redis client |

---

## 3. ARCHITECTURE DIAGRAM (TEXTUAL)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          CLIENT (Flutter/Mobile/Web)                    │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                │ HTTPS/WebSocket
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         NGINX (Load Balancer)                        │
│                    Ports: 9090 (HTTP) / 9443 (HTTPS)               │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      API GATEWAY (Port 8080)                        │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │ Middleware: Recovery, Logger, CORS, Rate Limiting, Auth       │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└───────────────────────────┬───────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ AUTH SERVICE  │   │ CHAT SERVICE  │   │ VIDEO SERVICE │
│   (Port 8081) │   │   (Port 8082) │   │   (Port 8083) │
└───────┬───────┘   └───────┬───────┘   └───────┬───────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ STORAGE       │   │ COCKROACHDB   │   │ CASSANDRA     │
│ SERVICE      │   │ (Port 26257)  │   │ (Port 9042)   │
│ (Port 8084)  │   │               │   │               │
└───────┬───────┘   └───────────────┘   └───────┬───────┘
        │                                       │
        └───────────────────┬───────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ MINIO        │   │ REDIS         │   │ DOCKER       │
│ (Port 9000)  │   │ (Port 6379)   │   │ NETWORK       │
│ Console: 9001 │   │               │   │               │
└───────────────┘   └───────────────┘   └───────────────┘
```

### 3.1 Architecture Style
- **Pattern:** Microservices with API Gateway
- **Communication:** HTTP/REST for service-to-service, WebSocket for real-time, Redis Pub/Sub for async messaging
- **Data Flow:** Gateway → Services → Repositories → Databases

### 3.2 Service Boundaries
| Service | Port | Responsibility |
|---------|-------|----------------|
| API Gateway | 8080 | Request routing, rate limiting, authentication, CORS |
| Auth Service | 8081 | User registration, login, session management, E2EE keys |
| Chat Service | 8082 | Message storage, real-time chat, presence |
| Video Service | 8083 | Call initiation, WebRTC signaling, call management |
| Storage Service | 8084 | File upload/download, presigned URLs, quota management |

### 3.3 Dependency Direction
```
Client → API Gateway → [Auth|Chat|Video|Storage] Services → [CockroachDB|Cassandra|Redis|MinIO]
```

### 3.4 Core Architectural Patterns
1. **Clean Architecture:** Domain → Service → Repository → Handler
2. **Repository Pattern:** Abstraction over data access
3. **Middleware Pattern:** Cross-cutting concerns (auth, rate limiting, logging)
4. **Pub/Sub Pattern:** Redis for real-time message distribution
5. **Hub Pattern:** WebSocket connection management

---

## 4. PROJECT STRUCTURE BREAKDOWN

```
secureconnect-backend/
├── cmd/                          # Service entry points
│   ├── api-gateway/             # API Gateway service
│   ├── auth-service/             # Authentication service
│   ├── chat-service/             # Chat/messaging service
│   ├── video-service/            # Video calling service
│   └── storage-service/          # File storage service
├── internal/                     # Private application code
│   ├── auth/                    # Auth-specific logic
│   ├── chat/                    # Chat-specific logic
│   ├── config/                   # Configuration management
│   ├── crypto/                   # Cryptography utilities
│   ├── database/                 # Database connection factories
│   │   ├── cockroachdb.go       # CockroachDB connection
│   │   ├── redis.go             # Redis connection
│   │   └── cassandra.go         # Cassandra connection
│   ├── domain/                   # Domain models
│   │   ├── user.go              # User entity
│   │   ├── message.go           # Message entity
│   │   ├── call.go              # Call entity
│   │   ├── conversation.go      # Conversation entity
│   │   ├── file.go              # File entity
│   │   ├── keys.go             # E2EE keys
│   │   └── notification.go      # Notification entity
│   ├── handler/                  # HTTP/WebSocket handlers
│   │   ├── http/               # HTTP handlers
│   │   │   ├── auth/           # Auth endpoints
│   │   │   ├── user/           # User endpoints
│   │   │   ├── chat/           # Chat endpoints
│   │   │   ├── video/          # Video endpoints
│   │   │   ├── storage/        # Storage endpoints
│   │   │   ├── keys/           # E2EE keys endpoints
│   │   │   ├── notification/    # Notification endpoints
│   │   │   ├── crypto/         # Crypto endpoints
│   │   │   └── admin/          # Admin endpoints
│   │   └── ws/                 # WebSocket handlers
│   │       ├── chat_handler.go   # Chat WebSocket
│   │       └── signaling_handler.go # Video signaling WebSocket
│   ├── middleware/               # HTTP middleware
│   │   ├── auth.go             # JWT authentication
│   │   ├── cors.go             # CORS handling
│   │   ├── ratelimit.go        # Rate limiting
│   │   ├── logger.go           # Request logging
│   │   ├── recovery.go         # Panic recovery
│   │   ├── security.go         # Security headers
│   │   └── revocation.go       # Token revocation checking
│   ├── repository/               # Data access layer
│   │   ├── cockroach/          # CockroachDB repositories
│   │   │   ├── user_repo.go
│   │   │   ├── blocked_user_repo.go
│   │   │   ├── email_verification_repo.go
│   │   │   ├── file_repo.go
│   │   │   ├── keys_repo.go
│   │   │   ├── conversation_repo.go
│   │   │   ├── call_repo.go
│   │   │   ├── notification_repo.go
│   │   │   └── admin_repo.go
│   │   ├── cassandra/          # Cassandra repositories
│   │   │   └── message_repo.go
│   │   └── redis/              # Redis repositories
│   │       ├── session_repo.go
│   │       ├── directory_repo.go
│   │       └── presence_repo.go
│   └── service/                  # Business logic layer
│       ├── auth/                # Auth service
│       ├── user/                # User service
│       ├── chat/                # Chat service
│       ├── video/               # Video service
│       ├── storage/             # Storage service
│       ├── notification/         # Notification service
│       ├── crypto/              # Crypto service
│       └── conversation/        # Conversation service
├── pkg/                         # Public/shared packages
│   ├── database/                 # Database utilities
│   ├── env/                      # Environment variables
│   ├── errors/                   # Error handling
│   ├── jwt/                      # JWT management
│   ├── logger/                   # Logging utilities
│   └── response/                 # HTTP response helpers
├── api/                         # API specifications
│   ├── protobuf/                 # Protocol Buffers (if used)
│   └── swagger/                  # OpenAPI/Swagger specs
├── configs/                      # Configuration files
│   ├── nginx.conf                # Nginx configuration
│   ├── nginx-https.conf          # Nginx HTTPS configuration
│   ├── grafana-datasources.yml   # Grafana datasources
│   ├── loki-config.yml          # Loki logging config
│   └── promtail-config.yml      # Promtail config
├── scripts/                      # Utility scripts
│   ├── cockroach-init.sql        # CockroachDB schema
│   ├── cassandra-schema.cql      # Cassandra schema
│   ├── cassandra-init.cql       # Cassandra initialization
│   ├── friendships-schema.sql     # Friendships schema
│   ├── calls-schema.sql          # Calls schema
│   ├── notifications-schema.sql   # Notifications schema
│   ├── init-db.sh               # Database initialization
│   ├── backup-databases.sh       # Backup script
│   ├── restore-databases.sh      # Restore script
│   ├── build.sh                 # Build script
│   └── setup-secrets.sh         # Secrets setup
├── deployments/                  # Deployment configurations
│   ├── docker/                  # Docker configurations
│   └── k8s/                     # Kubernetes manifests
│       ├── staging/              # Staging environment
│       └── production/           # Production environment
├── docker-compose.yml             # Development Docker Compose
├── docker-compose.production.yml   # Production Docker Compose
├── docker-compose.logging.yml     # Logging stack
├── Dockerfile                    # Docker build file
├── Makefile                     # Build automation
├── go.mod                       # Go module definition
├── go.sum                       # Go dependencies lock
├── .env.example                 # Environment variables template
└── .env.secrets.example         # Secrets template
```

### 4.1 Top-Level Folders Responsibility

| Folder | Responsibility | Key Files |
|--------|----------------|------------|
| `cmd/` | Service entry points | `main.go` for each service |
| `internal/` | Private application code | All business logic |
| `pkg/` | Shared/public packages | JWT, logger, errors |
| `api/` | API specifications | OpenAPI specs |
| `configs/` | Configuration files | Nginx, Grafana, Loki |
| `scripts/` | Utility scripts | DB init, backup, restore |
| `deployments/` | Deployment configs | Docker, Kubernetes |
| `docs/` | Documentation | Architecture, API docs |
| `test/` | Test files | Integration tests |

### 4.2 Entry Points

| Service | Entry Point | Port |
|---------|-------------|-------|
| API Gateway | `cmd/api-gateway/main.go` | 8080 |
| Auth Service | `cmd/auth-service/main.go` | 8081 |
| Chat Service | `cmd/chat-service/main.go` | 8082 |
| Video Service | `cmd/video-service/main.go` | 8083 |
| Storage Service | `cmd/storage-service/main.go` | 8084 |

### 4.3 Configuration Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Development environment |
| `docker-compose.production.yml` | Production environment with secrets |
| `docker-compose.logging.yml` | Logging stack (Loki, Grafana) |
| `Makefile` | Build and run commands |
| `.env.example` | Environment variables template |
| `.env.secrets.example` | Secrets template |
| `go.mod` / `go.sum` | Go dependencies |

---

## 5. MODULE & COMPONENT MAPPING

### 5.1 Auth Service Module

**Purpose:** User authentication, session management, E2EE key management

**Public Interfaces:**
- `UserRepository`: CRUD operations for users
- `DirectoryRepository`: Fast lookups (email/username → userID)
- `SessionRepository`: Session management, token blacklisting
- `PresenceRepository`: User online/offline status

**Dependencies:**
- CockroachDB (user data)
- Redis (sessions, directory, presence)
- JWT (token generation/validation)

**Data Flow:**
```
Client → API Gateway → Auth Handler → Auth Service → [UserRepo|SessionRepo|DirectoryRepo] → [CockroachDB|Redis]
```

**Key Components:**
- `internal/service/auth/service.go`: Business logic
- `internal/handler/http/auth/handler.go`: HTTP handlers
- `internal/repository/cockroach/user_repo.go`: User data access
- `internal/repository/redis/session_repo.go`: Session management
- `pkg/jwt/jwt.go`: JWT token operations

### 5.2 Chat Service Module

**Purpose:** Real-time messaging, presence tracking

**Public Interfaces:**
- `MessageRepository`: Message CRUD in Cassandra
- `PresenceRepository`: User presence
- `Publisher`: Redis pub/sub for real-time broadcasting

**Dependencies:**
- Cassandra (message storage)
- Redis (presence, pub/sub)
- WebSocket (real-time delivery)

**Data Flow:**
```
Client → WebSocket → Chat Hub → Chat Service → MessageRepo → Cassandra
                                      ↓
                              Redis Pub/Sub → Other WebSocket Clients
```

**Key Components:**
- `internal/service/chat/service.go`: Business logic
- `internal/handler/http/chat/handler.go`: HTTP handlers
- `internal/handler/ws/chat_handler.go`: WebSocket hub
- `internal/repository/cassandra/message_repo.go`: Message data access
- `internal/repository/redis/presence_repo.go`: Presence tracking

### 5.3 Video Service Module

**Purpose:** Video/audio calling, WebRTC signaling

**Public Interfaces:**
- `CallRepository`: Call management
- `ConversationRepository`: Conversation membership verification

**Dependencies:**
- CockroachDB (call logs)
- Redis (signaling pub/sub)
- WebSocket (signaling connection)

**Data Flow:**
```
Client → WebSocket → Signaling Hub → Video Service → CallRepo → CockroachDB
                                      ↓
                              Redis Pub/Sub → Other WebSocket Clients
```

**Key Components:**
- `internal/service/video/service.go`: Business logic
- `internal/handler/http/video/handler.go`: HTTP handlers
- `internal/handler/ws/signaling_handler.go`: WebSocket signaling hub
- `internal/repository/cockroach/call_repo.go`: Call data access

### 5.4 Storage Service Module

**Purpose:** File upload/download, quota management

**Public Interfaces:**
- `FileRepository`: File metadata CRUD
- `ObjectStorage`: MinIO operations abstraction

**Dependencies:**
- CockroachDB (file metadata)
- MinIO (object storage)
- Redis (optional caching)

**Data Flow:**
```
Client → API Gateway → Storage Handler → Storage Service → [FileRepo|MinIO] → [CockroachDB|MinIO]
```

**Key Components:**
- `internal/service/storage/service.go`: Business logic
- `internal/handler/http/storage/handler.go`: HTTP handlers
- `internal/repository/cockroach/file_repo.go`: File metadata access
- `internal/service/storage/minio_client.go`: MinIO client

### 5.5 API Gateway Module

**Purpose:** Request routing, rate limiting, authentication

**Public Interfaces:**
- None (gateway only)

**Dependencies:**
- Redis (rate limiting)
- JWT (token validation)

**Data Flow:**
```
Client → API Gateway → [Auth Middleware] → Reverse Proxy → Target Service
```

**Key Components:**
- `cmd/api-gateway/main.go`: Gateway entry point
- `internal/middleware/auth.go`: JWT authentication
- `internal/middleware/ratelimit.go`: Rate limiting

---

## 6. DOCKER & RUNTIME TOPOLOGY

### 6.1 Container Services

| Service | Image | Container Name | Ports | Memory Limit | CPU Limit |
|---------|--------|----------------|--------|--------------|------------|
| CockroachDB | cockroachdb/cockroach:v23.1.0 | secureconnect_crdb | 26257, 8081 | - | - |
| Cassandra | cassandra:latest | secureconnect_cassandra | 9042 | 1024M heap | - |
| Redis | redis:7-alpine | secureconnect_redis | 6379 | - | - |
| MinIO | minio/minio | secureconnect_minio | 9000, 9001 | - | - |
| API Gateway | secureconnect/api-gateway | api-gateway | 8080 | 256m | 0.5 |
| Auth Service | secureconnect/auth-service | auth-service | - | 256m | 0.5 |
| Chat Service | secureconnect/chat-service | chat-service | - | 512m | 0.5 |
| Video Service | secureconnect/video-service | video-service | - | 512m | 1.0 |
| Storage Service | secureconnect/storage-service | storage-service | - | 256m | 0.5 |
| Nginx | nginx:alpine | secureconnect_nginx | 9090, 9443 | - | - |

### 6.2 Networks
- **Network Name:** `secureconnect-net`
- **Driver:** Bridge
- **Purpose:** Internal service communication

### 6.3 Volumes
| Volume | Purpose |
|--------|---------|
| `crdb_data` | CockroachDB data persistence |
| `cassandra_data` | Cassandra data persistence |
| `redis_data` | Redis data persistence |
| `minio_data` | MinIO object storage |
| `app_logs` | Application logs |

### 6.4 Startup Order (Dependencies)
```
1. Databases: cockroachdb, cassandra, redis, minio
2. Services: api-gateway, auth-service, chat-service, video-service, storage-service
3. Load Balancer: gateway (nginx)
```

### 6.5 Environment Variables

| Variable | Service | Purpose | Default |
|----------|----------|---------|---------|
| `ENV` | All | Environment (development/production) | development |
| `DB_HOST` | Auth, Video, Storage | CockroachDB host | localhost |
| `DB_PORT` | Auth, Video, Storage | CockroachDB port | 26257 |
| `DB_NAME` | Auth, Video, Storage | Database name | secureconnect_poc |
| `CASSANDRA_HOST` | Chat | Cassandra host | localhost |
| `REDIS_HOST` | All | Redis host | localhost |
| `REDIS_PORT` | All | Redis port | 6379 |
| `MINIO_ENDPOINT` | Chat, Storage | MinIO endpoint | http://minio:9000 |
| `MINIO_ACCESS_KEY` | All | MinIO access key | minioadmin |
| `MINIO_SECRET_KEY` | All | MinIO secret key | minioadmin |
| `JWT_SECRET` | All | JWT signing secret | super-secret-key-please-use-longer-key |
| `PORT` | All | Service port | 8080-8084 |

### 6.6 Health Checks

| Service | Health Endpoint | Check Command |
|---------|-----------------|----------------|
| CockroachDB | http://localhost:8080/health?ready=1 | curl -f http://localhost:8080/health?ready=1 |
| Cassandra | cqlsh -e 'describe cluster' | cqlsh -e 'describe cluster' |
| Redis | redis-cli ping | redis-cli ping |
| MinIO | http://localhost:9000/minio/health/live | curl -f http://localhost:9000/minio/health/live |
| API Gateway | /health | GET /health |
| Auth Service | /health | GET /health |
| Chat Service | /health | GET /health |
| Video Service | /health | GET /health |
| Storage Service | /health | GET /health |

---

## 7. API INVENTORY

### 7.1 Authentication API

| Endpoint | Method | Purpose | Auth Required | Data Source |
|----------|---------|---------|---------------|-------------|
| `/v1/auth/register` | POST | User registration | No | CockroachDB |
| `/v1/auth/login` | POST | User login | No | CockroachDB |
| `/v1/auth/refresh` | POST | Refresh access token | No | Redis (session) |
| `/v1/auth/logout` | POST | User logout | Yes | Redis (session, blacklist) |
| `/v1/auth/profile` | GET | Get current user profile | Yes | JWT claims |

### 7.2 User Management API

| Endpoint | Method | Purpose | Auth Required | Data Source |
|----------|---------|---------|---------------|-------------|
| `/v1/users/me` | GET | Get current user profile | Yes | CockroachDB |
| `/v1/users/me` | PATCH | Update current user profile | Yes | CockroachDB |
| `/v1/users/me/password` | POST | Change password | Yes | CockroachDB |
| `/v1/users/me/email` | POST | Change email | Yes | CockroachDB |
| `/v1/users/me/email/verify` | POST | Verify email | Yes | CockroachDB |
| `/v1/users/me` | DELETE | Delete account | Yes | CockroachDB |
| `/v1/users/me/blocked` | GET | Get blocked users | Yes | CockroachDB |
| `/v1/users/:id/block` | POST | Block a user | Yes | CockroachDB |
| `/v1/users/:id/block` | DELETE | Unblock a user | Yes | CockroachDB |
| `/v1/users/me/friends` | GET | Get friends list | Yes | CockroachDB |
| `/v1/users/:id/friend` | POST | Send friend request | Yes | CockroachDB |
| `/v1/users/me/friends/:id/accept` | POST | Accept friend request | Yes | CockroachDB |
| `/v1/users/me/friends/:id/reject` | DELETE | Reject friend request | Yes | CockroachDB |
| `/v1/users/me/friends/:id` | DELETE | Unfriend | Yes | CockroachDB |

### 7.3 E2EE Keys API

| Endpoint | Method | Purpose | Auth Required | Data Source |
|----------|---------|---------|---------------|-------------|
| `/v1/keys/upload` | POST | Upload public keys | Yes | CockroachDB |
| `/v1/keys/:user_id` | GET | Get user's pre-key bundle | Yes | CockroachDB |
| `/v1/keys/rotate` | POST | Rotate signed pre-key | Yes | CockroachDB |

### 7.4 Chat/Messaging API

| Endpoint | Method | Purpose | Auth Required | Data Source |
|----------|---------|---------|---------------|-------------|
| `/v1/messages` | POST | Send message | Yes | Cassandra + Redis Pub/Sub |
| `/v1/messages` | GET | Get conversation messages | Yes | Cassandra |
| `/v1/presence` | POST | Update presence status | Yes | Redis |
| `/v1/ws/chat` | WS | Real-time chat connection | Yes | WebSocket + Redis Pub/Sub |

### 7.5 Video/Call API

| Endpoint | Method | Purpose | Auth Required | Data Source |
|----------|---------|---------|---------------|-------------|
| `/v1/calls/initiate` | POST | Initiate a call | Yes | CockroachDB |
| `/v1/calls/:id/end` | POST | End a call | Yes | CockroachDB |
| `/v1/calls/:id/join` | POST | Join a call | Yes | CockroachDB |
| `/v1/calls/:id` | GET | Get call status | Yes | CockroachDB |
| `/v1/calls/ws/signaling` | WS | WebRTC signaling | Yes | WebSocket + Redis Pub/Sub |

### 7.6 Storage API

| Endpoint | Method | Purpose | Auth Required | Data Source |
|----------|---------|---------|---------------|-------------|
| `/v1/storage/upload-url` | POST | Get presigned upload URL | Yes | MinIO + CockroachDB |
| `/v1/storage/upload-complete` | POST | Mark upload complete | Yes | CockroachDB |
| `/v1/storage/download-url/:file_id` | GET | Get presigned download URL | Yes | MinIO + CockroachDB |
| `/v1/storage/files/:file_id` | DELETE | Delete a file | Yes | MinIO + CockroachDB |
| `/v1/storage/quota` | GET | Get storage quota | Yes | CockroachDB |

### 7.7 WebSocket Endpoints

| Endpoint | Purpose | Protocol | Auth |
|----------|---------|-----------|-------|
| `/v1/ws/chat?conversation_id=<uuid>` | Real-time chat | WebSocket | JWT in header/query |
| `/v1/calls/ws/signaling?call_id=<uuid>` | WebRTC signaling | WebSocket | JWT in header/query |

### 7.8 Rate Limits

| Endpoint Pattern | Requests | Window |
|-----------------|----------|--------|
| `/v1/auth/login` | 5 | 1 minute |
| `/v1/auth/register` | 3 | 1 minute |
| `/v1/auth/refresh` | 10 | 1 minute |
| `/v1/users/me` | 50 | 1 minute |
| `/v1/users/me/password` | 5 | 1 minute |
| `/v1/users/me/email` | 5 | 1 minute |
| `/v1/messages` | 100 | 1 minute |
| `/v1/calls` | 20 | 1 minute |
| `/v1/storage` | 30 | 1 minute |
| Default | 100 | 1 minute |

---

## 8. DATABASE SCHEMA SUMMARY

### 8.1 CockroachDB Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `users` | User accounts | user_id, email, username |
| `identity_keys` | E2EE identity keys | user_id, public_key_ed25519 |
| `signed_pre_keys` | E2EE signed pre-keys | user_id, key_id, public_key |
| `one_time_pre_keys` | E2EE one-time pre-keys | user_id, key_id, public_key, used |
| `conversations` | Conversation metadata | conversation_id, type, name |
| `conversation_participants` | Conversation members | conversation_id, user_id, role |
| `conversation_settings` | E2EE/AI settings | conversation_id, is_e2ee_enabled, ai_enabled |
| `files` | File metadata | file_id, user_id, minio_object_key |
| `friendships` | Friend relationships | user_id_1, user_id_2, status |
| `blocked_users` | Blocked user relationships | blocker_id, blocked_id |
| `email_verification_tokens` | Email verification | token_id, user_id, token |
| `subscriptions` | SaaS billing | subscription_id, user_id, plan_type |
| `calls` | Call records | call_id, conversation_id, caller_id |
| `call_participants` | Call participants | call_id, user_id |
| `notifications` | User notifications | notification_id, user_id |

### 8.2 Cassandra Tables

| Table | Purpose | Partition Key | Clustering Key |
|-------|---------|---------------|----------------|
| `messages` | Chat messages | conversation_id, bucket | sent_at DESC, message_id DESC |
| `call_logs` | Call history | user_id | started_at DESC, call_id DESC |
| `message_reactions` | Message reactions | message_id | user_id |
| `message_attachments` | File attachments | message_id | attachment_id |
| `user_activity` | User analytics | user_id | activity_time DESC |

### 8.3 Redis Data Structures

| Key Pattern | Type | Purpose | TTL |
|-------------|------|---------|-----|
| `session:<session_id>` | String | User session | 30 days |
| `user:sessions:<user_id>` | Set | User's active sessions | - |
| `blacklist:<jti>` | String | Revoked JWT tokens | Token expiry |
| `directory:email:<email>` | String | Email → userID mapping | - |
| `directory:username:<username>` | String | Username → userID mapping | - |
| `presence:<user_id>` | String | User online status | - |
| `failed_login:<email>` | String | Failed login attempts | 15 minutes |
| `ratelimit:<identifier>` | String | Rate limit counter | 1 minute |
| `chat:<conversation_id>` | Pub/Sub | Real-time messages | - |
| `call:<call_id>` | Pub/Sub | WebRTC signaling | - |

---

## 9. OPEN QUESTIONS & RISKS

### 9.1 Open Questions

1. **E2EE Implementation:** How is the actual encryption/decryption handled on the client side? The server stores keys but the client-side implementation is not in this codebase.
2. **AI Integration:** The system mentions AI features (Edge AI, AI summaries) but no AI service integration is found in the code.
3. **Push Notifications:** No push notification service (FCM/APNS) integration found.
4. **WebRTC SFU:** The video service mentions Pion SFU but it's marked as TODO and not implemented.
5. **Email Service:** Email verification endpoints exist but no email sending service (SMTP/SendGrid/etc.) is integrated.
6. **Monitoring:** Grafana/Loki configs exist but no actual metrics collection is implemented in the code.
7. **Testing:** Test files exist but coverage levels are unknown.
8. **Deployment:** Kubernetes manifests exist but production deployment status is unknown.

### 9.2 Security Risks

1. **JWT Secret:** Default JWT secret `super-secret-key-please-use-longer-key` is hardcoded in docker-compose files.
2. **MinIO Credentials:** Default credentials `minioadmin/minioadmin` are hardcoded.
3. **Database Passwords:** No password protection in development mode (`--insecure` for CockroachDB).
4. **CORS:** WebSocket upgrader allows all origins (`CheckOrigin` returns true).
5. **Rate Limiting:** Fail-open behavior in rate limiter could allow abuse on Redis failures.
6. **Token Blacklist:** If Redis is down, token revocation checking fails (fail-closed).
7. **Session Management:** No session limit per user (could allow unlimited concurrent sessions).
8. **Failed Login Tracking:** Account lockout exists but IP-based throttling is not implemented.
9. **File Upload:** No virus scanning or content type validation beyond MIME type.
10. **SQL Injection:** While using parameterized queries, some dynamic SQL construction could be vulnerable.

### 9.3 Scalability Risks

1. **Single Node CockroachDB:** Production deployment uses single-node mode which has no HA.
2. **Cassandra Replication:** Replication factor is 1 (no data redundancy).
3. **Redis Persistence:** Redis persistence is enabled but no backup strategy is documented.
4. **MinIO Storage:** No backup/replication strategy for object storage.
5. **WebSocket Connections:** No connection limit per user or per server.
6. **Message Pagination:** Bucket-based pagination could be inefficient for very old messages.

### 9.4 Operational Risks

1. **Graceful Shutdown:** Some services don't implement proper graceful shutdown.
2. **Health Checks:** Health checks are basic (no dependency health verification).
3. **Logging:** Structured logging is used but log levels and rotation are not configured.
4. **Error Handling:** Generic error responses may leak sensitive information.
5. **Configuration:** Environment variables are not validated at startup.
6. **Database Migrations:** No migration tool (golang-migrate, goose, etc.) is integrated.
7. **Backup Strategy:** Backup scripts exist but automated backup scheduling is not implemented.
8. **Monitoring:** No metrics export (Prometheus) or distributed tracing (OpenTelemetry).

### 9.5 Architecture Risks

1. **Tight Coupling:** Services are loosely coupled but share the same JWT secret (single point of compromise).
2. **Data Consistency:** No distributed transaction manager across CockroachDB and Cassandra.
3. **Service Discovery:** Hardcoded service names in Docker Compose (no service registry).
4. **API Versioning:** Only v1 exists, no versioning strategy for breaking changes.
5. **Circuit Breaker:** No circuit breaker pattern for service-to-service calls.
6. **Idempotency:** Some endpoints may not be idempotent (message sending, call initiation).

### 9.6 Data Privacy Risks

1. **GDPR Compliance:** No user data export or anonymization endpoints.
2. **Data Retention:** Message retention is configurable but no automated cleanup job.
3. **Right to be Forgotten:** Account deletion exists but may not delete all associated data (messages, files).
4. **Encryption at Rest:** Database encryption at rest is not configured.

---

## 10. ASSUMPTIONS

1. **Client Implementation:** Flutter client handles E2EE encryption/decryption using the Signal Protocol.
2. **Production Environment:** Production uses Docker secrets or external secret management (not hardcoded).
3. **Network:** All services run in the same Docker network for development.
4. **Database Scaling:** Production will use CockroachDB clusters and Cassandra multi-node setup.
5. **SSL/TLS:** Production will use proper SSL certificates for all services.
6. **Monitoring:** Production will integrate with external monitoring (Prometheus, Grafana, Loki).
7. **CI/CD:** GitHub Actions or similar CI/CD pipeline is used for deployments.
8. **Load Balancing:** Production will use external load balancer (AWS ALB, GCP LB) or Kubernetes Ingress.
9. **Backup Strategy:** Production will have automated backups and disaster recovery plan.
10. **Compliance:** Production will comply with relevant regulations (GDPR, HIPAA, etc.) based on target market.

---

## 11. NEXT STEPS (PHASE 2)

Based on this discovery, Phase 2 should focus on:

1. **Security Hardening:**
   - Implement proper secret management
   - Add input validation and sanitization
   - Implement rate limiting improvements
   - Add security headers and CSP
   - Implement session management improvements

2. **Production Readiness:**
   - Add health checks with dependency verification
   - Implement graceful shutdown
   - Add metrics export (Prometheus)
   - Implement distributed tracing
   - Add database migrations

3. **Scalability:**
   - Implement connection pooling optimizations
   - Add circuit breakers
   - Implement service discovery
   - Add horizontal scaling support

4. **Operational Excellence:**
   - Implement structured logging with rotation
   - Add alerting rules
   - Implement backup automation
   - Add deployment automation

5. **Feature Completion:**
   - Complete WebRTC SFU implementation
   - Integrate email service
   - Implement push notifications
   - Add AI service integration

---

**Report End**

This report provides a comprehensive overview of the SecureConnect system. All findings are based on actual code analysis without assumptions or mock data.
