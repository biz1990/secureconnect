# CRITICAL GAPS ANALYSIS - MINIMAL FIXES

This document provides detailed root cause analysis and minimal fixes for confirmed critical and high gaps from the system-level validation report.

---

## Gap 1: Missing /metrics endpoint in API Gateway

**Root Cause:**
The API Gateway initializes metrics with `metrics.NewMetrics("api-gateway")` and applies the Prometheus middleware, but never exposes the `/metrics` HTTP endpoint. The Prometheus configuration in [`configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:21-24) expects `api-gateway:8080/metrics` to exist, but this route is not registered in the router at [`cmd/api-gateway/main.go`](secureconnect-backend/cmd/api-gateway/main.go:91-98).

**Why Docker running is not enough:**
Even with all containers running successfully, Prometheus will receive HTTP 404 errors when attempting to scrape metrics from the API Gateway. The scrape target is configured, but the endpoint is missing, resulting in failed metrics collection and blind spots in observability.

**Fix:**

File: `secureconnect-backend/cmd/api-gateway/main.go`

```diff
<<<<<<< SEARCH
:start_line:91
-------
	// 6. Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "api-gateway",
			"timestamp": time.Now().UTC(),
		})
	})

	// 9. Swagger documentation
=======
	// 6. Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "api-gateway",
			"timestamp": time.Now().UTC(),
		})
	})

	// 7. Metrics endpoint (for Prometheus scraping)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// 8. Swagger documentation
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# Start the system and verify the endpoint exists
curl -f http://localhost:8080/metrics
# Expected: Prometheus metrics output (not 404)
# Verify in Prometheus UI: http://localhost:9091/targets
```

---

## Gap 2: Missing /metrics endpoint in Auth Service

**Root Cause:**
The Auth Service initializes metrics with `metrics.NewMetrics("auth-service")` and applies the Prometheus middleware, but never exposes the `/metrics` HTTP endpoint. The Prometheus configuration in [`configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:28-32) expects `auth-service:8081/metrics` to exist, but this route is not registered in the router at [`cmd/auth-service/main.go`](secureconnect-backend/cmd/auth-service/main.go:176-183).

**Why Docker running is not enough:**
Even with all containers running successfully, Prometheus will receive HTTP 404 errors when attempting to scrape metrics from the Auth Service. The scrape target is configured, but the endpoint is missing, resulting in failed metrics collection.

**Fix:**

File: `secureconnect-backend/cmd/auth-service/main.go`

```diff
<<<<<<< SEARCH
:start_line:176
-------
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "auth-service",
			"time":    time.Now().UTC(),
		})
	})

	// API version 1 routes
=======
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "auth-service",
			"time":    time.Now().UTC(),
		})
	})

	// Metrics endpoint (for Prometheus scraping)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// API version 1 routes
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# Start the system and verify the endpoint exists
curl -f http://localhost:8081/metrics
# Expected: Prometheus metrics output (not 404)
# Verify in Prometheus UI: http://localhost:9091/targets
```

---

## Gap 3: Promtail log volume mismatch (Loki ingest failure)

**Root Cause:**
There is a configuration mismatch between where Promtail looks for logs and where services write them:
- [`docker-compose.monitoring.yml`](secureconnect-backend/docker-compose.monitoring.yml:100) mounts `./logs:/var/log/secureconnect:ro` for Promtail
- [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:202) mounts `app_logs:/logs` volume for services

Services write logs to stdout (captured by Docker), but Promtail is configured to read from the `./logs` directory on the host, which may not contain Docker-captured logs.

**Why Docker running is not enough:**
Even if all containers run successfully, Loki will not receive application logs because Promtail is watching the wrong directory. The logs exist in Docker's stdout/stderr but are not being collected by Promtail, resulting in empty log aggregation.

**Fix:**

**Option A: Fix Promtail volume mount (RECOMMENDED)**

File: `secureconnect-backend/docker-compose.monitoring.yml`

```diff
<<<<<<< SEARCH
:start_line:95
-------
  # 4. PROMTAIL - Log Collector (Optional)
  promtail:
    image: grafana/promtail:2.9.2
    container_name: secureconnect_promtail
    volumes:
      - ./configs/promtail-config.yml:/etc/promtail/config.yml:ro
      - ./logs:/var/log/secureconnect:ro
=======
  # 4. PROMTAIL - Log Collector (Optional)
  promtail:
    image: grafana/promtail:2.9.2
    container_name: secureconnect_promtail
    volumes:
      - ./configs/promtail-config.yml:/etc/promtail/config.yml:ro
      - app_logs:/var/log/secureconnect:ro
>>>>>>> REPLACE
```

**Option B: Configure Promtail to read from Docker socket (ALTERNATIVE)**

File: `secureconnect-backend/configs/promtail-config.yml`

Change all scrape_configs to use Docker's socket instead of file paths:

```yaml
scrape_configs:
  - job_name: api-gateway
    docker_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        labels:
          job: api-gateway
          service: api-gateway
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout
```

**Validation Command:**
```bash
# After starting the system, check if logs appear in Loki
# Query Loki via Grafana Explore
# Filter: {job="api-gateway"} or {job="auth-service"}
# Expected: Should show recent log entries
```

---

## Gap 4: Redis failure causes full system outage

**Root Cause:**
Services (API Gateway, Auth, Chat, Video) depend on Redis for critical operations but have no fallback mechanism:
- API Gateway: Rate limiting only
- Auth Service: Sessions, directory, presence
- Chat Service: Presence, pub/sub
- Video Service: Signaling pub/sub

When Redis is unavailable, these operations fail immediately with no degraded mode or local fallback. The Redis client in [`internal/database/redis.go`](secureconnect-backend/internal/database/redis.go:31-43) uses standard `redis.NewClient()` without any retry or fallback configuration.

**Why Docker running is not enough:**
A running Redis container doesn't guarantee availability. Network issues, resource exhaustion, or Redis process crashes can cause Redis to become temporarily unavailable. Without fallback, the entire system becomes non-functional.

**Fix:**

File: `secureconnect-backend/internal/database/redis.go`

```diff
<<<<<<< SEARCH
:start_line:1
-------
package database

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
	Timeout  time.Duration
}

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(addr string) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisClient{Client: client}
}

// NewRedisDB creates a new Redis client from config
func NewRedisDB(cfg *RedisConfig) (*RedisClient, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		DialTimeout:  cfg.Timeout,
	})
	return &RedisClient{Client: client}, nil
}

func (r *RedisClient) Close() {
	r.Client.Close()
}
=======
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

// SetAvailability sets Redis availability status
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
>>>>>>> REPLACE
```

**Usage Example in Rate Limiter:**

File: `secureconnect-backend/internal/middleware/ratelimit.go`

```diff
<<<<<<< SEARCH
:start_line:1
-------
// Middleware returns a rate limiting middleware
func (rl *AdvancedRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
=======
// Middleware returns a rate limiting middleware with Redis fallback
func (rl *AdvancedRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if Redis is available
		if !rl.redisDB.IsAvailable() {
			// Degraded mode: allow request without rate limiting
			logger.Debug("Redis unavailable, bypassing rate limit",
				zap.String("ip", c.ClientIP()),
				zap.String("path", c.Request.URL.Path))
			c.Next()
			return
		}
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# Test Redis failure handling
docker-compose stop redis
# Make API requests - should continue in degraded mode
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test"}'
# Expected: Request succeeds (degraded mode)
docker-compose start redis
# Verify system recovers automatically
```

---

## Gap 5: Video service restart drops active calls

**Root Cause:**
Video call state is stored in memory within the SignalingHub at [`internal/handler/ws/signaling_handler.go`](secureconnect-backend/internal/handler/ws/signaling_handler.go:24-46) and not persisted to Redis. When the video-service container restarts, all in-memory state is lost, causing active calls to be terminated without recovery.

The video service initializes calls in CockroachDB for persistence, but the real-time signaling state (WebSocket connections, active participants) exists only in the hub's memory map.

**Why Docker running is not enough:**
A running video-service container can be restarted by Docker due to:
- Container crash
- Deployment update
- Resource limits
- Manual restart

When this happens, all active WebSocket connections are dropped and participants are not notified. Call metadata exists in the database but there's no mechanism to re-establish connections.

**Fix:**

File: `secureconnect-backend/internal/handler/ws/signaling_handler.go`

```diff
<<<<<<< SEARCH
:start_line:23
-------
// SignalingHub manages WebRTC signaling connections
type SignalingHub struct {
	// Registered clients per call
	calls map[uuid.UUID]map[*SignalingClient]bool

	// Cancel functions for call subscriptions
	subscriptionCancels map[uuid.UUID]context.CancelFunc

	// Redis client for Pub/Sub
	redisClient *redis.Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Channels
	register   chan *SignalingClient
	unregister chan *SignalingClient
	broadcast  chan *SignalingMessage

	// Concurrency limit: maxConnections is maximum number of concurrent WebSocket connections
	maxConnections int
	// Semaphore for limiting concurrent connections
	semaphore chan struct{}
}
=======
// SignalingHub manages WebRTC signaling connections
type SignalingHub struct {
	// Registered clients per call
	calls map[uuid.UUID]map[*SignalingClient]bool

	// Cancel functions for call subscriptions
	subscriptionCancels map[uuid.UUID]context.CancelFunc

	// Redis client for Pub/Sub and call state persistence
	redisClient *redis.Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Channels
	register   chan *SignalingClient
	unregister chan *SignalingClient
	broadcast  chan *SignalingMessage

	// Concurrency limit: maxConnections is maximum number of concurrent WebSocket connections
	maxConnections int
	// Semaphore for limiting concurrent connections
	semaphore chan struct{}
}

// CallState represents persisted call state for recovery
type CallState struct {
	CallID      uuid.UUID
	ConversationID uuid.UUID
	ParticipantIDs []string
	CreatedAt    time.Time
	Status       string
}
>>>>>>> REPLACE
```

Add persistence and recovery methods to SignalingHub:

```diff
<<<<<<< SEARCH
:start_line:128
-------
// NewSignalingHub creates a new signaling hub
func NewSignalingHub(redisClient *redis.Client) *SignalingHub {
=======
// NewSignalingHub creates a new signaling hub
func NewSignalingHub(redisClient *redis.Client) *SignalingHub {
	// Start recovery goroutine on startup
	go h.recoverActiveCalls()
>>>>>>> REPLACE
```

Add the recovery method:

```go
// recoverActiveCalls attempts to recover call state from Redis on startup
func (h *SignalingHub) recoverActiveCalls() {
	ctx := context.Background()
	
	// Get all active call keys from Redis
	keys, err := h.redisClient.Keys(ctx, "call:*").Result()
	if err != nil {
		logger.Error("Failed to retrieve active calls from Redis", zap.Error(err))
		return
	}
	
	if len(keys) == 0 {
		return
	}
	
	logger.Info("Recovering active calls from Redis", zap.Int("count", len(keys)))
	
	// For each call, restore state
	for _, key := range keys {
		// Get call state
		val, err := h.redisClient.Get(ctx, key).Result()
		if err != nil {
			logger.Warn("Failed to retrieve call state",
				zap.String("key", key),
				zap.Error(err))
			continue
		}
		
		var callState CallState
		if err := json.Unmarshal([]byte(val), &callState); err != nil {
			logger.Warn("Failed to unmarshal call state",
				zap.String("key", key),
				zap.Error(err))
			continue
		}
		
		// Restore call state in memory
		h.mu.Lock()
		if h.calls[callState.CallID] == nil {
			h.calls[callState.CallID] = make(map[*SignalingClient]bool)
		}
		h.mu.Unlock()
		
		logger.Info("Recovered call state",
			zap.String("call_id", callState.CallID.String()),
			zap.Int("participants", len(callState.ParticipantIDs)))
	}
}

// persistCallState saves call state to Redis for recovery
func (h *SignalingHub) persistCallState(callID uuid.UUID, conversationID uuid.UUID, participantIDs []string) error {
	ctx := context.Background()
	key := fmt.Sprintf("call:%s", callID)
	
	callState := CallState{
		CallID:        callID,
		ConversationID: conversationID,
		ParticipantIDs: participantIDs,
		CreatedAt:     time.Now(),
		Status:         "active",
	}
	
	data, err := json.Marshal(callState)
	if err != nil {
		return fmt.Errorf("failed to marshal call state: %w", err)
	}
	
	// Persist for 24 hours
	return h.redisClient.Set(ctx, key, data, 24*time.Hour).Err()
}

// clearCallState removes call state from Redis
func (h *SignalingHub) clearCallState(callID uuid.UUID) error {
	ctx := context.Background()
	key := fmt.Sprintf("call:%s", callID)
	return h.redisClient.Del(ctx, key).Err()
}
```

Update the register/unregister logic to persist state:

```diff
<<<<<<< SEARCH
:start_line:133
-------
		case client := <-h.register:
			h.mu.Lock()
			if h.calls[client.callID] == nil {
				h.calls[client.callID] = make(map[*SignalingClient]bool)

				// Create cancelable context for subscription
				ctx, cancel := context.WithCancel(context.Background())
				h.subscriptionCancels[client.callID] = cancel

				// Subscribe to Redis channel for this call
				go h.subscribeToCall(ctx, client.callID)
			}
			h.calls[client.callID][client] = true
			h.mu.Unlock()
=======
		case client := <-h.register:
			h.mu.Lock()
			if h.calls[client.callID] == nil {
				h.calls[client.callID] = make(map[*SignalingClient]bool)

				// Create cancelable context for subscription
				ctx, cancel := context.WithCancel(context.Background())
				h.subscriptionCancels[client.callID] = cancel

				// Subscribe to Redis channel for this call
				go h.subscribeToCall(ctx, client.callID)
				
				// Persist call state for recovery
				participantIDs := make([]string, 0, len(h.calls[client.callID]))
				for c := range h.calls[client.callID] {
					participantIDs = append(participantIDs, c.userID.String())
				}
				if err := h.persistCallState(client.callID, client.callID, participantIDs); err != nil {
					logger.Error("Failed to persist call state",
						zap.String("call_id", client.callID.String()),
						zap.Error(err))
				}
			}
			h.calls[client.callID][client] = true
			h.mu.Unlock()
>>>>>>> REPLACE
```

```diff
<<<<<<< SEARCH
:start_line:172
-------
			// Clean up empty calls
			if len(clients) == 0 {
				// Cancel Redis subscription
				if cancel, ok := h.subscriptionCancels[client.callID]; ok {
					cancel()
					delete(h.subscriptionCancels, client.callID)
				}
				delete(h.calls, client.callID)
			}
=======
			// Clean up empty calls
			if len(clients) == 0 {
				// Cancel Redis subscription
				if cancel, ok := h.subscriptionCancels[client.callID]; ok {
					cancel()
					delete(h.subscriptionCancels, client.callID)
				}
				
				// Clear persisted call state
				if err := h.clearCallState(client.callID); err != nil {
					logger.Error("Failed to clear call state",
						zap.String("call_id", client.callID.String()),
						zap.Error(err))
				}
				
				delete(h.calls, client.callID)
			}
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# Test call recovery
# 1. Start an active call
# 2. Persist call state to Redis (verify with: redis-cli GET "call:<call_id>")
# 3. Restart video-service: docker-compose restart video-service
# 4. Verify call state is recovered from Redis (check logs)
# Expected: Logs show "Recovering active calls from Redis"
```

---

## Gap 6: No Prometheus alerting rules

**Root Cause:**
Prometheus is configured in [`configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:1-58) but the `rule_files` section at line 15-16 is commented out:
```yaml
# Rule files (optional)
rule_files:
  # - "alerts.yml"
```

The Alertmanager is configured but no alert rules are defined, meaning no proactive failure detection or notifications will occur.

**Why Docker running is not enough:**
Even with Prometheus, Alertmanager, and Grafana running, no alerts will be triggered because no rules exist. Operators will not be notified of:
- Service health check failures
- High error rates
- Redis unavailability
- Database connection pool exhaustion
- Metrics scrape failures

**Fix:**

File: `secureconnect-backend/configs/prometheus.yml`

```diff
<<<<<<< SEARCH
:start_line:14
-------
# Rule files (optional)
rule_files:
  # - "alerts.yml"
=======
# Rule files (optional)
rule_files:
  - "/etc/prometheus/alerts.yml"
>>>>>>> REPLACE
```

File: `secureconnect-backend/configs/alerts.yml` (create if not exists)

```yaml
groups:
  - name: secureconnect_alerts
    interval: 30s
    rules:
      # Service Health Alerts
      - alert: ServiceDown
        expr: up{job=~"api-gateway|auth-service|chat-service|video-service|storage-service"} == 0
        for: 1m
        labels:
          severity: critical
          component: service
        annotations:
          summary: "Service {{ $labels.job }} is down"
          description: "Service {{ $labels.job }} has been down for more than 1 minute"

      # High Error Rate Alert
      - alert: HighErrorRate
        expr: |
          (
            rate(http_requests_total{status=~"5.."}[5m]) 
            / 
            rate(http_requests_total[5m])
          ) > 0.05
        for: 5m
        labels:
          severity: warning
          component: http
        annotations:
          summary: "High error rate detected on {{ $labels.job }}"
          description: "Error rate is {{ $value | humanizePercentage }} for {{ $labels.job }}"

      # Database Connection Pool Alert
      - alert: DBPoolExhaustion
        expr: |
          (
            db_connections_active{job=~"auth-service|chat-service|video-service|storage-service"} 
            / 
            db_connections_active{job=~"auth-service|chat-service|video-service|storage-service"} + db_connections_idle{job=~"auth-service|chat-service|video-service|storage-service"}
          ) > 0.8
        for: 2m
        labels:
          severity: warning
          component: database
        annotations:
          summary: "Database connection pool exhausted on {{ $labels.job }}"
          description: "Pool usage is {{ $value | humanizePercentage }}"

      # Redis Unavailable Alert
      - alert: RedisDown
        expr: redis_up == 0
        for: 1m
        labels:
          severity: critical
          component: redis
        annotations:
          summary: "Redis is down"
          description: "Redis has been down for more than 1 minute"

      # Metrics Scrape Failure Alert
      - alert: ScrapeFailed
        expr: up{job=~"api-gateway|auth-service|chat-service|video-service|storage-service"} == 0
        for: 2m
        labels:
          severity: warning
          component: monitoring
        annotations:
          summary: "Metrics scrape failed for {{ $labels.job }}"
          description: "Unable to scrape metrics from {{ $labels.job }}"

      # No Active Connections Alert
      - alert: NoActiveConnections
        expr: |
          (
            sum(rate(http_requests_total[5m])) by (job) == 0 
            and 
            up{job=~"api-gateway|auth-service|chat-service|video-service|storage-service"} == 1
          )
        for: 10m
        labels:
          severity: info
          component: traffic
        annotations:
          summary: "No traffic detected on {{ $labels.job }}"
          description: "No requests received in the last 10 minutes"

      # High Response Time Alert
      - alert: HighResponseTime
        expr: |
          histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
          component: performance
        annotations:
          summary: "High P95 latency on {{ $labels.job }}"
          description: "P95 response time is {{ $value }}s"

      # WebSocket Connection Alert
      - alert: WebSocketConnectionsHigh
        expr: websocket_connections > 1000
        for: 5m
        labels:
          severity: warning
          component: websocket
        annotations:
          summary: "High WebSocket connection count"
          description: "{{ $value }} active WebSocket connections"

      # Active Calls Alert
      - alert: ActiveCallsHigh
        expr: calls_active > 100
        for: 5m
        labels:
          severity: warning
          component: video
        annotations:
          summary: "High number of active calls"
          description: "{{ $value }} active calls detected"
```

Update Alertmanager to mount alerts file:

File: `secureconnect-backend/docker-compose.monitoring.yml`

```diff
<<<<<<< SEARCH
:start_line:428
-------
  # ALERTMANAGER (Prometheus Alerting)
  alertmanager:
    image: prom/alertmanager:v0.26.0
    container_name: secureconnect_alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./configs/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
=======
  # ALERTMANAGER (Prometheus Alerting)
  alertmanager:
    image: prom/alertmanager:v0.26.0
    container_name: secureconnect_alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./configs/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
      - ./configs/alerts.yml:/etc/prometheus/alerts.yml:ro
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# After starting the system, verify alerts are loaded
curl http://localhost:9091/api/v1/rules
# Expected: Returns list of loaded alert rules
# Test an alert by stopping a service
docker-compose stop auth-service
# Expected: Alert fires in Alertmanager UI at http://localhost:9093
```

---

## Gap 7: Permission denial audit logs incomplete

**Root Cause:**
Permission checks are performed in various services but are not consistently logged for audit trail. While authentication attempts are comprehensively logged, authorization failures (permission denials) return error responses without explicit audit logging.

**Why Docker running is not enough:**
Even with the system running, audit trails for security events (permission denials, unauthorized access attempts) are incomplete. This makes compliance verification and security incident investigation difficult.

**Fix:**

Create a new middleware for audit logging:

File: `secureconnect-backend/internal/middleware/audit.go` (new file)

```go
package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	UserID      uuid.UUID `json:"user_id,omitempty"`
	Resource     string    `json:"resource"`
	Action       string    `json:"action"`
	Result       string    `json:"result"`
	Reason       string    `json:"reason,omitempty"`
	ClientIP     string    `json:"client_ip,omitempty"`
	RequestID    string    `json:"request_id,omitempty"`
}

// AuditMiddleware logs authorization events for security audit
func AuditMiddleware(resource string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, _ := c.Get("user_id")
		
		// Get request ID
		requestID, _ := c.Get("request_id")
		
		// Process request
		c.Next()
		
		// Check if authorization was denied
		if c.Writer.Status() == http.StatusForbidden {
			event := AuditEvent{
				EventID:   uuid.New().String(),
				Timestamp: time.Now(),
				UserID:    userID.(uuid.UUID),
				Resource:  resource,
				Action:    action,
				Result:    "denied",
				Reason:    "insufficient_permissions",
				ClientIP:  c.ClientIP(),
				RequestID: requestID.(string),
			}
			
			logger.Warn("Permission denied - audit event",
				zap.String("event_id", event.EventID),
				zap.String("user_id", event.UserID.String()),
				zap.String("resource", event.Resource),
				zap.String("action", event.Action),
				zap.String("client_ip", event.ClientIP),
				zap.String("request_id", event.RequestID))
		}
	}
}
```

Apply to conversation routes:

File: `secureconnect-backend/cmd/auth-service/main.go`

```diff
<<<<<<< SEARCH
:start_line:231
-------
		// Conversation Management routes (all require authentication)
		conversations := v1.Group("/conversations")
		conversations.Use(middleware.AuthMiddleware(jwtManager, authSvc))
		{
=======
		// Conversation Management routes (all require authentication)
		conversations := v1.Group("/conversations")
		conversations.Use(middleware.AuthMiddleware(jwtManager, authSvc))
		conversations.Use(middleware.AuditMiddleware("conversation", "access"))
		{
>>>>>>> REPLACE
```

Apply to file routes:

File: `secureconnect-backend/cmd/storage-service/main.go`

```diff
<<<<<<< SEARCH
:start_line:155
-------
		// Storage routes (all require authentication)
		v1 := router.Group("/v1/storage")
		v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
=======
		// Storage routes (all require authentication)
		v1 := router.Group("/v1/storage")
		v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		v1.Use(middleware.AuditMiddleware("storage", "access"))
		{
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# Test permission denial logging
# 1. Login as user
# 2. Attempt to access another user's file
curl -X GET http://localhost:8084/v1/storage/download-url/<other_user_file_id> \
  -H "Authorization: Bearer <token>"
# 3. Check logs for audit event
docker logs storage-service | grep "Permission denied"
# Expected: Audit log entry with event_id, user_id, resource, action, result=denied
```

---

## Gap 8: File access audit logs missing

**Root Cause:**
File operations (upload, download, delete) in the storage service at [`internal/service/storage/service.go`](secureconnect-backend/internal/service/storage/service.go:108-213) do not generate explicit audit logs. While the service performs ownership verification, these access events are not logged for audit trail.

**Why Docker running is not enough:**
Even with the system running, there is no record of file access events for compliance and security auditing. This makes it impossible to track:
- Who accessed which files
- When files were downloaded
- When files were deleted
- Suspicious access patterns

**Fix:**

File: `secureconnect-backend/internal/service/storage/service.go`

```diff
<<<<<<< SEARCH
:start_line:107
-------
// GenerateUploadURL creates presigned URL for file upload
func (s *Service) GenerateUploadURL(ctx context.Context, userID uuid.UUID, input *GenerateUploadURLInput) (*GenerateUploadURLOutput, error) {
	// Check storage quota before allowing upload
	used, quota, err := s.GetUserQuota(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check storage quota: %w", err)
	}
=======
// GenerateUploadURL creates presigned URL for file upload
func (s *Service) GenerateUploadURL(ctx context.Context, userID uuid.UUID, input *GenerateUploadURLInput) (*GenerateUploadURLOutput, error) {
	// Audit log: Upload URL requested
	logger.Info("File upload URL generated",
		zap.String("event_id", uuid.New().String()),
		zap.String("user_id", userID.String()),
		zap.String("file_name", input.FileName),
		zap.Int64("file_size", input.FileSize),
		zap.String("content_type", input.ContentType),
		zap.Bool("encrypted", input.IsEncrypted))
	
	// Check storage quota before allowing upload
	used, quota, err := s.GetUserQuota(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check storage quota: %w", err)
	}
>>>>>>> REPLACE
```

```diff
<<<<<<< SEARCH
:start_line:165
-------
// GenerateDownloadURL creates presigned URL for file download
func (s *Service) GenerateDownloadURL(ctx context.Context, userID, fileID uuid.UUID) (string, error) {
	// Fetch file metadata from CockroachDB
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// Verify user owns file
	if file.UserID != userID {
		return "", fmt.Errorf("unauthorized access to file")
	}
=======
// GenerateDownloadURL creates presigned URL for file download
func (s *Service) GenerateDownloadURL(ctx context.Context, userID, fileID uuid.UUID) (string, error) {
	// Fetch file metadata from CockroachDB
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// Verify user owns file
	if file.UserID != userID {
		// Audit log: Unauthorized access attempt
		logger.Warn("Unauthorized file access attempt",
			zap.String("event_id", uuid.New().String()),
			zap.String("user_id", userID.String()),
			zap.String("file_id", fileID.String()),
			zap.String("file_owner_id", file.UserID.String()))
		return "", fmt.Errorf("unauthorized access to file")
	}
	
	// Audit log: Download authorized
	logger.Info("File download authorized",
		zap.String("event_id", uuid.New().String()),
		zap.String("user_id", userID.String()),
		zap.String("file_id", fileID.String()),
		zap.String("file_name", file.FileName),
		zap.Int64("file_size", file.FileSize))
>>>>>>> REPLACE
```

```diff
<<<<<<< SEARCH
:start_line:188
-------
// DeleteFile removes file from storage
func (s *Service) DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	// Get file metadata
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify ownership
	if file.UserID != userID {
		return fmt.Errorf("unauthorized")
	}
=======
// DeleteFile removes file from storage
func (s *Service) DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	// Get file metadata
	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify ownership
	if file.UserID != userID {
		// Audit log: Unauthorized delete attempt
		logger.Warn("Unauthorized file delete attempt",
			zap.String("event_id", uuid.New().String()),
			zap.String("user_id", userID.String()),
			zap.String("file_id", fileID.String()),
			zap.String("file_owner_id", file.UserID.String()))
		return fmt.Errorf("unauthorized")
	}
	
	// Audit log: Delete authorized
	logger.Info("File deletion authorized",
		zap.String("event_id", uuid.New().String()),
		zap.String("user_id", userID.String()),
		zap.String("file_id", fileID.String()),
		zap.String("file_name", file.FileName),
		zap.Int64("file_size", file.FileSize))
>>>>>>> REPLACE
```

**Validation Command:**
```bash
# Test file access logging
# 1. Login as user
# 2. Upload a file
curl -X POST http://localhost:8084/v1/storage/upload-url \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"test.txt","file_size":1024,"content_type":"text/plain"}'
# 3. Check logs for audit event
docker logs storage-service | grep "File upload URL generated"
# Expected: Audit log with event_id, user_id, file_name, file_size
```

---

## SUMMARY OF FIXES

| Gap | Files Modified | Lines Changed | Complexity |
|------|---------------|----------------|-------------|
| Missing /metrics (API Gateway) | `cmd/api-gateway/main.go` | +3 | LOW |
| Missing /metrics (Auth Service) | `cmd/auth-service/main.go` | +3 | LOW |
| Promtail volume mismatch | `docker-compose.monitoring.yml` | 1 | LOW |
| Redis fallback | `internal/database/redis.go` | +60 | MEDIUM |
| Video call recovery | `internal/handler/ws/signaling_handler.go` | +80 | MEDIUM |
| No alerting rules | `configs/alerts.yml` (new), `docker-compose.monitoring.yml` | +100 | MEDIUM |
| Permission audit logs | `internal/middleware/audit.go` (new), multiple main.go | +50 | MEDIUM |
| File access audit logs | `internal/service/storage/service.go` | +15 | LOW |

**Total Estimated Effort:** ~4-6 hours for all fixes

---

## VALIDATION CHECKLIST

After applying fixes, validate:

- [ ] API Gateway /metrics endpoint returns Prometheus format
- [ ] Auth Service /metrics endpoint returns Prometheus format
- [ ] Prometheus targets show "UP" status for all services
- [ ] Logs appear in Grafana Loki Explore for all services
- [ ] Redis failure allows degraded operation
- [ ] Video service restart recovers active calls from Redis
- [ ] Alertmanager shows loaded rules
- [ ] Permission denials appear in logs with audit metadata
- [ ] File operations appear in logs with audit metadata
