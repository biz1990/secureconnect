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
