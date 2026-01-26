package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisDB connection wrapper
type RedisDB struct {
	Client *redis.Client
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int           // Redis database number (0-15)
	PoolSize int           // Connection pool size
	Timeout  time.Duration // Command timeout
}

// NewRedisDB creates a new Redis client
func NewRedisDB(config *RedisConfig) (*RedisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		MaxRetries:   3,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisDB{Client: client}, nil
}

// Close closes the Redis connection
func (db *RedisDB) Close() error {
	return db.Client.Close()
}

// Ping tests the Redis connection
func (db *RedisDB) Ping(ctx context.Context) error {
	return db.Client.Ping(ctx).Err()
}

// Set stores a key-value pair with optional expiration
func (db *RedisDB) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return db.Client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key
func (db *RedisDB) Get(ctx context.Context, key string) (string, error) {
	return db.Client.Get(ctx, key).Result()
}

// Delete removes a key
func (db *RedisDB) Delete(ctx context.Context, keys ...string) error {
	return db.Client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (db *RedisDB) Exists(ctx context.Context, keys ...string) (int64, error) {
	return db.Client.Exists(ctx, keys...).Result()
}

// Publish publishes a message to a channel (for Pub/Sub)
func (db *RedisDB) Publish(ctx context.Context, channel string, message interface{}) error {
	return db.Client.Publish(ctx, channel, message).Err()
}

// Subscribe subscribes to channels (returns PubSub object)
func (db *RedisDB) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return db.Client.Subscribe(ctx, channels...)
}

// HSet sets hash field
func (db *RedisDB) HSet(ctx context.Context, key string, values ...interface{}) error {
	return db.Client.HSet(ctx, key, values...).Err()
}

// HGet gets hash field value
func (db *RedisDB) HGet(ctx context.Context, key, field string) (string, error) {
	return db.Client.HGet(ctx, key, field).Result()
}

// HGetAll gets all hash fields
func (db *RedisDB) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return db.Client.HGetAll(ctx, key).Result()
}

// Expire sets key expiration
func (db *RedisDB) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return db.Client.Expire(ctx, key, expiration).Err()
}

// Helper: NewRedisDBFromEnv creates Redis connection from environment variables
func NewRedisDBFromEnv() (*RedisDB, error) {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	portStr := os.Getenv("REDIS_PORT")
	port := 6379
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	config := &RedisConfig{
		Host:     host,
		Port:     port,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
		PoolSize: 10,
		Timeout:  5 * time.Second,
	}

	return NewRedisDB(config)
}
