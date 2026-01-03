.PHONY: build run clean

# Biến môi trường
GO=go
GOFLAGS=-v
DOCKER_COMPOSE=docker-compose

# Mục tiêu: Build tất cả services
build:
    @echo "Running build script..."
    @chmod +x scripts/build.sh
    ./scripts/build.sh

# Mục tiêu: Chạy Auth service (Docker Compose)
run-auth:
    $(DOCKER_COMPOSE) up -d auth-service

# Mục tiêu: Xem logs Auth service
logs-auth:
    $(DOCKER_COMPOSE) logs -f auth-service

# Mục tiêu: Chạy toàn bộ hệ thống
run-all:
    $(DOCKER_COMPOSE) up -d

# Mục tiêu: Dừng toàn bộ hệ thống
stop:
    $(DOCKER_COMPOSE) down

# Mục tiêu: Clean binaries
clean:
    rm -f bin/*