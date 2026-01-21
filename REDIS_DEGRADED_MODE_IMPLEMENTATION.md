# Redis Degraded Mode Implementation Summary

**Date:** 2026-01-19
**Author:** Backend Engineer

---

## Executive Summary

Implemented a comprehensive Redis degraded mode framework that allows all SecureConnect services to gracefully handle Redis unavailability without panics or data corruption.

**Key Features:**
- ✅ Degraded mode detection with health checks
- ✅ Prometheus metric exposure (`redis_degraded_mode`)
- ✅ Safe wrapper methods for all Redis operations
- ✅ Thread-safe degraded mode state management
- ✅ No panics - all Redis operations return errors gracefully
- ✅ Clear code comments marking degraded paths

---

## 1. Core Implementation

### File: [`internal/database/redis.go`](secureconnect-backend/internal/database/redis.go:1)

### 1.1 New RedisClient Structure

```go
type RedisClient struct {
    Client         *redis.Client
    degradedMode   bool
    degradedModeMu  sync.RWMutex
    healthCheckMu   sync.Mutex
    metrics         *redisMetrics
}
```

**Design Decisions:**
- `degradedModeMu` (RWMutex): Allows concurrent reads of degraded mode state
- `healthCheckMu` (Mutex): Prevents concurrent health checks from overwhelming Redis
- `metrics`: Prometheus metrics for observability

### 1.2 Prometheus Metrics

```go
type redisMetrics struct {
    degradedMode prometheus.Gauge
    healthCheck  prometheus.Counter
}

// redis_degraded_mode: 1 = degraded, 0 = healthy
// redis_health_check_total: Total number of health checks
```

**Metric Name:** `redis_degraded_mode`
**Metric Type:** Gauge
**Labels:** None (can be extended with `service` label)

---

## 2. Degraded Mode Detection

### 2.1 HealthCheck Method

```go
func (r *RedisClient) HealthCheck(ctx context.Context) error {
    r.healthCheckMu.Lock()
    defer r.healthCheckMu.Unlock()
    
    // Use a short timeout for health checks
    healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    
    err := r.Client.Ping(healthCtx).Err()
    if err != nil {
        // Redis is unavailable, enter degraded mode
        r.setDegradedMode(true)
        return fmt.Errorf("redis health check failed: %w", err)
    }
    
    // Redis is healthy, exit degraded mode
    r.setDegradedMode(false)
    
    // Increment health check counter
    if r.metrics != nil {
        metrics := getRedisMetrics()
        metrics.healthCheck.Inc()
    }
    
    return nil
}
```

**Design Decisions:**
- **2-second timeout**: Prevents health checks from hanging
- **Mutex protection**: Prevents concurrent health checks from overwhelming Redis
- **Automatic recovery**: Exits degraded mode when Redis becomes healthy
- **Metrics tracking**: Increments `redis_health_check_total` counter

### 2.2 Degraded Mode State Management

```go
func (r *RedisClient) setDegradedMode(degraded bool) {
    r.degradedModeMu.Lock()
    defer r.degradedModeMu.Unlock()
    
    if r.degradedMode != degraded {
        r.degradedMode = degraded
        if r.metrics != nil {
            metrics := getRedisMetrics()
            if degraded {
                metrics.degradedMode.Set(1)
            } else {
                metrics.degradedMode.Set(0)
            }
        }
    }
}
```

**Design Decisions:**
- **State change tracking**: Only updates metrics when state actually changes
- **Thread-safe**: Uses RWMutex for concurrent access
- **Lazy initialization**: Metrics initialized on first use

---

## 3. Safe Redis Operation Wrappers

All Redis operations have safe wrapper methods that check degraded mode before executing:

### 3.1 Basic Operations

| Method | Redis Operation | Degraded Behavior |
|--------|----------------|------------------|
| `SafePing()` | PING | Returns error "redis is in degraded mode, ping skipped" |
| `SafeGet()` | GET | Returns empty result with error "redis is in degraded mode, get skipped" |
| `SafeSet()` | SET | Returns error "redis is in degraded mode, set skipped" |
| `SafeDel()` | DEL | Returns 0 with error "redis is in degraded mode, del skipped" |

### 3.2 Hash Operations

| Method | Redis Operation | Degraded Behavior |
|--------|----------------|------------------|
| `SafeHSet()` | HSET | Returns false with error "redis is in degraded mode, hset skipped" |
| `SafeHGet()` | HGET | Returns empty result with error "redis is in degraded mode, hget skipped" |
| `SafeHDel()` | HDEL | Returns 0 with error "redis is in degraded mode, hdel skipped" |

### 3.3 Pub/Sub Operations

| Method | Redis Operation | Degraded Behavior |
|--------|----------------|------------------|
| `SafePublish()` | PUBLISH | Returns 0 with error "redis is in degraded mode, publish skipped" |
| `SafeSubscribe()` | SUBSCRIBE | Returns nil (no subscription in degraded mode) |

**Note:** `SafeSubscribe()` returns `nil` instead of an error because a failed subscription is not a critical error - services should continue without pub/sub.

### 3.4 Sorted Set Operations

| Method | Redis Operation | Degraded Behavior |
|--------|----------------|------------------|
| `SafeZAdd()` | ZADD | Returns 0 with error "redis is in degraded mode, zadd skipped" |
| `SafeZRem()` | ZREM | Returns 0 with error "redis is in degraded mode, zrem skipped" |
| `SafeZRange()` | ZRANGE | Returns empty slice with error "redis is in degraded mode, zrange skipped" |
| `SafeExpire()` | EXPIRE | Returns false with error "redis is in degraded mode, expire skipped" |

---

## 4. Usage Guidelines

### 4.1 Initialization

```go
// Use NewRedisDB instead of NewRedisClient
redisClient, err := database.NewRedisDB(&database.RedisConfig{
    Host:     os.Getenv("REDIS_HOST"),
    Port:     6379,
    Password: os.Getenv("REDIS_PASSWORD"),
    DB:       0,
    PoolSize: 100,
    Timeout:  5 * time.Second,
})
```

### 4.2 Health Check Pattern

```go
// Periodic health check (e.g., every 10 seconds)
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        ctx := context.Background()
        if err := redisClient.HealthCheck(ctx); err != nil {
            logger.Warn("Redis health check failed", 
                zap.Error(err),
                zap.String("service", "api-gateway"))
        }
    }
}()
```

### 4.3 Operation Pattern

```go
// Before: Direct Redis call (fails when Redis is down)
err := redisClient.Get(ctx, key).Err()

// After: Safe wrapper (gracefully handles degraded mode)
result := redisClient.SafeGet(ctx, key)
if result.Err() != nil {
    if redisClient.IsDegraded() {
        // Expected: Redis is in degraded mode
        logger.Warn("Redis operation skipped", zap.Error(result.Err()))
    } else {
        // Unexpected: Redis is healthy but operation failed
        logger.Error("Redis operation failed", zap.Error(result.Err()))
    }
}
```

---

## 5. Service-Specific Implementation

### 5.1 API Gateway - In-Memory Rate Limiting

**Requirements:**
- Fallback to in-memory rate limiting when Redis is degraded
- Track request counts per IP/user in memory
- Log degraded mode warnings

**Implementation Pattern:**
```go
type InMemoryRateLimiter struct {
    mu     sync.RWMutex
    limits map[string]*rate.Limiter
}

func (r *APIGateway) handleRequest(c *gin.Context) {
    // Try Redis rate limiting first
    if !r.redis.IsDegraded() {
        // Use Redis-based rate limiting
        if !checkRedisRateLimit(r.redis, c.ClientIP()) {
            c.JSON(429, gin.H{"error": "rate limit exceeded"})
            c.Abort()
            return
        }
    } else {
        // Redis is degraded, use in-memory rate limiting
        logger.Warn("Using in-memory rate limiting (Redis degraded)")
        if !checkInMemoryRateLimit(r.inMemoryLimiter, c.ClientIP()) {
            c.JSON(429, gin.H{"error": "rate limit exceeded"})
            c.Abort()
            return
        }
    }
    // ... continue with request
}
```

### 5.2 Auth Service - Stateless Login

**Requirements:**
- Allow login without Redis for session storage
- JWT-based authentication only
- Skip Redis session storage in degraded mode

**Implementation Pattern:**
```go
func (s *AuthService) Login(c *gin.Context, req LoginRequest) {
    // Validate credentials against database
    user, err := s.userRepo.FindByEmail(req.Email)
    if err != nil || !checkPassword(user, req.Password) {
        c.JSON(401, gin.H{"error": "invalid credentials"})
        return
    }
    
    // Generate JWT token (stateless, no Redis needed)
    token, err := s.jwt.GenerateToken(user.ID)
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to generate token"})
        return
    }
    
    // Try to store session in Redis
    if !s.redis.IsDegraded() {
        if err := s.redis.SafeSet(ctx, sessionKey, token, 24*time.Hour); err != nil {
            // Session storage failed, but login still works
            logger.Warn("Failed to store session in Redis", zap.Error(err))
        }
    } else {
        // Redis is degraded, skip session storage
        logger.Warn("Session storage skipped (Redis degraded)")
    }
    
    c.JSON(200, gin.H{"token": token})
}
```

### 5.3 Chat Service - Skip Presence Updates

**Requirements:**
- Skip presence updates when Redis is degraded
- Continue message delivery via Cassandra
- Log degraded mode warnings

**Implementation Pattern:**
```go
func (s *ChatService) UpdatePresence(userID string, status string) {
    if s.redis.IsDegraded() {
        // Skip presence update in degraded mode
        logger.Warn("Presence update skipped (Redis degraded)",
            zap.String("user_id", userID),
            zap.String("status", status))
        return nil
    }
    
    // Store presence in Redis
    if err := s.redis.SafeHSet(ctx, presenceKey, status); err != nil {
        logger.Error("Failed to update presence", zap.Error(err))
        return err
    }
    
    return nil
}
```

### 5.4 Video Service - Local-Only Signaling

**Requirements:**
- Use in-memory signaling when Redis is degraded
- Skip Redis pub/sub for signaling
- Log degraded mode warnings

**Implementation Pattern:**
```go
type SignalingHub struct {
    mu       sync.RWMutex
    rooms    map[string]*Room
    redis    *database.RedisClient
}

func (h *SignalingHub) HandleSignal(userID, roomID, signal Signal) {
    if h.redis.IsDegraded() {
        // Use in-memory signaling
        h.mu.Lock()
        room, ok := h.rooms[roomID]
        h.mu.Unlock()
        
        if ok {
            room.Broadcast(signal)
        }
        logger.Warn("Using in-memory signaling (Redis degraded)")
        return nil
    }
    
    // Use Redis pub/sub for signaling
    if err := h.redis.SafePublish(ctx, signalingChannel, signal); err != nil {
        logger.Error("Failed to publish signal", zap.Error(err))
        return err
    }
    
    return nil
}
```

---

## 6. Testing Guidelines

### 6.1 Unit Tests

```go
func TestRedisDegradedMode(t *testing.T) {
    // Create Redis client
    redisClient := NewRedisClient(":6379")
    
    // Test normal operation
    ctx := context.Background()
    err := redisClient.HealthCheck(ctx)
    assert.Nil(t, err)
    assert.False(t, redisClient.IsDegraded())
    
    // Simulate Redis failure (stop Redis container)
    // docker stop secureconnect_redis
    
    // Test degraded mode detection
    err = redisClient.HealthCheck(ctx)
    assert.Error(t, err)
    assert.True(t, redisClient.IsDegraded())
    
    // Test safe operations in degraded mode
    result := redisClient.SafeGet(ctx, "test-key")
    assert.Error(t, result.Err())
    assert.Contains(t, result.Err().Error(), "redis is in degraded mode")
    
    // Test recovery (start Redis container)
    // docker start secureconnect_redis
    
    // Test automatic recovery
    err = redisClient.HealthCheck(ctx)
    assert.Nil(t, err)
    assert.False(t, redisClient.IsDegraded())
}
```

### 6.2 Integration Tests

```bash
# Test degraded mode with actual services
1. Start all services
2. Verify Redis health (all services healthy)
3. Stop Redis: docker stop secureconnect_redis
4. Verify degraded mode activated:
   - Check Prometheus: `redis_degraded_mode` = 1
   - Check logs for "Redis is in degraded mode" warnings
5. Test degraded behavior:
   - API Gateway: requests still work with in-memory rate limiting
   - Auth Service: login works without session storage
   - Chat Service: messages work, presence updates skipped
   - Video Service: calls work with in-memory signaling
6. Start Redis: docker start secureconnect_redis
7. Verify automatic recovery:
   - Check Prometheus: `redis_degraded_mode` = 0
   - Verify normal operation resumes
```

---

## 7. Prometheus Alerting

### 7.1 Alert Rule

```yaml
groups:
  - name: redis_degraded
    rules:
      - alert: RedisDegradedMode
        expr: redis_degraded_mode == 1
        for: 5m
        labels:
          severity: warning
          service: redis
        annotations:
          summary: "Redis is in degraded mode"
          description: "Services are operating with reduced functionality due to Redis unavailability"
```

### 7.2 Dashboard Panel

```json
{
  "title": "Redis Degraded Mode",
  "targets": [
    {
      "expr": "redis_degraded_mode",
      "refId": "redis-degraded-mode",
      "legendFormat": "Degraded Mode",
      "type": "gauge"
    }
  ],
  "fieldConfig": {
    "defaults": {
      "min": 0,
      "max": 1
    },
    "mappings": [
      {
        "value": 0,
        "text": "Healthy"
      },
      {
        "value": 1,
        "text": "Degraded"
      }
    ]
  }
}
```

---

## 8. Deployment Checklist

- [ ] Update all services to use `NewRedisDB` instead of `NewRedisClient`
- [ ] Add periodic health checks to all services
- [ ] Implement service-specific degraded mode handling
- [ ] Add `redis_degraded_mode` metric to Prometheus
- [ ] Configure Prometheus alert for `redis_degraded_mode == 1`
- [ ] Add Redis degraded mode panel to Grafana dashboard
- [ ] Test degraded mode with all services
- [ ] Update runbook with Redis degraded mode procedures

---

## 9. Rollback Plan

If degraded mode causes issues:

1. **Immediate:** Disable health checks temporarily
   ```go
   // Comment out health check goroutines
   // go func() { ... }()
   ```

2. **Alternative:** Use longer health check timeout
   ```go
   healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
   ```

3. **Fallback:** Disable degraded mode entirely
   ```go
   // Always call Redis operations directly
   // Remove IsDegraded() checks
   ```

---

## 10. Monitoring and Observability

### 10.1 Metrics to Monitor

| Metric | Type | Expected Values |
|--------|------|----------------|
| `redis_degraded_mode` | Gauge | 0 (healthy) or 1 (degraded) |
| `redis_health_check_total` | Counter | Increments on each health check |
| `http_requests_total` | Counter | Should continue in degraded mode |
| `http_request_duration_seconds` | Histogram | May increase in degraded mode |

### 10.2 Logs to Monitor

| Log Level | Pattern | Severity |
|-----------|---------|----------|
| WARN | "redis is in degraded mode" | Normal during degradation |
| WARN | "Using in-memory rate limiting" | Normal during degradation |
| WARN | "Session storage skipped" | Normal during degradation |
| ERROR | "Redis operation failed" | Investigate if not degraded |
| ERROR | "Failed to update presence" | Investigate if not degraded |

---

## Summary

**Implementation Status:** ✅ **COMPLETE**

The Redis degraded mode framework is implemented and ready for integration into all services. The implementation:

1. ✅ Provides thread-safe degraded mode detection
2. ✅ Exposes Prometheus metrics for observability
3. ✅ Includes safe wrapper methods for all Redis operations
4. ✅ Prevents panics by returning errors gracefully
5. ✅ Supports automatic recovery when Redis becomes healthy
6. ✅ Includes clear code comments for maintenance

**Next Steps:**
1. Integrate `NewRedisDB` into all service initializations
2. Add periodic health checks to all services
3. Implement service-specific degraded mode handling patterns
4. Add Prometheus alerting rules
5. Test degraded mode with all services

**Estimated Integration Time:** 4-6 hours

---

**Document Version:** 1.0
**Last Updated:** 2026-01-19T09:06:00Z
