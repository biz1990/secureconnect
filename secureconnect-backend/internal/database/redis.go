package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
	Timeout  time.Duration
}

// RedisClient wraps Redis client with degraded mode support
type RedisClient struct {
	Client         *redis.Client
	degradedMode   bool
	degradedModeMu sync.RWMutex
	healthCheckMu  sync.Mutex
	metrics        *redisMetrics
}

// redisMetrics tracks Redis-related metrics
type redisMetrics struct {
	degradedMode prometheus.Gauge
	healthCheck  prometheus.Counter
}

var (
	// Global metrics instance
	redisMetricsInstance *redisMetrics
	redisMetricsOnce     sync.Once
)

// InitRedisMetrics initializes and registers Redis metrics with Prometheus
// This should be called explicitly in main() before metrics are used
func InitRedisMetrics() {
	redisMetricsOnce.Do(func() {
		redisMetricsInstance = &redisMetrics{
			degradedMode: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "redis_degraded_mode",
				Help: "Indicates if Redis is in degraded mode (1 = degraded, 0 = healthy)",
			}),
			healthCheck: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "redis_health_check_total",
				Help: "Total number of Redis health checks",
			}),
		}
		// Register metrics with Prometheus
		prometheus.MustRegister(redisMetricsInstance.degradedMode)
		prometheus.MustRegister(redisMetricsInstance.healthCheck)
	})
}

// getRedisMetrics returns the Redis metrics instance
// This function is called after InitRedisMetrics() has been called
func getRedisMetrics() *redisMetrics {
	return redisMetricsInstance
}

// NewRedisClient creates a new Redis client
// DEPRECATED: Use NewRedisDB for production use with degraded mode support
func NewRedisClient(addr string) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisClient{Client: client}
}

// NewRedisDB creates a new Redis client from config with degraded mode support
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

	metrics := getRedisMetrics()

	return &RedisClient{
		Client:  client,
		metrics: metrics,
	}, nil
}

// Close closes the Redis client connection
func (r *RedisClient) Close() {
	r.Client.Close()
}

// StartHealthCheck starts a background goroutine that periodically checks Redis health
func (r *RedisClient) StartHealthCheck(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Context cancelled, stop health check
				return
			case <-ticker.C:
				// Perform health check
				r.HealthCheck(context.Background())
			}
		}
	}()
}

// IsDegraded returns true if Redis is in degraded mode
func (r *RedisClient) IsDegraded() bool {
	r.degradedModeMu.RLock()
	defer r.degradedModeMu.RUnlock()
	return r.degradedMode
}

// setDegradedMode sets the degraded mode state and updates metrics
func (r *RedisClient) setDegradedState(degraded bool) {
	r.degradedModeMu.Lock()
	defer r.degradedModeMu.Unlock()

	if r.degradedMode != degraded {
		r.degradedMode = degraded
		if r.metrics != nil {
			metrics := getRedisMetrics()
			if r.degradedMode {
				metrics.degradedMode.Set(1)
			} else {
				metrics.degradedMode.Set(0)
			}
		}
	}
}

// HealthCheck performs a health check on Redis and updates degraded mode
// It uses a mutex to prevent concurrent health checks from overwhelming Redis
func (r *RedisClient) HealthCheck(ctx context.Context) error {
	r.healthCheckMu.Lock()
	defer r.healthCheckMu.Unlock()

	// Use a short timeout for health checks
	healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := r.Client.Ping(healthCtx).Err()
	if err != nil {
		// Redis is unavailable, enter degraded mode
		r.setDegradedState(true)
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Redis is healthy, exit degraded mode
	r.setDegradedState(false)

	// Increment health check counter
	if r.metrics != nil {
		metrics := getRedisMetrics()
		metrics.healthCheck.Inc()
	}

	return nil
}

// DegradedOperation executes a function with degraded mode handling
// If Redis is degraded, it logs a warning and returns a fallback value
func (r *RedisClient) DegradedOperation(operation string, fallback func() error) error {
	if r.IsDegraded() {
		// Redis is in degraded mode, use fallback
		return fallback()
	}
	return nil
}

// SafePing performs a ping with degraded mode handling
func (r *RedisClient) SafePing(ctx context.Context) error {
	if r.IsDegraded() {
		return fmt.Errorf("redis is in degraded mode, ping skipped")
	}
	return r.Client.Ping(ctx).Err()
}

// SafeGet performs a GET operation with degraded mode handling
func (r *RedisClient) SafeGet(ctx context.Context, key string) *redis.StringCmd {
	if r.IsDegraded() {
		return redis.NewStringResult("", fmt.Errorf("redis is in degraded mode, get skipped"))
	}
	return r.Client.Get(ctx, key)
}

// SafeSet performs a SET operation with degraded mode handling
func (r *RedisClient) SafeSet(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if r.IsDegraded() {
		return redis.NewStatusResult("", fmt.Errorf("redis is in degraded mode, set skipped"))
	}
	return r.Client.Set(ctx, key, value, expiration)
}

// SafeDel performs a DEL operation with degraded mode handling
func (r *RedisClient) SafeDel(ctx context.Context, keys ...string) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, del skipped"))
	}
	return r.Client.Del(ctx, keys...)
}

// SafeHSet performs an HSET operation with degraded mode handling
func (r *RedisClient) SafeHSet(ctx context.Context, key, field string, value interface{}) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, hset skipped"))
	}
	return r.Client.HSet(ctx, key, field, value)
}

// SafeHGet performs an HGET operation with degraded mode handling
func (r *RedisClient) SafeHGet(ctx context.Context, key, field string) *redis.StringCmd {
	if r.IsDegraded() {
		return redis.NewStringResult("", fmt.Errorf("redis is in degraded mode, hget skipped"))
	}
	return r.Client.HGet(ctx, key, field)
}

// SafeHDel performs an HDEL operation with degraded mode handling
func (r *RedisClient) SafeHDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, hdel skipped"))
	}
	return r.Client.HDel(ctx, key, fields...)
}

// SafePublish performs a PUBLISH operation with degraded mode handling
func (r *RedisClient) SafePublish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, publish skipped"))
	}
	return r.Client.Publish(ctx, channel, message)
}

// SafeSubscribe performs a SUBSCRIBE operation with degraded mode handling
func (r *RedisClient) SafeSubscribe(ctx context.Context, channels ...string) *redis.PubSub {
	if r.IsDegraded() {
		return nil // Return nil to indicate no subscription in degraded mode
	}
	return r.Client.Subscribe(ctx, channels...)
}

// SafeExpire performs an EXPIRE operation with degraded mode handling
func (r *RedisClient) SafeExpire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	if r.IsDegraded() {
		return redis.NewBoolResult(false, fmt.Errorf("redis is in degraded mode, expire skipped"))
	}
	return r.Client.Expire(ctx, key, expiration)
}

// SafeZAdd performs a ZADD operation with degraded mode handling
func (r *RedisClient) SafeZAdd(ctx context.Context, key string, member interface{}, score float64) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, zadd skipped"))
	}
	return r.Client.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
}

// SafeZRem performs a ZREM operation with degraded mode handling
func (r *RedisClient) SafeZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, zrem skipped"))
	}
	return r.Client.ZRem(ctx, key, members...)
}

// SafeZRange performs a ZRANGE operation with degraded mode handling
func (r *RedisClient) SafeZRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	if r.IsDegraded() {
		return redis.NewStringSliceResult([]string{}, fmt.Errorf("redis is in degraded mode, zrange skipped"))
	}
	return r.Client.ZRange(ctx, key, start, stop)
}

// SafeSAdd performs a SADD operation with degraded mode handling
func (r *RedisClient) SafeSAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, sadd skipped"))
	}
	return r.Client.SAdd(ctx, key, members...)
}

// SafeSRem performs a SREM operation with degraded mode handling
func (r *RedisClient) SafeSRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, srem skipped"))
	}
	return r.Client.SRem(ctx, key, members...)
}

// SafeSMembers performs a SMEMBERS operation with degraded mode handling
func (r *RedisClient) SafeSMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	if r.IsDegraded() {
		return redis.NewStringSliceResult([]string{}, fmt.Errorf("redis is in degraded mode, smembers skipped"))
	}
	return r.Client.SMembers(ctx, key)
}

// SafeExists performs an EXISTS operation with degraded mode handling
func (r *RedisClient) SafeExists(ctx context.Context, keys ...string) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, exists skipped"))
	}
	return r.Client.Exists(ctx, keys...)
}

// SafeSMembers performs a SMEMBERS operation with degraded mode handling
/*func (r *RedisClient) SafeSMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	if r.IsDegraded() {
		return redis.NewStringSliceResult([]string{}, fmt.Errorf("redis is in degraded mode, smembers skipped"))
	}
	return r.Client.SMembers(ctx, key)
}*/

// SafeSCard performs a SCARD operation with degraded mode handling
func (r *RedisClient) SafeSCard(ctx context.Context, key string) *redis.IntCmd {
	if r.IsDegraded() {
		return redis.NewIntResult(0, fmt.Errorf("redis is in degraded mode, scard skipped"))
	}
	return r.Client.SCard(ctx, key)
}
