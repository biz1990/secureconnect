package database

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// DefaultCassandraQueryTimeout is the default timeout for Cassandra queries
const DefaultCassandraQueryTimeout = 5 * time.Second

// CassandraDB wraps the gocql Session with context support
type CassandraDB struct {
	Session *gocql.Session
}

// CassandraConfig holds Cassandra connection configuration
type CassandraConfig struct {
	Hosts    []string
	Keyspace string
	Username string
	Password string
	Timeout  time.Duration
}

// NewCassandraDB creates a new CassandraDB instance with configured timeouts
// DEPRECATED: Use NewCassandraDBWithConfig for authentication support
func NewCassandraDB(hosts []string, keyspace string) (*CassandraDB, error) {
	return NewCassandraDBWithConfig(&CassandraConfig{
		Hosts:    hosts,
		Keyspace: keyspace,
		Timeout:  DefaultCassandraQueryTimeout,
	})
}

// NewCassandraDBWithConfig creates a new CassandraDB instance with full configuration
func NewCassandraDBWithConfig(config *CassandraConfig) (*CassandraDB, error) {
	cluster := gocql.NewCluster(config.Hosts...)
	cluster.Keyspace = config.Keyspace
	cluster.Consistency = gocql.Quorum

	// Set default timeout for all queries
	if config.Timeout > 0 {
		cluster.Timeout = config.Timeout
	} else {
		cluster.Timeout = DefaultCassandraQueryTimeout
	}

	// Configure authentication if credentials are provided
	if config.Username != "" && config.Password != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: config.Username,
			Password: config.Password,
		}
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create Cassandra session: %w", err)
	}
	return &CassandraDB{Session: session}, nil
}

// Close closes the Cassandra session
func (c *CassandraDB) Close() {
	c.Session.Close()
}

// QueryWithContext executes a query with context-based timeout
// This ensures that queries respect context cancellation and don't hang indefinitely
func (c *CassandraDB) QueryWithContext(ctx context.Context, stmt string, values ...interface{}) *gocql.Query {
	// Check if context has a deadline
	if deadline, ok := ctx.Deadline(); ok {
		// Calculate timeout from context deadline
		timeout := time.Until(deadline)
		if timeout <= 0 {
			timeout = DefaultCassandraQueryTimeout
		}

		// Create query with timeout
		return c.Session.Query(stmt, values...).WithContext(ctx)
	}

	// No deadline in context, use default timeout
	return c.Session.Query(stmt, values...).WithContext(ctx)
}

// ExecWithContext executes a query without returning results, with context-based timeout
func (c *CassandraDB) ExecWithContext(ctx context.Context, stmt string, values ...interface{}) error {
	query := c.QueryWithContext(ctx, stmt, values...)
	return query.Exec()
}
