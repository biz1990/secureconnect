package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"secureconnect-backend/internal/database"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

// DBPoolLimiter implements connection pool exhaustion protection
type DBPoolLimiter struct {
	db *database.DB
}

// NewDBPoolLimiter creates a new database pool limiter
func NewDBPoolLimiter(db *database.DB) *DBPoolLimiter {
	return &DBPoolLimiter{db: db}
}

// Middleware returns a Gin middleware for database connection pool protection
func (dpl *DBPoolLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current pool statistics
		stats := dpl.db.Stats()

		// Calculate idle connections
		acquireCount := stats.AcquireCount()
		totalConns := int64(stats.TotalConns())
		idleConns := totalConns - acquireCount

		// Update metrics
		metrics.RecordDBConnectionsInUse(int(acquireCount))
		metrics.RecordDBConnectionsIdle(int(idleConns))

		// Check if pool is exhausted (80% threshold)
		poolUsageThreshold := 0.8
		maxConns := float64(stats.MaxConns())
		currentConns := float64(acquireCount)
		poolUsage := currentConns / maxConns

		// Log pool status
		logger.Debug("Database connection pool status",
			zap.Int32("max_conns", stats.MaxConns()),
			zap.Int64("acquire_count", acquireCount),
			zap.Int64("idle_conns", idleConns),
			zap.Float64("pool_usage", poolUsage),
		)

		// If pool is exhausted, return 503 Service Unavailable
		if poolUsage >= poolUsageThreshold {
			logger.Warn("Database connection pool exhausted",
				zap.Int32("max_conns", stats.MaxConns()),
				zap.Int64("acquire_count", acquireCount),
				zap.Float64("pool_usage", poolUsage),
			)
			metrics.RecordDBConnectionAcquireTimeout()

			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Service temporarily unavailable",
				"code":  "DB_POOL_EXHAUSTED",
			})
			c.Abort()
			return
		}

		// Try to acquire connection with timeout
		startTime := time.Now()
		conn, err := dpl.db.AcquireConn(c.Request.Context())
		if err != nil {
			// Check if context was cancelled
			if c.Request.Context().Err() != nil {
				logger.Debug("Request cancelled before acquiring connection",
					zap.Error(err))
				c.Abort()
				return
			}

			// Connection acquisition failed
			logger.Error("Failed to acquire database connection",
				zap.Error(err),
				zap.Int32("max_conns", stats.MaxConns()),
				zap.Int64("acquire_count", acquireCount),
			)
			metrics.RecordDBConnectionAcquireTimeout()

			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Service temporarily unavailable",
				"code":  "DB_CONNECTION_FAILED",
			})
			c.Abort()
			return
		}

		// Record connection acquisition metrics
		duration := time.Since(startTime).Seconds()
		metrics.RecordDBConnectionAcquire()
		metrics.RecordDBConnectionAcquireDuration(duration)

		// Store connection in context for later use
		c.Set("db_conn", conn)

		// Connection is automatically released when conn goes out of scope
		// No manual release needed for pgxpool v5

		c.Next()
	}
}

// GetDBConn retrieves database connection from context
func GetDBConn(c *gin.Context) *pgxpool.Conn {
	conn, exists := c.Get("db_conn")
	if !exists {
		return nil
	}
	return conn.(*pgxpool.Conn)
}

// CheckPoolHealth checks the health of the database connection pool
func (dpl *DBPoolLimiter) CheckPoolHealth(ctx context.Context) error {
	stats := dpl.db.Stats()

	// Check if pool is exhausted
	acquireCount := stats.AcquireCount()
	maxConns := int64(stats.MaxConns())
	if acquireCount >= maxConns {
		return fmt.Errorf("connection pool exhausted: %d/%d connections in use",
			acquireCount, maxConns)
	}

	// Check if there are any idle connections
	totalConns := int64(stats.TotalConns())
	idleConns := totalConns - acquireCount
	if idleConns == 0 && acquireCount > 0 {
		logger.Warn("No idle connections available",
			zap.Int64("acquire_count", acquireCount),
			zap.Int32("max_conns", stats.MaxConns()),
		)
	}

	return nil
}

// GetPoolStats returns current pool statistics
func (dpl *DBPoolLimiter) GetPoolStats() *pgxpool.Stat {
	return dpl.db.Stats()
}

// GetPoolUsage returns the current pool usage percentage
func (dpl *DBPoolLimiter) GetPoolUsage() float64 {
	stats := dpl.db.Stats()
	if stats.MaxConns() == 0 {
		return 0.0
	}
	return float64(stats.AcquireCount()) / float64(stats.MaxConns())
}
