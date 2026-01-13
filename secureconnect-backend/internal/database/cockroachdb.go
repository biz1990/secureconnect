package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

type CockroachConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int
	MinConns int
}

// NewCockroachDB creates a new connection pool from connection string
func NewCockroachDB(ctx context.Context, connString string) (*DB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// NewCockroachDBWithConfig creates a new connection pool from config struct
func NewCockroachDBWithConfig(ctx context.Context, cfg *CockroachConfig) (*DB, error) {
	connString := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Configure connection pool
	if cfg.MaxConns > 0 {
		config.MaxConns = int32(cfg.MaxConns)
	}
	if cfg.MinConns > 0 {
		config.MinConns = int32(cfg.MinConns)
	}
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection
func (db *DB) Close() {
	db.Pool.Close()
}

// GetPool returns the underlying pgxpool.Pool
func (db *DB) GetPool() *pgxpool.Pool {
	return db.Pool
}
