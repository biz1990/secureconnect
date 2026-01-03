# --- STAGE 1: BUILD ---
# Sử dụng image Go chuẩn để biên dịch
FROM golang:1.21-alpine AS builder

# Cài đặt các công cụ cần thiết (git, cacerts)
RUN apk add --no-cache git ca-certificates tzdata

# Thiết lập thư mục làm việc
WORKDIR /app

# Copy go.mod và go.sum trước để tận dụng Docker Cache (rất quan trọng để build nhanh lần sau)
COPY go.mod go.sum ./

# Download các thư viện dependencies
RUN go mod download

# Copy toàn bộ source code vào container
# (Sẽ copy các thư mục cmd/, internal/, pkg/, ...)
COPY . .

# --- BUILD BINARY ---
# Chúng ta sử dụng ARG để biết đang build service nào (do docker-compose truyền vào)
ARG SERVICE_NAME=""
ARG CMD=""

# Biên dịch code Go thành file binary tĩnh (static)
# Binary được đặt tên là $SERVICE_NAME (ví dụ: api-gateway)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/${SERVICE_NAME} /app/${CMD}

# --- STAGE 2: RUN ---
# Sử dụng image Alpine nhẹ nhất để chạy ứng dụng (image chạy thực tế)
FROM alpine:latest

# Cài đặt thư viện liên kết (thiếu thì Go code bị lỗi)
RUN apk --no-cache add ca-certificates tzdata curl

# Tạo user không root để tăng bảo mật
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy file binary từ Stage Builder sang Stage Runner
# Binary này nằm ở /app/${SERVICE_NAME}
COPY --from=builder /app/${SERVICE_NAME} /app/service

# Copy file config (nếu cần)
COPY configs /app/configs

# Gán quyền sở hữu cho appuser
RUN chown -R appuser:appgroup /app

# Chuyển sang user appuser (không chạy bằng root)
USER appuser

# Expose port 8080
EXPOSE 8080

# Lệnh chạy mặc định là chạy file binary tên là 'service'
CMD ["./service"]