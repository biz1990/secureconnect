package database

import (
	"fmt"
	"os"
	"time"
	
	"github.com/gocql/gocql"
)

// CassandraDB connection wrapper
type CassandraDB struct {
	Session *gocql.Session
	Cluster *gocql.ClusterConfig
}

// CassandraConfig holds Cassandra connection configuration
type CassandraConfig struct {
	Hosts    []string      // Cassandra node addresses
	Keyspace string        // Keyspace to use
	Username string        // Optional authentication
	Password string        // Optional authentication
	Timeout  time.Duration // Connection timeout
}

// NewCassandraDB creates a new Cassandra session
func NewCassandraDB(config *CassandraConfig) (*CassandraDB, error) {
	// Create cluster configuration
	cluster := gocql.NewCluster(config.Hosts...)
	cluster.Keyspace = config.Keyspace
	cluster.Consistency = gocql.LocalQuorum // Write to 2/3 nodes for durability
	cluster.Timeout = config.Timeout
	
	// Set connection pool
	cluster.NumConns = 2 // Connections per host
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	
	// Authentication if provided
	if config.Username != "" && config.Password != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: config.Username,
			Password: config.Password,
		}
	}
	
	// Retry policy
	cluster.RetryPolicy = &gocql.ExponentialBackoffRetryPolicy{
		NumRetries: 3,
		Min:        time.Second,
		Max:        10 * time.Second,
	}
	
	// Create session
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create Cassandra session: %w", err)
	}
	
	return &CassandraDB{
		Session: session,
		Cluster: cluster,
	}, nil
}

// Close closes the Cassandra session
func (db *CassandraDB) Close() {
	if db.Session != nil {
		db.Session.Close()
	}
}

// Ping tests the connection
func (db *CassandraDB) Ping() error {
	// Execute simple query to test connection
	query := "SELECT now() FROM system.local"
	if err := db.Session.Query(query).Exec(); err != nil {
		return fmt.Errorf("cassandra ping failed: %w", err)
	}
	return nil
}

// Helper: NewCassandraDBFromEnv creates connection from environment variables
func NewCassandraDBFromEnv() (*CassandraDB, error) {
	host := os.Getenv("CASSANDRA_HOST")
	if host == "" {
		host = "localhost"
	}
	
	keyspace := os.Getenv("CASSANDRA_KEYSPACE")
	if keyspace == "" {
		keyspace = "secureconnect_ks"
	}
	
	config := &CassandraConfig{
		Hosts:    []string{host},
		Keyspace: keyspace,
		Username: os.Getenv("CASSANDRA_USER"),
		Password: os.Getenv("CASSANDRA_PASSWORD"),
		Timeout:  10 * time.Second,
	}
	
	return NewCassandraDB(config)
}
