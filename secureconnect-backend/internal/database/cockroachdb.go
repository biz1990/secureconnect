package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"secureconnect-backend/pkg/logger"
)

// DBConfig contains database configuration
type DBConfig struct {
	MaxOpenConns       int
	MaxIdleConns       int
	ConnAcquireTimeout time.Duration
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	HealthCheckPeriod  time.Duration
}

// DefaultDBConfig returns default database configuration
func DefaultDBConfig() *DBConfig {
	return &DBConfig{
		MaxOpenConns:       25,               // Maximum number of open connections
		MaxIdleConns:       25,               // Maximum number of idle connections
		ConnAcquireTimeout: 5 * time.Second,  // Wait time for acquiring connection
		ConnMaxLifetime:    1 * time.Hour,    // Maximum connection lifetime
		ConnMaxIdleTime:    5 * time.Minute,  // Maximum idle time before closing
		HealthCheckPeriod:  30 * time.Second, // Health check interval
	}
}

// DB wraps the pgxpool.Pool with additional configuration and helper methods
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new database connection pool with configured limits
func NewDB(ctx context.Context, connString string, dbConfig *DBConfig) (*DB, error) {
	// Parse connection string
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Apply configuration
	if dbConfig == nil {
		dbConfig = DefaultDBConfig()
	}

	// Set pool configuration
	config.MaxConns = int32(dbConfig.MaxOpenConns)
	config.MaxConnLifetime = dbConfig.ConnMaxLifetime
	config.MaxConnIdleTime = dbConfig.ConnMaxIdleTime
	config.HealthCheckPeriod = dbConfig.HealthCheckPeriod

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() error {
	db.Pool.Close()
	logger.Info("Database connection pool closed")
	return nil
}

// GetPool returns the underlying pgxpool.Pool for direct access if needed
func (db *DB) GetPool() *pgxpool.Pool {
	return db.Pool
}

// AcquireConn attempts to acquire a connection with timeout
func (db *DB) AcquireConn(ctx context.Context) (*pgxpool.Conn, error) {
	// Try to acquire connection with timeout
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire database connection: %w", err)
	}

	return conn, nil
}

// Stats returns connection pool statistics
func (db *DB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}
