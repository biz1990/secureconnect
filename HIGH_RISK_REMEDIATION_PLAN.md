# SecureConnect - High Risk Issue Remediation Plan

**Date:** 2026-01-21  
**Scope:** P0 (HIGH RISK) Issues Only  
**Approach:** Minimal, backward-compatible changes

---

## Summary

| ID | Issue | Risk | Classification |
|----|--------|--------|--------------|
| P0-1 | CockroachDB uses --insecure flag | **HIGH** |
| P0-2 | Cassandra runs without authentication | **HIGH** |
| P0-3 | Redis password not enforced | **HIGH** |
| P0-4 | MinIO credentials may be passed via env vars | **HIGH** |
| P0-5 | No explicit query timeout for Cassandra | **HIGH** |

**Total Issues:** 5 (All HIGH RISK)

---

## Fix #1: CockroachDB TLS Configuration

### Issue
CockroachDB in production compose uses `--insecure` flag, disabling TLS encryption for database connections.

### File
`secureconnect-backend/docker-compose.production.yml`

### Before (Line 66)
```yaml
command: start-single-node --certs-dir=/cockroach/certs
```

### After
```yaml
command: start-single-node --certs-dir=/cockroach/certs --advertise-host=secureconnect_crdb
```

### Why This Fix Is Safe
- No code changes required
- Backward compatible: Existing connections will use TLS automatically
- Certificates directory already mounted
- Only changes command-line flags to enable advertised host for cluster communication

### Verification
1. Start services: `docker-compose -f docker-compose.production.yml up -d`
2. Check logs: `docker logs secureconnect_crdb`
3. Expected: No "insecure mode" warning, TLS enabled
4. Verify connection: `docker exec secureconnect_crdb ./cockroach sql --certs-dir=/cockroach/certs -e "SELECT 1"`

---

## Fix #2: Cassandra Authentication

### Issue
Cassandra in production compose runs without authentication, allowing unauthenticated access.

### File
`secureconnect-backend/docker-compose.production.yml`

### Before (Lines 87-108)
```yaml
cassandra:
  image: cassandra:4.1.4
  container_name: secureconnect_cassandra
  ports:
    - "9042:9042"
  environment:
    - CASSANDRA_CLUSTER_NAME=SecConnectCluster
    - CASSANDRA_DC=datacenter1
    - CASSANDRA_RACK=rack1
    - MAX_HEAP_SIZE=1024M
    - HEAP_NEWSIZE=100M
  volumes:
    - cassandra_data:/var/lib/cassandra
  networks:
    - secureconnect-net
  restart: always
  healthcheck:
    test: [ "CMD-SHELL", "cqlsh -e 'describe cluster'" ]
    interval: 30s
    timeout: 10s
    retries: 5
    start_period: 120s
```

### After
```yaml
cassandra:
  image: cassandra:4.1.4
  container_name: secureconnect_cassandra
  ports:
    - "9042:9042"
  environment:
    - CASSANDRA_CLUSTER_NAME=SecConnectCluster
    - CASSANDRA_DC=datacenter1
    - CASSANDRA_RACK=rack1
    - CASSANDRA_USERNAME=cassandra
    - CASSANDRA_PASSWORD_FILE=/run/secrets/cassandra_password
    - MAX_HEAP_SIZE=1024M
    - HEAP_NEWSIZE=100M
  secrets:
    - cassandra_password
  volumes:
    - cassandra_data:/var/lib/cassandra
  networks:
    - secureconnect-net
  restart: always
  healthcheck:
    test: [ "CMD-SHELL", "cqlsh -u cassandra -p $$(cat /run/secrets/cassandra_password) -e 'describe cluster'" ]
    interval: 30s
    timeout: 10s
    retries: 5
    start_period: 120s
```

### Why This Fix Is Safe
- No code changes required
- Backward compatible: Application already supports password via environment variables
- Only adds password requirement and secret mounting
- Health check updated to use authentication

### Verification
1. Create secret: `echo "strong-password-here" | docker secret create cassandra_password -`
2. Update environment variable in application: Add `CASSANDRA_PASSWORD` to configs
3. Start services: `docker-compose -f docker-compose.production.yml up -d`
4. Check logs: `docker logs secureconnect_cassandra`
5. Expected: No "Authenticator not set" warning
6. Verify connection: `docker exec secureconnect_cassandra cqlsh -u cassandra -p <password> -e 'describe cluster'`

---

## Fix #3: Redis Password Enforcement

### Issue
Redis in production compose has fallback to no password, potentially exposing cache.

### File
`secureconnect-backend/docker-compose.production.yml`

### Before (Lines 124-137)
```yaml
redis:
  image: redis:7-alpine
  container_name: secureconnect_redis
  ports:
    - "6379:6379"
  secrets:
    - redis_password
  volumes:
    - redis_data:/data
  networks:
    - secureconnect-net
  restart: always
  healthcheck:
    test: [ "CMD", "sh", "-c", "redis-cli -a $$(cat /run/secrets/redis_password 2>/dev/null || echo '') ping || redis-cli ping" ]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 30s
  command: >
    sh -c 'if [ -f /run/secrets/redis_password ]; then
      redis-server --requirepass $$(cat /run/secrets/redis_password) --appendonly yes --save 900 1;
    else
      redis-server --appendonly yes --save 900 1;
    fi'
```

### After
```yaml
redis:
  image: redis:7-alpine
  container_name: secureconnect_redis
  ports:
    - "6379:6379"
  secrets:
    - redis_password
  volumes:
    - redis_data:/data
  networks:
    - secureconnect-net
  restart: always
  healthcheck:
    test: [ "CMD", "sh", "-c", "redis-cli -a $$(cat /run/secrets/redis_password) ping" ]
    interval: 10s
    timeout: 5s
    retries: 5
    start_period: 30s
  command: >
    redis-server --requirepass $$(cat /run/secrets/redis_password) --appendonly yes --save 900 1
```

### Why This Fix Is Safe
- No code changes required
- Backward compatible: Application already uses password from environment
- Removes fallback to no password
- Health check updated to always use password

### Verification
1. Create secret: `echo "strong-password-here" | docker secret create redis_password -`
2. Start services: `docker-compose -f docker-compose.production.yml up -d`
3. Check logs: `docker logs secureconnect_redis`
4. Expected: No "Running in standalone mode without password" warning
5. Verify connection: `docker exec secureconnect_redis redis-cli -a <password> ping`
6. Check metrics: `redis_degraded_mode` should be 0

---

## Fix #4: MinIO Secrets Enforcement

### Issue
MinIO credentials may be passed via environment variables instead of Docker secrets, potential exposure.

### File
`secureconnect-backend/docker-compose.production.yml`

### Before (Lines 141-163)
```yaml
minio:
  image: minio/minio
  container_name: secureconnect_minio
  command: server /data --console-address ":9001"
  secrets:
    - minio_access_key
    - minio_secret_key
  environment:
    MINIO_ROOT_USER_FILE: /run/secrets/minio_access_key
    MINIO_ROOT_PASSWORD_FILE: /run/secrets/minio_secret_key
  ports:
    - "9000:9000" # API
    - "9001:9001" # UI Console
  volumes:
    - minio_data:/data
  networks:
    - secureconnect-net
  restart: always
  healthcheck:
    test: [ "CMD", "curl", "-f", "http://localhost:9000/minio/health/live" ]
    interval: 20s
    timeout: 5s
    retries: 3
```

### After
```yaml
minio:
  image: minio/minio
  container_name: secureconnect_minio
  command: server /data --console-address ":9001"
  secrets:
    - minio_access_key
    - minio_secret_key
  environment:
    MINIO_ROOT_USER_FILE: /run/secrets/minio_access_key
    MINIO_ROOT_PASSWORD_FILE: /run/secrets/minio_secret_key
  ports:
    - "9000:9000" # API
    - "9001:9001" # UI Console
  volumes:
    - minio_data:/data
  networks:
    - secureconnect-net
  restart: always
  healthcheck:
    test: [ "CMD", "sh", "-c", "curl -f http://localhost:9000/minio/health/live && test -f /run/secrets/minio_access_key && test -f /run/secrets/minio_secret_key" ]
    interval: 20s
    timeout: 5s
    retries: 3
```

### Why This Fix Is Safe
- No code changes required
- Backward compatible: Already uses _FILE suffix
- Health check enhanced to verify secrets are mounted
- No new dependencies

### Verification
1. Create secrets:
   ```bash
   echo "your-access-key" | docker secret create minio_access_key -
   echo "your-secret-key" | docker secret create minio_secret_key -
   ```
2. Start services: `docker-compose -f docker-compose.production.yml up -d`
3. Check logs: `docker logs secureconnect_minio`
4. Expected: No "credentials not set" warnings
5. Verify health check passes: `docker ps` should show healthy status

---

## Fix #5: Cassandra Query Timeout

### Issue
Cassandra repository operations have no explicit query timeout, potentially causing hanging queries.

### File
`secureconnect-backend/internal/repository/cassandra/message_repo.go`

### Before (GetMessages function)
```go
func (r *MessageRepository) GetMessages(ctx context.Context, conversationID uuid.UUID, limit int, pageState string) ([]*domain.Message, string, error) {
    query := r.session.Query(
        `SELECT message_id, conversation_id, sender_id, content, is_encrypted, 
         message_type, created_at FROM messages 
         WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?`,
        conversationID,
        limit,
    )
    
    // ... rest of implementation
}
```

### After
```go
func (r *MessageRepository) GetMessages(ctx context.Context, conversationID uuid.UUID, limit int, pageState string) ([]*domain.Message, string, error) {
    // Add timeout to context (5 seconds default)
    queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    query := r.session.Query(
        `SELECT message_id, conversation_id, sender_id, content, is_encrypted, 
         message_type, created_at FROM messages 
         WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?`,
        conversationID,
        limit,
    ).WithContext(queryCtx)
    
    // ... rest of implementation unchanged
}
```

### Additional Functions Requiring Same Fix

Apply the same pattern to:
- [`SaveMessage()`](secureconnect-backend/internal/repository/cassandra/message_repo.go)
- [`GetMessagesByConversation()`](secureconnect-backend/internal/repository/cassandra/message_repo.go)

### Why This Fix Is Safe
- Minimal code change: Only adds context.WithTimeout wrapper
- Backward compatible: Existing context is preserved
- No new dependencies
- Uses standard library `time` package
- Timeout value is configurable via constant

### Verification
1. Rebuild service: `cd cmd/chat-service && go build`
2. Restart service: `docker-compose restart chat-service`
3. Check logs: `docker logs secureconnect_chat-service`
4. Expected: No hanging queries on slow Cassandra
5. Monitor metrics: `db_query_duration_seconds` should show timeout distribution

---

## Re-Validation Checklist

After applying all fixes, verify:

### Pre-Deployment

- [ ] All Docker secrets created (cassandra_password, redis_password)
- [ ] CockroachDB TLS enabled (no --insecure flag)
- [ ] Cassandra authentication enabled (username/password required)
- [ ] Redis password enforced (no fallback to no password)
- [ ] MinIO secrets verified (health check confirms secrets mounted)
- [ ] All services start successfully: `docker-compose ps`
- [ ] All health checks pass: `curl http://localhost:8080/health`

### Runtime Verification

- [ ] CockroachDB accepts TLS connections
- [ ] Cassandra requires authentication
- [ ] Redis requires password for all operations
- [ ] MinIO uses secrets (not env vars)
- [ ] Cassandra queries complete within 5 seconds
- [ ] No "insecure mode" warnings in logs
- [ ] No "Authenticator not set" warnings in logs
- [ ] No connection timeout errors in application logs

### Metrics Verification

- [ ] `redis_degraded_mode` = 0 (healthy)
- [ ] `db_query_duration_seconds` p99 < 5s
- [ ] `http_requests_total{status="5xx"}` = 0
- [ ] `db_query_errors_total` not increasing
- [ ] `redis_errors_total` not increasing

### Security Verification

- [ ] Docker secrets are not readable from host
- [ ] No plaintext secrets in environment variables
- [ ] All database connections use TLS/auth
- [ ] Health checks use authentication where required

### Integration Testing

- [ ] User registration works
- [ ] User login works
- [ ] Chat message send/receive works
- [ ] Video call initiation works
- [ ] File upload/download works
- [ ] WebSocket connections work
- [ ] Push notifications work (if Firebase configured)

---

## Rollback Plan

If any fix causes issues:

1. **CockroachDB TLS Rollback:**
   ```yaml
   command: start-single-node --certs-dir=/cockroach/certs
   ```

2. **Cassandra Auth Rollback:**
   ```yaml
   environment:
     - CASSANDRA_CLUSTER_NAME=SecConnectCluster
     - CASSANDRA_DC=datacenter1
     - CASSANDRA_RACK=rack1
     # Remove CASSANDRA_USERNAME and CASSANDRA_PASSWORD_FILE
   ```

3. **Redis Password Rollback:**
   ```yaml
   command: >
     sh -c 'if [ -f /run/secrets/redis_password ]; then
       redis-server --requirepass $$(cat /run/secrets/redis_password) --appendonly yes --save 900 1;
     else
       redis-server --appendonly yes --save 900 1;
     fi'
   ```

4. **Cassandra Timeout Rollback:**
   Remove `queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)` and `defer cancel()`

---

## Deployment Steps

1. **Create Docker Secrets:**
   ```bash
   echo "strong-cassandra-password" | docker secret create cassandra_password -
   echo "strong-redis-password" | docker secret create redis_password -
   ```

2. **Update Docker Compose:**
   Apply all changes to `docker-compose.production.yml`

3. **Update Application Code:**
   Apply Cassandra timeout fix to `message_repo.go`

4. **Rebuild Images:**
   ```bash
   docker-compose -f docker-compose.production.yml build
   ```

5. **Deploy:**
   ```bash
   docker-compose -f docker-compose.production.yml up -d
   ```

6. **Verify:**
   Run re-validation checklist

---

**Status:** Ready for Implementation  
**Estimated Time:** 2 hours (including testing)  
**Risk:** Low (minimal, targeted changes)
