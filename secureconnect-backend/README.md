# SecureConnect Backend

Backend microservices cho SecureConnect SaaS Platform - Hệ thống liên lạc bảo mật với E2EE và AI tích hợp.

## 🏗 Kiến trúc (Architecture)

Hệ thống được xây dựng theo **Clean Architecture** với **Microservices pattern**:

- **API Gateway** (Port 8080): Entry point, routing, authentication middleware
- **Auth Service** (Port 8081): User registration, login, JWT management
- **Chat Service** (Port 8082): Real-time messaging, WebSocket gateway
- **Video Service** (Port 8083): WebRTC signaling, video call management
- **Storage Service** (Port 8084): File upload/download với MinIO

## 📂 Cấu trúc thư mục (Project Structure)

```
secureconnect-backend/
├── cmd/                          # Entry points cho các services
│   ├── api-gateway/             # API Gateway (port 8080)
│   │   ├── main.go
│   │   └── Dockerfile
│   ├── auth-service/            # Auth Service (port 8081)
│   │   ├── main.go
│   │   └── Dockerfile
│   ├── chat-service/            # Chat Service (port 8082)
│   │   ├── main.go
│   │   └── Dockerfile
│   ├── video-service/           # Video Service (port 8083)
│   │   ├── main.go
│   │   └── Dockerfile
│   └── storage-service/         # Storage Service (port 8084)
│       ├── main.go
│       └── Dockerfile
├── internal/                     # Private application code
│   ├── domain/                  # Domain models (entities)
│   │   ├── user.go              # User entity
│   │   ├── message.go           # Message entity (Hybrid E2EE)
│   │   ├── conversation.go      # Conversation metadata
│   │   ├── keys.go              # E2EE keys (Signal Protocol)
│   │   └── file.go              # File metadata
│   ├── repository/              # Data access layer
│   │   ├── cockroach/           # CockroachDB repositories
│   │   ├── cassandra/           # Cassandra repositories
│   │   └── redis/               # Redis repositories
│   ├── service/                 # Business logic
│   │   ├── auth/
│   │   ├── chat/
│   │   ├── video/
│   │   ├── storage/
│   │   └── crypto/              # E2EE implementation
│   ├── handler/                 # HTTP/WebSocket handlers
│   │   ├── http/
│   │   └── ws/
│   └── middleware/              # Middleware (auth, rate limit, etc.)
├── pkg/                         # Shared packages
│   ├── config/
│   ├── database/
│   ├── logger/
│   ├── jwt/
│   ├── response/
│   └── storage/
├── scripts/                     # Database init scripts
├── test/integration/            # Integration tests
├── go.mod
├── go.sum
└── docker-compose.yml
```

## 🛠 Tech Stack

- **Language**: Go 1.21+
- **Framework**: Gin (HTTP), Gorilla WebSocket
- **Databases**:
  - CockroachDB: Users, billing, keys metadata
  - Cassandra: Messages, call logs (time-series)
  - Redis: Cache, sessions, pub/sub
- **Storage**: MinIO (S3-compatible)
- **Crypto**: golang.org/x/crypto (Signal Protocol)

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Make (optional)

### Development

1. **Clone repository**
```bash
cd d:\secureconnect\secureconnect-backend
```

2. **Install dependencies**
```bash
go mod download
```

3. **Start databases** (using Docker Compose)
```bash
docker-compose up -d cockroachdb cassandra redis minio
```

4. **Run services locally**

Each service can be run individually:
```bash
# API Gateway
cd cmd/api-gateway && go run main.go

# Auth Service
cd cmd/auth-service && go run main.go

# Chat Service
cd cmd/chat-service && go run main.go

# Video Service
cd cmd/video-service && go run main.go

# Storage Service
cd cmd/storage-service && go run main.go
```

### Docker Build

Build all services:
```bash
# Build API Gateway
docker build -f cmd/api-gateway/Dockerfile -t secureconnect-api-gateway .

# Build Auth Service
docker build -f cmd/auth-service/Dockerfile -t secureconnect-auth-service .

# Build Chat Service
docker build -f cmd/chat-service/Dockerfile -t secureconnect-chat-service .

# Build Video Service
docker build -f cmd/video-service/Dockerfile -t secureconnect-video-service .

# Build Storage Service
docker build -f cmd/storage-service/Dockerfile -t secureconnect-storage-service .
```

## 📋 Domain Models (Phase 1 Completed)

✅ **User**: User entity với authentication data
✅ **Message**: Hybrid E2EE message model với `is_encrypted` flag
✅ **Conversation**: Conversation metadata với E2EE settings
✅ **Keys**: Signal Protocol keys (Identity, PreKeys, OneTimeKeys)
✅ **File**: File metadata cho Storage Service

## 🔐 Security Features (Planned)

- **E2EE**: Signal Protocol implementation (X3DH + Double Ratchet)
- **Hybrid Security**: Opt-out encryption per conversation
- **JWT Authentication**: Access & Refresh token pattern
- **Client-side File Encryption**: Zero-knowledge storage option

## 📝 API Documentation

API spec được định nghĩa theo `docs/05-api-design.md`.

### Health Check Endpoints

- `GET /health` - API Gateway
- `GET /health` - Auth Service (port 8081)
- `GET /health` - Chat Service (port 8082)
- `GET /health` - Video Service (port 8083)
- `GET /health` - Storage Service (port 8084)

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Integration tests
go test ./test/integration/...
```

## 📖 Documentation

Xem thêm tài liệu chi tiết trong thư mục `docs/`:
- `01-system-overview.md`: Tổng quan hệ thống
- `03-security-architecture.md`: Kiến trúc bảo mật & E2EE
- `05-api-design.md`: API design standards
- `07-database-schema.md`: Database schemas
- `08-data-models-go-vs-dart.md`: Data models mapping

## 📜 License

Private License - For educational and internal use only.

## 👤 Author

System Architect - SecureConnect Team
