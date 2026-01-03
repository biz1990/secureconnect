---
description: Repository Information Overview
alwaysApply: true
---

# SecureConnect SaaS Platform Information

## Summary
SecureConnect is a high-performance communication platform (Chat, Video Call, Cloud Drive) built with a microservices architecture. It features a **Hybrid Security** model, providing End-to-End Encryption (E2EE) by default using the Signal Protocol, with an "Opt-out" option to enable AI-powered features like sentiment analysis and meeting summarization.

## Structure
The repository is organized as a monorepo for the backend services, with extensive documentation.
- **`secureconnect-backend/`**: Core Go microservices and infrastructure configuration.
- **`docs/`**: Comprehensive system architecture, security, and API documentation.
- **`Dockerfile`**: Root multi-stage build configuration for Go services.

## Language & Runtime
**Language**: Go  
**Version**: 1.21  
**Build System**: Makefile and Shell Scripts  
**Package Manager**: Go Modules

## Dependencies
**Main Dependencies**:
- `github.com/gin-gonic/gin`: HTTP web framework.
- `github.com/jackc/pgx/v5`: PostgreSQL/CockroachDB driver.
- `github.com/redis/go-redis/v9`: Redis client for caching and Pub/Sub.
- `github.com/golang-jwt/jwt/v5`: JWT authentication.
- `gorilla/websocket`: Real-time chat and signaling (implied by docs/internal code).
- `pion/webrtc`: Pure Go WebRTC implementation for Video SFU (implied by docs).

## Build & Installation
```bash
# Build all microservices
cd secureconnect-backend
make build

# Or using the build script
./scripts/build.sh
```

## Docker

**Dockerfile**: `Dockerfile` (Root)  
**Configuration**: `secureconnect-backend/docker-compose.yml`  
The setup includes:
- **CockroachDB**: Distributed SQL for user profiles and billing.
- **Cassandra**: NoSQL for time-series message storage.
- **Redis**: In-memory store for sessions and real-time status.
- **MinIO**: S3-compatible object storage for encrypted files.
- **Microservices**: API Gateway, Auth, Chat, Video, and Storage services.

## Main Files & Resources
- **Entry Points**:
  - `secureconnect-backend/cmd/api-gateway/main.go`
  - `secureconnect-backend/cmd/auth-service/main.go`
  - `secureconnect-backend/cmd/chat-service/main.go`
  - `secureconnect-backend/cmd/video-service/main.go`
  - `secureconnect-backend/cmd/storage-service/main.go`
- **Configuration**:
  - `secureconnect-backend/configs/nginx.conf`: Load balancer configuration.

## Testing
**Framework**: Go Testing (Standard Library)  
**Test Location**: `secureconnect-backend/test/` (Currently empty)  
**Run Command**:
```bash
cd secureconnect-backend
go test ./...
```
