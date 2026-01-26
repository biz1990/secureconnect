# REDIS DEGRADED MODE DESIGN

## Overview

This document defines a degraded mode strategy for Redis failures that allows the system to remain partially operational when Redis is unavailable, without data corruption or breaking API changes.

## Design Principles

1. **Fail-Closed for Security**: Operations that MUST fail to protect data integrity
2. **Fail-Open for Availability**: Operations that can continue with reduced functionality
3. **No API Contract Changes**: All endpoints continue to exist and return expected responses
4. **Explicit Degraded State**: System clearly communicates degraded mode via metrics and logs
5. **Automatic Recovery**: System automatically returns to normal operation when Redis recovers
6. **No Data Corruption**: Degraded mode doesn't create inconsistent state

---

## SERVICE-BY-SERVICE ANALYSIS

### 1. API Gateway

**Redis Usage:**
- Rate limiting (AdvancedRateLimiter)

**Fail-Closed Operations:**
- ❌ None (all rate limiting depends on Redis)

**Fail-Open Operations:**
- ✅ All HTTP routing and proxying
- ✅ JWT validation and revocation checking
- ✅ Request logging
- ✅ Response handling

**Degraded Mode Behavior:**
- **Rate Limiting**: Bypassed - all requests allowed
- **Impact**: Increased vulnerability to DoS attacks
- **Mitigation**: Add in-memory rate limiter as fallback

**Strategy:**
```go
// In-memory fallback rate limiter
type InMemoryRateLimiter struct {
    counters map[string]int64
    mu       sync.RWMutex
    window   time.Duration
}

func (rl *InMemoryRateLimiter) Allow(ip, endpoint string) bool {
    // Simple token bucket or sliding window
    // Returns true if request allowed
}
```

---

### 2. Auth Service

**Redis Usage:**
- Session storage (CreateSession, GetSession, DeleteSession)
- Token blacklisting (BlacklistToken, IsTokenBlacklisted)
- Directory lookups (SetEmailToUserID, SetUsernameToUserID)
- Presence (SetUserOnline, SetUserOffline)
- Failed login attempts (GetFailedLoginAttempts, SetFailedLoginAttempt)
- Account lockout (GetAccountLock, LockAccount)

**Fail-Closed Operations:**
- ❌ Login (requires session creation)
- ❌ Logout (requires session deletion)
- ❌ Token refresh (requires blacklisting old token)
- ❌ Password reset (requires token storage)
- ❌ Account lockout (requires lock storage)

**Fail-Open Operations:**
- ✅ User registration (no Redis dependency)
- ✅ User profile retrieval (from CockroachDB)
- ✅ Password hashing and comparison
- ✅ Email verification (from CockroachDB)
- ✅ User CRUD operations (from CockroachDB)

**Degraded Mode Behavior:**
- **Authentication**: Users can still login if credentials are valid, but sessions won't persist
- **Sessions**: Not stored, users must re-authenticate on each request
- **Rate Limiting**: Bypassed, no account lockout protection
- **Token Revocation**: Cannot blacklist tokens, logout doesn't invalidate sessions
- **Presence**: Not tracked, always shows "offline"

**Strategy:**
- Allow login without session creation (stateless auth)
- Return short-lived access tokens (e.g., 5 minutes instead of 15)
- Log degraded mode prominently

---

### 3. Chat Service

**Redis Usage:**
- Presence (SetUserOnline, SetUserOffline)
- Pub/Sub for distributed messaging (subscribeToCall)
- Directory lookups (via presence repository)

**Fail-Closed Operations:**
- ❌ Presence updates (online/offline status)
- ❌ Distributed messaging (pub/sub)
- ❌ User directory lookups

**Fail-Open Operations:**
- ✅ Message sending/retrieval (from Cassandra)
- ✅ Conversation management (from CockroachDB)
- ✅ User CRUD operations (from CockroachDB)
- ✅ WebSocket connections (in-memory hub)

**Degraded Mode Behavior:**
- **Presence**: Always shows users as "offline"
- **Real-time**: No distributed messaging, only in-memory WebSocket
- **Directory**: Cannot look up users by email/username

**Strategy:**
- Continue WebSocket operations (in-memory hub works for single instance)
- Log presence as "unknown" in degraded mode
- Allow message sending without presence updates

---

### 4. Video Service

**Redis Usage:**
- Pub/Sub for signaling (subscribeToCall, broadcast)
- Push token storage (via PushTokenRepository)
- Call state (NOT CURRENTLY STORED - this is a gap)

**Fail-Closed Operations:**
- ❌ Signaling pub/sub (distributed signaling)
- ❌ Push notifications (requires token storage)

**Fail-Open Operations:**
- ✅ Call initiation (from CockroachDB)
- ✅ Call ending (from CockroachDB)
- ✅ Participant management (from CockroachDB)
- ✅ WebSocket signaling (in-memory hub for single instance)

**Degraded Mode Behavior:**
- **Signaling**: No distributed messaging, only in-memory WebSocket
- **Push Notifications**: Cannot send push notifications
- **Call Recovery**: Cannot recover active calls (already a gap)

**Strategy:**
- Continue in-memory signaling (works for single instance)
- Log signaling as "local-only" in degraded mode
- Skip push notifications with warning log

---

### 5. Storage Service

**Redis Usage:**
- None identified in code review

**Fail-Closed Operations:**
- ❌ None

**Fail-Open Operations:**
- ✅ All file operations (upload, download, delete)
- ✅ Quota management (from CockroachDB)
- ✅ MinIO operations (direct connection)

**Degraded Mode Behavior:**
- **No Impact**: Service operates normally without Redis

**Strategy:**
- No changes needed - service is Redis-independent

---

## CODE IMPLEMENTATION

### Enhanced Redis Client with Degraded Mode

**File:** `secureconnect-backend/internal/database/redis.go`

```go
package database

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
)

type RedisConfig struct {
	Host           string
	Port           int
	Password       string
	DB             int
	PoolSize       int
	Timeout        time.Duration
	EnableFallback bool // Enable degraded mode when Redis is unavailable
}

type RedisClient struct {
	Client     *redis.Client
	available int32 // Track Redis availability atomically
}

func NewRedisClient(addr string) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		MaxRetries: 3, // Add retry for transient failures
	})
	return &RedisClient{Client: client, available: 1}
}

// NewRedisDB creates a new Redis client from config with fallback support
func NewRedisDB(cfg *RedisConfig) (*RedisClient, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client := redis.NewClient(&redis.Options{
		Addr:           addr,
		Password:       cfg.Password,
		DB:             cfg.DB,
		PoolSize:       cfg.PoolSize,
		ReadTimeout:    cfg.Timeout,
		WriteTimeout:   cfg.Timeout,
		DialTimeout:    cfg.Timeout,
		MaxRetries:     3, // Retry transient failures
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})
	
	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		if cfg.EnableFallback {
			// Log warning but don't fail - system will operate in degraded mode
			logger.Warn("Redis unavailable, operating in degraded mode",
				zap.String("host", cfg.Host),
				zap.Int("port", cfg.Port),
				zap.Error(err))
			return &RedisClient{Client: client, available: 0}, nil
		}
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	return &RedisClient{Client: client, available: 1}, nil
}

// IsAvailable returns whether Redis is currently available
func (r *RedisClient) IsAvailable() bool {
	return atomic.LoadInt32(&r.available) == 1
}

// SetAvailability sets Redis availability status (called by health checker)
func (r *RedisClient) SetAvailability(available bool) {
	if available {
		atomic.StoreInt32(&r.available, 1)
	} else {
		atomic.StoreInt32(&r.available, 0)
	}
}

func (r *RedisClient) Close() {
	r.Client.Close()
}
```

---

## SERVICE-SPECIFIC DEGRADED MODE IMPLEMENTATIONS

### API Gateway - Rate Limiter Fallback

**File:** `secureconnect-backend/internal/middleware/ratelimit.go`

```go
// Add in-memory fallback rate limiter
type InMemoryRateLimiter struct {
    counters map[string]int64
    mu       sync.RWMutex
    window   time.Duration
    maxRequests int
}

func NewInMemoryRateLimiter(window time.Duration, maxRequests int) *InMemoryRateLimiter {
    return &InMemoryRateLimiter{
        counters: make(map[string]int64),
        mu:       sync.RWMutex{},
        window:   window,
        maxRequests: maxRequests,
    }
}

func (rl *InMemoryRateLimiter) Allow(ip, endpoint string) bool {
    key := fmt.Sprintf("%s:%s", ip, endpoint)
    
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    
    // Clean old entries
    for k, v := range rl.counters {
        if now.Sub(v) > rl.window {
            delete(rl.counters, k)
        }
    }
    
    // Check and increment
    count := rl.counters[key]
    if count >= int64(rl.maxRequests) {
        return false // Rate limited
    }
    
    rl.counters[key] = count + 1
    return true
}

// Update AdvancedRateLimiter with Redis fallback
func (rl *AdvancedRateLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Check if Redis is available
        if !rl.redisDB.IsAvailable() {
            // Degraded mode: use in-memory fallback
            logger.Debug("Using in-memory rate limiter (Redis unavailable)",
                zap.String("ip", c.ClientIP()),
                zap.String("path", c.Request.URL.Path))
            
            // Use in-memory limiter (create one-time or per-request)
            inMemoryLimiter := NewInMemoryRateLimiter(1*time.Minute, 100)
            
            if !inMemoryLimiter.Allow(c.ClientIP(), c.FullPath()) {
                c.JSON(http.StatusTooManyRequests, gin.H{
                    "error": "Rate limit exceeded",
                    "mode": "degraded",
                })
                c.Abort()
                return
            }
            
            c.Next()
            return
        }
        
        // Normal Redis-based rate limiting
        // ... existing code ...
    }
}
```

### Auth Service - Stateless Login in Degraded Mode

**File:** `secureconnect-backend/internal/service/auth/service.go`

```go
// Add environment variable to control stateless auth
const EnableStatelessAuth = false // Set via ENV: ENABLE_STATELESS_AUTH

// Modify Login function for degraded mode
func (s *Service) Login(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
    // Check if Redis is available
    if !s.sessionRepo.IsAvailable() {
        logger.Warn("Redis unavailable, using stateless authentication (degraded mode)")
        
        // Verify credentials without session creation
        user, err := s.userRepo.GetByEmail(ctx, input.Email)
        if err != nil {
            _ = s.recordFailedLogin(ctx, input.Email, input.IP, uuid.Nil)
            return nil, fmt.Errorf("invalid credentials")
        }
        
        // Compare password
        err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
        if err != nil {
            _ = s.recordFailedLogin(ctx, input.Email, input.IP, user.UserID)
            return nil, fmt.Errorf("invalid credentials")
        }
        
        // Clear failed login attempts (can't store in Redis)
        // _ = s.clearFailedLoginAttempts(ctx, input.Email)
        
        // Generate short-lived token for degraded mode
        accessToken, err := s.jwtManager.GenerateAccessToken(user.UserID, user.Email, user.Username, "user")
        if err != nil {
            return nil, fmt.Errorf("failed to generate access token: %w", err)
        }
        
        // Return without refresh token (stateless)
        return &LoginOutput{
            User:        user.ToResponse(),
            AccessToken:  accessToken,
            RefreshToken: "", // No refresh token in degraded mode
        }, nil
}
```

### Chat Service - Presence Fallback

**File:** `secureconnect-backend/internal/service/chat/service.go`

```go
// Modify presence updates for degraded mode
func (s *Service) UpdatePresence(ctx context.Context, userID uuid.UUID, online bool) error {
    if !s.presenceRepo.IsAvailable() {
        logger.Debug("Presence update skipped (Redis unavailable - degraded mode)",
            zap.String("user_id", userID.String()),
            zap.Bool("online", online))
        return nil // Silently skip
    }
    
    // Normal Redis-based presence update
    if online {
        return s.presenceRepo.SetUserOnline(ctx, userID)
    } else {
        return s.presenceRepo.SetUserOffline(ctx, userID)
    }
}
```

### Video Service - Signaling Fallback

**File:** `secureconnect-backend/internal/handler/ws/signaling_handler.go`

```go
// Modify SignalingHub to detect Redis availability
type SignalingHub struct {
    // ... existing fields ...
    redisAvailable bool // Track Redis availability
}

// Add method to check Redis availability
func (h *SignalingHub) checkRedisAvailability() {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()
    
    if err := h.redisClient.Ping(ctx).Err(); err != nil {
        h.redisAvailable = false
        logger.Warn("Redis unavailable, signaling in local-only mode (degraded)")
    } else {
        h.redisAvailable = true
    }
}

// Modify broadcast to log degraded mode
func (h *SignalingHub) run() {
    for {
        select {
        case message := <-h.broadcast:
            if !h.redisAvailable {
                logger.Debug("Signaling broadcast skipped (Redis unavailable - degraded mode)",
                    zap.String("call_id", message.CallID.String()),
                    zap.String("type", message.Type))
                continue
            }
            // ... existing broadcast code ...
        }
    }
}
```

---

## METRICS FOR DEGRADED MODE

Add to `pkg/metrics/prometheus.go`:

```go
// Add degraded mode metrics
var (
    redisDegradedMode = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name:        "redis_degraded_mode",
            Help:        "Whether Redis is in degraded mode (1 = degraded, 0 = normal)",
            ConstLabels: prometheus.Labels{"service": "redis"},
        },
        []string{"service"},
    )
)

// SetDegradedMode sets the degraded mode metric
func (m *Metrics) SetDegradedMode(service string, degraded bool) {
    if degraded {
        redisDegradedMode.WithLabelValues(service).Set(1)
    } else {
        redisDegradedMode.WithLabelValues(service).Set(0)
    }
}
```

Update each service to set the metric:

```go
// In main.go of each service
appMetrics.SetDegradedMode("api-gateway", !redisDB.IsAvailable())
appMetrics.SetDegradedMode("auth-service", !redisDB.IsAvailable())
appMetrics.SetDegradedMode("chat-service", !redisDB.IsAvailable())
appMetrics.SetDegradedMode("video-service", !redisDB.IsAvailable())
```

---

## HEALTH CHECKER FOR RECOVERY

**File:** `secureconnect-backend/pkg/health/redis_health.go` (new file)

```go
package health

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"

    "secureconnect-backend/internal/database"
    "secureconnect-backend/pkg/logger"
)

// RedisHealthChecker monitors Redis and updates availability
type RedisHealthChecker struct {
    redisClient *database.RedisClient
    interval    time.Duration
    services   []string // Services to notify
}

func NewRedisHealthChecker(redisClient *database.RedisClient, interval time.Duration, services []string) *RedisHealthChecker {
    return &RedisHealthChecker{
        redisClient: redisClient,
        interval:    interval,
        services:   services,
    }
}

// Start begins the health check loop
func (h *RedisHealthChecker) Start(ctx context.Context) {
    ticker := time.NewTicker(h.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Check Redis health
            checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            err := h.redisClient.Client.Ping(checkCtx).Err()
            cancel()
            
            wasAvailable := h.redisClient.IsAvailable()
            isAvailable := err == nil
            
            if isAvailable && !wasAvailable {
                // Redis recovered
                logger.Info("Redis recovered from degraded mode")
                h.redisClient.SetAvailability(true)
            } else if !isAvailable && wasAvailable {
                // Redis failed
                logger.Warn("Redis entered degraded mode")
                h.redisClient.SetAvailability(false)
            }
        }
    }
}
```

Start health checker in each service's main.go:

```go
// In cmd/api-gateway/main.go
redisHealthChecker := health.NewRedisHealthChecker(redisDB, 10*time.Second, []string{"api-gateway"})
go redisHealthChecker.Start(context.Background())

// In cmd/auth-service/main.go
redisHealthChecker := health.NewRedisHealthChecker(redisDB, 10*time.Second, []string{"auth-service"})
go redisHealthChecker.Start(context.Background())

// In cmd/chat-service/main.go
redisHealthChecker := health.NewRedisHealthChecker(redisDB, 10*time.Second, []string{"chat-service"})
go redisHealthChecker.Start(context.Background())

// In cmd/video-service/main.go
redisHealthChecker := health.NewRedisHealthChecker(redisDB, 10*time.Second, []string{"video-service"})
go redisHealthChecker.Start(context.Background())
```

---

## DEGRADED MODE RESPONSES

Add consistent error responses for degraded mode:

```go
// In error handling
const (
    ErrDegradedMode = "SERVICE_DEGRADED"
    ErrRedisUnavailable = "redis_unavailable"
)

// Response format for degraded mode
type DegradedResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Mode    string `json:"mode"`
    Message string `json:"message,omitempty"`
}

func NewDegradedResponse(operation string) DegradedResponse {
    return DegradedResponse{
        Error:   ErrDegradedMode,
        Code:    ErrRedisUnavailable,
        Mode:    "degraded",
        Message: fmt.Sprintf("%s is operating in degraded mode due to Redis unavailability", operation),
    }
}
```

---

## VALIDATION COMMANDS

```bash
# Test degraded mode with Redis down
docker-compose stop redis

# Test API Gateway - should still work with degraded rate limiting
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test"}'

# Test Auth Service - should still work with stateless auth
curl -X POST http://localhost:8081/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test"}'

# Test Chat Service - should still work for messages
curl -X GET http://localhost:8082/v1/messages \
  -H "Authorization: Bearer <token>"

# Test Video Service - should still work for call initiation
curl -X POST http://localhost:8083/v1/calls/initiate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"call_type":"video","conversation_id":"<id>"}'

# Check degraded mode metric
curl http://localhost:9091/api/v1/query?query=redis_degraded_mode

# Expected: Should see redis_degraded_mode{service="..."}=1

# Verify logs show degraded mode
docker logs api-gateway | grep "degraded mode"
docker logs auth-service | grep "degraded mode"
docker logs chat-service | grep "degraded mode"
docker logs video-service | grep "degraded mode"

# Restart Redis
docker-compose start redis

# Verify recovery
# Expected: redis_degraded_mode metric goes to 0
# Logs show "Redis recovered from degraded mode"
```

---

## SUMMARY TABLE

| Service | Redis Usage | Fail-Closed | Fail-Open | Degraded Strategy |
|----------|-------------|-------------|-----------------|
| API Gateway | Rate limiting | All | HTTP routing | In-memory rate limiter |
| Auth Service | Sessions, tokens, lockout | Login, logout | Stateless auth (short tokens) |
| Chat Service | Presence, pub/sub | Directory | Message sending | Skip presence, in-memory hub |
| Video Service | Signaling, push tokens | Call metadata | In-memory signaling, skip push |
| Storage Service | None | None | None | No changes needed |

---

## IMPLEMENTATION PRIORITY

1. **HIGH PRIORITY** - Redis client with availability tracking
   - Files: `internal/database/redis.go`, `pkg/metrics/prometheus.go`
   - Effort: 2 hours

2. **HIGH PRIORITY** - API Gateway in-memory rate limiter
   - File: `internal/middleware/ratelimit.go`
   - Effort: 1 hour

3. **MEDIUM PRIORITY** - Auth service stateless login
   - File: `internal/service/auth/service.go`
   - Effort: 1 hour

4. **MEDIUM PRIORITY** - Chat service presence skip
   - File: `internal/service/chat/service.go`
   - Effort: 30 minutes

5. **MEDIUM PRIORITY** - Video service local signaling
   - File: `internal/handler/ws/signaling_handler.go`
   - Effort: 30 minutes

6. **LOW PRIORITY** - Redis health checker
   - File: `pkg/health/redis_health.go` (new)
   - Effort: 1 hour

**Total Estimated Effort:** 6 hours

---

## ENVIRONMENT VARIABLES

Add to `.env.production.example`:

```bash
# Enable Redis degraded mode fallback
ENABLE_REDIS_FALLBACK=true

# Enable stateless authentication in degraded mode
ENABLE_STATELESS_AUTH=false

# Redis health check interval (seconds)
REDIS_HEALTH_CHECK_INTERVAL=10
```
