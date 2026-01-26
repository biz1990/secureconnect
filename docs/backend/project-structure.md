# Backend Project Structure (Go)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 9.1. Tổng quan

Hệ thống backend được xây dựng theo mô hình **Monorepo** (Kho lưu trữ thống nhất). Điều này giúp quản lý các Microservices (Auth, Chat, Video, Gateway) trong một chỗ, đồng thời chia sẻ các thư viện mã nguồn chung (như models, utils, crypto) một cách hiệu quả.

Cấu trúc dựa trên tiêu chuẩn của cộng đồng Go, chia tách rõ ràng giữa **Command** (điểm vào) và **Internal** (logic nghiệp vụ).

---

## 9.2. Cây thư mục chi tiết (Directory Tree)

```bash
secureconnect-backend/
├── cmd/                        # Điểm vào của ứng dụng (Entry Points)
│   ├── api-gateway/            # Gateway Service
│   │   └── main.go
│   ├── auth-service/           # Authentication Service
│   │   └── main.go
│   ├── chat-service/           # Messaging Service
│   │   └── main.go
│   ├── video-service/          # WebRTC/Signaling Service
│   │   └── main.go
│   └── storage-service/        # File Upload/MinIO Service
│       └── main.go
│
├── internal/                   # Mã nguồn riêng tư, không thể import từ bên ngoài
│   ├── auth/                   # Logic cho Auth Service
│   │   ├── handler.go          # HTTP Handlers (Controllers)
│   │   ├── service.go          # Business Logic
│   │   └── repository.go       # Database Access Layer
│   │
│   ├── chat/                   # Logic cho Chat Service
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── websocket.go        # WebSocket logic riêng
│   │
│   ├── video/                  # Logic cho Video Service (Pion)
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── sfu.go              # Selective Forwarding Unit Logic
│   │   └── signaling.go        # WebSocket Signaling logic
│   │
│   ├── middleware/             # Middleware dùng chung (Giống Django/Express middleware)
│   │   ├── auth.go             # JWT Verification
│   │   ├── logging.go          # HTTP Logging
│   │   ├── cors.go
│   │   └── rate_limit.go
│   │
│   ├── database/               # Các gói kết nối và khởi tạo DB dùng chung
│   │   ├── cockroachdb.go      # SQL connection pool
│   │   ├── cassandra.go        # NoSQL connection
│   │   └── redis.go            # Cache connection
│   │
│   ├── models/                 # Data Models (Structs) dùng chung
│   │   ├── user.go
│   │   ├── message.go
│   │   └── conversation.go
│   │
│   ├── crypto/                 # Xử lý mã hóa (E2EE Libraries wrappers)
│   │   ├── keys.go             # Xử lý Public Keys
│   │   └── utils.go             # Helper functions
│   │
│   └── config/                 # Quản lý cấu hình app
│       └── config.go
│
├── pkg/                        # Mã nguồn công khai (có thể dùng bên ngoài, nhưng chủ yếu nội bộ)
│   ├── logger/                 # Gói ghi log tùy chỉnh (đóng gói lại Zap)
│   │   └── logger.go
│   └── errors/                 # Custom Error types
│       └── errors.go
│
├── api/                        # Định nghĩa API / Protobuf
│   ├── swagger/                # OpenAPI spec (đã tạo ở file khác)
│   │   └── api-openapi-spec.yaml
│   └── protobuf/               # Nếu dùng gRPC (tương lai)
│       └── user.proto
│
├── configs/                    # File cấu hình môi trường
│   ├── config.dev.yaml
│   ├── config.staging.yaml
│   └── config.prod.yaml
│
├── deployments/                # K8s và Docker configs
│   ├── docker/
│   │   ├── Dockerfile.chat-service
│   │   ├── Dockerfile.video-service
│   │   └── Dockerfile.gateway
│   └── k8s/
│       ├── chat-service.yaml
│       └── video-service.yaml
│
├── scripts/                    # Các script tiện ích
│   ├── build.sh                # Build tất cả binaries
│   ├── run-local.sh            # Chạy toàn bộ services (docker-compose)
│   └── migrate.sh              # Chạy DB migrations
│
├── test/                       # Integration / E2E tests
│   └── integration/
│       └── chat_test.go
│
├── go.mod                      # Go Modules definition
├── go.sum                      # Dependency locks
├── Makefile                    # Build automation
└── README.md
```

---

## 9.3. Giải thích chi tiết các thành phần

### 9.3.1. `/cmd` (Application Entry Points)
Mỗi thư mục trong này là một file thực thi (binary) riêng biệt. Mỗi service (Chat, Video, Auth) sẽ là một process riêng chạy trong Docker container.

*   **Nguyên tắc:** Giữ `main.go` thật đơn giản. Chỉ khởi tạo config, kết nối DB, khởi tạo routes và start server.
*   **Ví dụ `cmd/chat-service/main.go`:**
    ```go
    func main() {
        // 1. Load Config
        cfg := config.Load()
        
        // 2. Init DB (Cassandra, Redis)
        db := database.NewCassandra(cfg.Cassandra)
        
        // 3. Init Layers
        repo := chat.NewRepository(db)
        svc := chat.NewService(repo)
        handler := chat.NewHandler(svc)
        
        // 4. Start Server
        r := gin.Default()
        r.Use(middleware.Auth())
        handler.SetupRoutes(r)
        r.Run(":8080")
    }
    ```

### 9.3.2. `/internal` (Private Application Code)
Đây là nơi "tâm hồn" của ứng dụng nằm. Code ở đây **không thể** được import bởi các dự án Go bên ngoài (Go compiler cấm điều này).

#### Quy tắc tổ chức theo Service (Clean Architecture)
Mỗi service (ví dụ `internal/chat`) nên được chia thành 3 lớp chuẩn:

1.  **Handler Layer (Controller):**
    *   *File:* `handler.go`
    *   *Trách nhiệm:* Nhận HTTP Request, parse JSON, validate input, gọi Service, trả về HTTP Response.
    *   *Thư viện:* Ngoại lệ có thể dùng `Gin` (HTTP framework).

2.  **Service Layer (Business Logic):**
    *   *File:* `service.go`
    *   *Trách nhiệm:* Xử lý logic nghiệp vụ. Ví dụ: Kiểm tra `is_encrypted` flag -> Có gọi AI hay không -> Gọi Repository lưu vào DB.
    *   *Quy tắc:* Không được biết đến HTTP (Request/Response). Chỉ xử lý Data Struct.

3.  **Repository Layer (Data Access):**
    *   *File:* `repository.go`
    *   *Trách nhiệm:* Giao tiếp trực tiếp với Database (SQL/NoSQL).
    *   *Quy tắc:* Chỉ trả về Data Struct, không biết logic nghiệp vụ.

### 9.3.3. `/internal/middleware`
Chứa các interceptor dùng chung cho tất cả các services (hoặc ít nhất là Gateway).

*   **`auth.go`:** Kiểm tra JWT trong header, inject `user_id` vào context.
*   **`rate_limit.go`:** Kiểm tra Redis xem user có spam không.

### 9.3.4. `/internal/crypto`
Gói này bao quanh thư viện `libsodium` hoặc `go-sodium` để tạo interface dễ dùng cho toàn bộ hệ thống. Các service (Chat, Video) chỉ cần gọi `crypto.Encrypt()` mà không cần lo chi tiết thuật toán.

### 9.3.5. `/pkg`
Code trong thư mục này **có thể** được import từ bên ngoài (nhưng trong dự án này chủ yếu là nội bộ).
*   Dùng để chứa các thư viện utilities mà độc lập với nghiệp vụ (ví dụ: wrapper cho logger, helper để format UUID, v.v.).

---

## 9.4. Configuration Management (Quản lý cấu hình)

Chúng ta sử dụng thư viện **Viper** để quản lý cấu hình từ file YAML hoặc Environment Variables.

**File cấu hình mẫu (`configs/config.dev.yaml`):**
```yaml
server:
  port: 8080
  mode: debug # release

database:
  cockroachdb:
    host: "localhost"
    port: 26257
    user: "root"
    dbname: "secureconnect"
  cassandra:
    hosts: ["localhost:9042"]
    keyspace: "secureconnect_ks"
  redis:
    host: "localhost:6379"

jwt:
  secret: "super-secret-key" # Trong production dùng ENV VAR
  expiry: 15m

webrtc:
  stun_servers:
    - "stun:stun.l.google.com:19302"
  turn_servers:
    - url: "turn:localhost:3478"
      username: "user"
      credential: "pass"
```

---

## 9.5. Build & Dependency Management

### 9.5.1. Go Modules (`go.mod`)
File gốc ở thư mục root: `go.mod`.
*   Tất cả các dependencies (Gin, Gocql, Pion, Viper) đều khai báo ở đây.
*   Không tạo `go.mod` riêng cho từng service.

### 9.5.2. Makefile
Để đơn giản hóa việc build và chạy, sử dụng `Makefile` ở thư mục root.

**Nội dung `Makefile`:**
```makefile
# Biến môi trường
APP_NAME=secureconnect
GO=go

# Build các binaries cho từng service
build-chat:
    @echo "Building Chat Service..."
    $(GO) build -o bin/chat-service ./cmd/chat-service

build-video:
    @echo "Building Video Service..."
    $(GO) build -o bin/video-service ./cmd/video-service

build-gateway:
    @echo "Building API Gateway..."
    $(GO) build -o bin/api-gateway ./cmd/api-gateway

build-all: build-chat build-video build-gateway

# Chạy cục bộ (Sử dụng docker-compose nếu cần phụ thuộc DB)
run-chat:
    $(GO) run ./cmd/chat-service

run-video:
    $(GO) run ./cmd/video-service

# Test
test:
    $(GO) test -v ./...

# Clean
clean:
    rm -rf bin/
```

---

## 9.6. Logging Strategy

Thay vì `fmt.Println`, sử dụng thư viện structured logging (như **Zap** hoặc **Logrus**), được gói gọn trong `pkg/logger`.

*   **Development:** Log ra dạng Console màu sắc, dễ đọc.
*   **Production:** Log ra dạng JSON để gửi vào ELK Stack (Elasticsearch, Logstash, Kibana) hoặc Loki.

**Ví dụ code trong Handler:**
```go
logger := app.GetLogger()
logger.Info("User sent a message",
    zap.String("user_id", userID),
    zap.String("conversation_id", convID),
    zap.Bool("is_encrypted", msg.IsEncrypted),
)
```

---

## 9.7. Docker Integration

Mỗi service sẽ có file `Dockerfile` riêng trong `deployments/docker/`.

**Dockerfile mẫu (`deployments/docker/Dockerfile.chat-service`):**
```dockerfile
# Multi-stage build để giảm image size
# Stage 1: Build
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/chat-service

# Stage 2: Run
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]
```

---

*Liên kết đến tài liệu tiếp theo:* `backend/e2ee-implementation-guide.md`
