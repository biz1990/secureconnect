package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
	
	"github.com/jackc/pgx/v5/pgxpool"
)

// CockroachDB connection using pgx (PostgreSQL-compatible driver)
type CockroachDB struct {
	Pool *pgxpool.Pool
}

// CockroachConfig holds CockroachDB connection configuration
type CockroachConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// NewCockroachDB creates a new CockroachDB connection pool
func NewCockroachDB(ctx context.Context, config *CockroachConfig) (*CockroachDB, error) {
	// Build connection string
	// Format: postgresql://user:password@host:port/database?sslmode=disable
	connString := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.SSLMode,
	)
	
	// Configure pool
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}
	
	// Set pool parameters
	poolConfig.MaxConns = 25                      // Max concurrent connections
	poolConfig.MinConns = 5                       // Min idle connections
	poolConfig.MaxConnLifetime = time.Hour        // Max connection lifetime
	poolConfig.MaxConnIdleTime = 30 * time.Minute // Max idle time
	poolConfig.HealthCheckPeriod = time.Minute    // Health check interval
	
	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}
	
	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	return &CockroachDB{Pool: pool}, nil
}

// Close closes the connection pool
func (db *CockroachDB) Close() {
	db.Pool.Close()
}

// Ping tests the database connection
func (db *CockroachDB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// Stats returns pool statistics
func (db *CockroachDB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}

// Helper: NewCockroachDBFromEnv creates connection from environment variables
func NewCockroachDBFromEnv(ctx context.Context) (*CockroachDB, error) {
	config := &CockroachConfig{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvPortOrDefault("DB_PORT", 26257),
		User:     getEnvOrDefault("DB_USER", "root"),
		Password: getEnvOrDefault("DB_PASSWORD", ""),
		Database: getEnvOrDefault("DB_NAME", "secureconnect_poc"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"), // insecure for dev
	}
	
	return NewCockroachDB(ctx, config)
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvPortOrDefault(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
