.PHONY: help build run clean test lint docker-build docker-up docker-down init-db

# Variables
GO=go
GOFLAGS=-v
DOCKER_COMPOSE=docker-compose
GOLANGCI_LINT=golangci-lint
SERVICES=api-gateway auth-service chat-service video-service storage-service

# Default target
.DEFAULT_GOAL := help

## help: Display this help message
help:
	@echo "SecureConnect Backend - Makefile Commands"
	@echo "=========================================="
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/^## ([^:]+): (.+)/  \1|~\2/' | column -t -s '|~'

## deps: Download Go dependencies
deps:
	@echo "📦 Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

## build: Build all microservices
build:
	@echo "🔨 Building all services..."
	@for service in $(SERVICES); do \\
		echo "Building $$service..."; \\
		$(GO) build $(GOFLAGS) -o bin/$$service ./cmd/$$service; \\
	done
	@echo "✅ Build complete"

## build-%: Build specific service (e.g., make build-auth-service)
build-%:
	@echo "🔨 Building $*..."
	$(GO) build $(GOFLAGS) -o bin/$* ./cmd/$*
	@echo "✅ $* built successfully"

## test: Run all tests
test:
	@echo "🧪 Running tests..."
	$(GO) test -v ./... -cover

## test-unit: Run unit tests only
test-unit:
	@echo "🧪 Running unit tests..."
	$(GO) test -v -short ./...

## test-integration: Run integration tests
test-integration:
	@echo "🧪 Running integration tests..."
	$(GO) test -v -run Integration ./test/integration/...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "📊 Generating coverage report..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

## lint: Run linter
lint:
	@echo "🔍 Running linter..."
	$(GOLANGCI_LINT) run ./...

## fmt: Format code
fmt:
	@echo "✨ Formatting code..."
	$(GO) fmt ./...
	goimports -w .

## vet: Run go vet
vet:
	@echo "🔍 Running go vet..."
	$(GO) vet ./...

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	$(GO) clean

## docker-build: Build all Docker images
docker-build:
	@echo "🐳 Building Docker images..."
	$(DOCKER_COMPOSE) build

## docker-up: Start all services with Docker Compose
docker-up:
	@echo "🐳 Starting all services..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Services started. Check status: make docker-ps"

## docker-down: Stop all services
docker-down:
	@echo "🐳 Stopping all services..."
	$(DOCKER_COMPOSE) down

## docker-ps: Show running containers
docker-ps:
	$(DOCKER_COMPOSE) ps

## docker-logs: Tail logs from all services
docker-logs:
	$(DOCKER_COMPOSE) logs -f

## docker-logs-%: Tail logs from specific service
docker-logs-%:
	$(DOCKER_COMPOSE) logs -f $*

## init-db: Initialize databases	
init-db:
	@echo "🗄️  Initializing databases..."
	@chmod +x scripts/init-db.sh
	@./scripts/init-db.sh

## db-migrate: Run database migrations
db-migrate:
	@echo "🔄 Running migrations..."
	# Add migration tool command here

## run-dev: Run services in development mode
run-dev:
	@echo "🚀 Starting development environment..."
	$(DOCKER_COMPOSE) up -d cockroachdb cassandra redis minio
	@echo "Waiting for databases to be ready..."
	@sleep 5
	@echo "✅ Development environment ready"

## run-%: Run specific service locally
run-%:
	@echo "🚀 Running $*..."
	$(GO) run ./cmd/$*/main.go

## health: Check health of all services
health:
	@echo "🏥 Checking service health..."
	@curl -f http://localhost:8080/health || echo "❌ API Gateway"
	@curl -f http://localhost:8081/health || echo "❌ Auth Service"
	@curl -f http://localhost:8082/health || echo "❌ Chat Service"
	@curl -f http://localhost:8083/health || echo "❌ Video Service"
	@curl -f http://localhost:8084/health || echo "❌ Storage Service"

## gen-mocks: Generate mocks for testing
gen-mocks:
	@echo "🔧 Generating mocks..."
	mockgen -source=internal/repository/cockroach/call_repo.go -destination=test/mocks/mock_call_repo.go -package=mocks
	@echo "✅ Mocks generated"

## install-tools: Install development tools
install-tools:
	@echo "🔧 Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/golang/mock/mockgen@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "✅ Tools installed"

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "✅ All checks passed"