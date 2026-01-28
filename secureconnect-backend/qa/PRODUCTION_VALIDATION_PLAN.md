# Production Validation Plan

**Role:** Principal Production QA Engineer
**System:** SecureConnect Backend
**Environment:** Production (Docker Compose)

## 1. Authentication & Authorization
- [ ] **Register**: Create new user (success case).
- [ ] **Login**: Authenticate user, receive JWT (Access + Refresh).
- [ ] **Refresh**: Use refresh token to get new access token.
- [ ] **Security**: Verify JWT expiration and invalid token handling.

## 2. Chat Functionality
- [ ] **Send Message (HTTP)**: Send text message to conversation.
- [ ] **Get Messages**: Retrieve conversation history.
- [ ] **WebSocket**: Verify connection upgrade (handshake).

## 3. Video & TURN
- [ ] **SFU Health**: Verify video-service health.
- [ ] **TURN Config**: Inspect `turnserver.conf` for secure settings (auth, realm).
- [ ] **Signaling**: Verify signaling endpoints accessibility.

## 4. Storage (MinIO)
- [ ] **Upload URL**: specific file upload pre-signed URL generation.
- [ ] **Download URL**: specific file access URL generation.
- [ ] **Persistence**: Verify file metadata in DB.

## 5. Resilience & Reliability
- [ ] **Redis Degraded Mode**: Stop Redis, verify partial system functionality (if supported) or graceful failure.
- [ ] **Cassandra Timeouts**: Simulate timeout (via config or network) and check error handling.
- [ ] **Failure Handling**: Verify restart policies and error responses.

## 6. Observability
- [ ] **Metrics**: Verify `/metrics` endpoints for Prometheus scraping.
- [ ] **Logging**: Verify structured (JSON) logging in container logs.

## 7. Configuration Security
- [ ] **Secrets**: Verify no plain-text secrets in env vars (inspect).
- [ ] **TLS/SSL**: Check NGINX config for TLS termination settings (if applicable in this env).

## Report Output
- `QA_VALIDATION_REPORT.md`: Findings, Severity, Fix Recommendations.
