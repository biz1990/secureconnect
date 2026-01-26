# Docker Setup & Deployment Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 18.1. Tổng quan

Docker được sử dụng để đảm bảo tính "Build once, run anywhere" (Xây dựng một lần, chạy mọi nơi).

### Chiến lược Container
*   **Backend (Go):** Sử dụng **Multi-stage Builds**. Giai đoạn 1: Biên dịch code với môi trường Go. Giai đoạn 2: Copy kết quả ra image Alpine nhẹ nhất.
*   **Frontend (Flutter Web):** Sử dụng **Multi-stage Builds**. Giai đoạn 1: Biên dịch Flutter Web. Giai đoạn 2: Serve bằng Nginx.

---

## 18.2. Cấu trúc Dockerfiles

Mỗi service backend sẽ có `Dockerfile` riêng hoặc dùng một Dockerfile chung với `ARG` chỉ định binary. Chúng tôi chọn cách **một Dockerfile chung** trong thư mục root để dễ quản lý.

**Cấu trúc thư mục:**
```bash
secureconnect/
├── Dockerfile.backend          # Dùng chung cho tất cả Go Services
├── Dockerfile.frontend         # Dùng cho Flutter Web (nếu deploy web)
├── docker-compose.yml           # Cho môi trường Development/Staging
├── docker-compose.prod.yml      # Cho môi trường Production (tùy chọn)
└── .env                        # Environment variables
```

---

## 18.3. Backend Dockerfile (Go Multi-stage)

File này nằm ở thư mục root. Nó có khả năng build bất kỳ service nào (Gateway, Chat, Video) dựa trên `SERVICE_NAME` argument.

**File: `Dockerfile.backend`**

```dockerfile
# --- STAGE 1: BUILD ---
# Dùng image Go chuẩn để biên dịch
FROM golang:1.21-alpine AS builder

# Cài đặt các công cụ build cần thiết
RUN apk add --no-cache git ca-certificates tzdata

# Thiết lập thư mục làm việc
WORKDIR /app

# Copy go.mod và go.sum để tận dụng Docker Cache Layer (Quan trọng cho tốc độ build)
COPY go.mod go.sum ./
RUN go mod download

# Copy toàn bộ source code
COPY . .

# Biên dịch Binary.
# CGO_ENABLED=0: Biên dịch thành static binary (không phụ thuộc C runtime)
# GOOS=linux: Chỉ chạy được trên Linux
ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/${SERVICE_NAME} ./cmd/${SERVICE_NAME}

# --- STAGE 2: RUN ---
# Dùng image Alpine siêu nhẹ để chạy ứng dụng
FROM alpine:latest

# Cài đặt ca-certificates (Để Go call HTTPS API không bị lỗi cert)
RUN apk --no-cache add ca-certificates tzdata curl

# Tạo user không root để bảo mật
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary từ Stage 1
ARG SERVICE_NAME
COPY --from=builder /app/bin/${SERVICE_NAME} /app/${SERVICE_NAME}

# Copy file config (Nếu có)
COPY configs/config.prod.yaml /app/config.yaml

# Gán quyền sở hữu
RUN chown -R appuser:appgroup /app

# Switch sang user không root
USER appuser

# Environment Variables
ENV TZ="Asia/Ho_Chi_Minh"

# Port expose (Mặc định)
EXPOSE 8080

# Healthcheck (Cần endpoint GET /healthz ở Go code)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

# Lệnh chạy
CMD ["/app/${SERVICE_NAME}"]
```

---

## 18.4. Frontend Dockerfile (Flutter Web)

Dùng để deploy Web version của app lên server (Nginx).

**File: `Dockerfile.frontend`**

```dockerfile
# --- STAGE 1: BUILD FLUTTER ---
# Dùng image Flutter ổn định
FROM cirrusci/flutter:3.16.0-stable AS builder

WORKDIR /app

# Copy source code
COPY . .

# Phải chỉ định platform web
RUN flutter config --no-analytics
RUN flutter pub get
RUN flutter build web --release

# --- STAGE 2: SERVE NGINX ---
FROM nginx:alpine

# Copy kết quả build web vào thư mục nginx
COPY --from=builder /app/build /usr/share/nginx/html

# Copy config nginx tùy chỉnh (ví dụ: cache control, gzip)
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

**File `nginx.conf` (Nằm cùng thư mục):**
```nginx
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/css application/javascript image/svg+xml;

    # SPA Routing (Tất cả request trỏ về index.html)
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

---

## 18.5. Docker Compose (Development/Local)

Dùng cho team lập trình chạy full hệ thống trên máy cá nhân (Laptop).

**File: `docker-compose.yml`**

```yaml
version: '3.8'

services:
  # --- 1. Databases (Dependencies) ---
  
  cockroachdb:
    image: cockroachdb/cockroach:v23.1.0
    command: start-single-node --insecure
    ports:
      - "26257:26257"
      - "8081:8080" # UI Admin
    volumes:
      - crdb_data:/cockroach/cockroach-data
    environment:
      - POSTGRES_USER=root
      - POSTGRES_DB=secureconnect

  cassandra:
    image: cassandra:latest
    ports:
      - "9042:9042"
    volumes:
      - cassandra_data:/var/lib/cassandra
    environment:
      - CASSANDRA_CLUSTER_NAME=DevCluster
      - CASSANDRA_DC=datacenter1

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - minio_data:/data

  # --- 2. Backend Services ---

  api-gateway:
    build:
      context: .
      dockerfile: Dockerfile.backend
      args:
        SERVICE_NAME: api-gateway
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=cockroachdb
      - REDIS_HOST=redis
      - JWT_SECRET=dev_secret
      - ENV=development
    depends_on:
      - cockroachdb
      - redis

  auth-service:
    build:
      context: .
      dockerfile: Dockerfile.backend
      args:
        SERVICE_NAME: auth-service
    environment:
      - DB_HOST=cockroachdb
      - REDIS_HOST=redis
    depends_on:
      - cockroachdb

  chat-service:
    build:
      context: .
      dockerfile: Dockerfile.backend
      args:
        SERVICE_NAME: chat-service
    environment:
      - CASSANDRA_HOST=cassandra
      - REDIS_HOST=redis
    depends_on:
      - cassandra

  video-service:
    build:
      context: .
      dockerfile: Dockerfile.backend
      args:
        SERVICE_NAME: video-service
    environment:
      - REDIS_HOST=redis
    depends_on:
      - redis

  # --- 3. Frontend (Optional) ---
  web-app:
    build:
      context: .
      dockerfile: Dockerfile.frontend
    ports:
      - "3000:80"
    depends_on:
      - api-gateway

volumes:
  crdb_data:
  cassandra_data:
  redis_data:
  minio_data:
```

**Cách chạy:**
```bash
# Build và khởi tạo tất cả containers
docker-compose up -d --build

# Xem logs
docker-compose logs -f chat-service

# Dừng tất cả
docker-compose down
```

---

## 18.6. Docker Compose (Production / K8s Pre-check)

Trong môi trường Production, chúng ta không dùng `docker-compose` trực tiếp để run, mà dùng nó để **image build** hoặc test.

**File: `docker-compose.prod.yml`**

```yaml
version: '3.8'

# Không có service database (sẽ dùng K8s cluster hoặc Managed Cloud DB như AWS RDS)
services:
  api-gateway:
    build:
      context: .
      dockerfile: Dockerfile.backend
      args:
        SERVICE_NAME: api-gateway
    image: secureconnect/api-gateway:latest # Tag version rõ ràng
    environment:
      - ENV=production
```

**Lệnh đẩy image lên Registry:**
```bash
# Login vào Docker Hub hoặc AWS ECR
docker login

# Build image
docker-compose -f docker-compose.prod.yml build

# Tag version (ví dụ: v1.0.0)
docker tag secureconnect/api-gateway:latest secureconnect/api-gateway:v1.0.0

# Push lên registry
docker push secureconnect/api-gateway:latest
docker push secureconnect/api-gateway:v1.0.0
```

---

## 18.7. Best Practices cho Dockerfile

1.  **Leverage Layer Caching (Tận dụng Cache):**
    *   Trong Dockerfile, luôn copy `go.mod` và `go.sum` trước khi copy toàn bộ thư mục `.`
    *   Khi bạn sửa code (`main.go`), Docker sẽ build lại từ bước copy code, không cần tải lại dependencies nữa (nếu không đổi `go.mod`). Điều này giúp build cực nhanh.

2.  **Run as Non-Root User:**
    *   Mặc định container chạy bằng `root`. Nếu hacker xâm nhập, họ có quyền root.
    *   Hãy luôn tạo user `appuser` và chạy app với user đó (như ví dụ Dockerfile trên).

3.  **Healthchecks:**
    *   Luôn định nghĩa `HEALTHCHECK` trong Dockerfile.
    *   Trong code Go, tạo một endpoint đơn giản: `GET /healthz` trả về `200 OK`. Docker (hoặc Kubernetes) sẽ dùng endpoint này để biết container có "sống" không để restart lại nếu nó "chết".

4.  **Minimize Image Size:**
    *   Dùng `alpine:latest` thay vì `ubuntu` hoặc `debian` (Dung lượng chỉ khoảng 5MB so với 80MB).
    *   Xóa các file không cần thiết trong build stage.

---

## 18.8. Lưu ý cho Video Service (WebRTC)

Vì Video Service sử dụng thư viện `Pion WebRTC` (thuần Go), nó hoạt động rất tốt trong container Alpine.
*   **Port UDP:** WebRTC sử dụng cả TCP và UDP. Khi expose port trên Docker (`ports: ["3478:3478/udp"]`), hãy đảm bảo map cả protocol UDP nếu có dùng TURN server.
*   **Performance:** Nếu thấy hình ảnh bị giật (low framerate), hãy tăng CPU limit cho container (trong K8s `requests.cpu` hoặc Docker Compose `cpus: '2.0'`).

---

## 18.9. Clean-up (Dọn dẹp)

Để tránh ổ cứng đầy rác Docker image:

```bash
# Xóa các dangling images (không được dùng)
docker image prune -f

# Xóa tất cả volume không sử dụng
docker volume prune -f

# Xóa tất cả system (thận trọng)
docker system prune -a
```

---

*Liên kết đến tài liệu tiếp theo:* `devops/kubernetes-manifests.md`