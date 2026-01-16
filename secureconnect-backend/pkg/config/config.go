package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Cassandra CassandraConfig
	MinIO     MinIOConfig
	SMTP      SMTPConfig
	JWT       JWTConfig
	Log       LogConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port        int
	Environment string // development, staging, production
	ServiceName string
}

// DatabaseConfig holds CockroachDB configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int
	MinConns int
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
	Timeout  time.Duration
}

// CassandraConfig holds Cassandra configuration
type CassandraConfig struct {
	Hosts       []string
	Keyspace    string
	Consistency string
	Timeout     time.Duration
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// MinIOConfig holds MinIO configuration
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret             string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level    string // debug, info, warn, error
	Format   string // json, text
	Output   string // stdout, file
	FilePath string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:        getEnvAsInt("PORT", 8080),
			Environment: getEnv("ENV", "development"),
			ServiceName: getEnv("SERVICE_NAME", "secureconnect"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 26257),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "secureconnect"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
			MaxConns: getEnvAsInt("DB_MAX_CONNS", 25),
			MinConns: getEnvAsInt("DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			PoolSize: getEnvAsInt("REDIS_POOL_SIZE", 10),
			Timeout:  time.Duration(getEnvAsInt("REDIS_TIMEOUT", 5)) * time.Second,
		},
		Cassandra: CassandraConfig{
			Hosts:       getEnvAsSlice("CASSANDRA_HOSTS", []string{"localhost"}),
			Keyspace:    getEnv("CASSANDRA_KEYSPACE", "secureconnect"),
			Consistency: getEnv("CASSANDRA_CONSISTENCY", "QUORUM"),
			Timeout:     time.Duration(getEnvAsInt("CASSANDRA_TIMEOUT", 600)) * time.Millisecond,
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:     getEnvAsInt("SMTP_PORT", 587),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@secureconnect.com"),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			UseSSL:    getEnvAsBool("MINIO_USE_SSL", false),
			Bucket:    getEnv("MINIO_BUCKET", "secureconnect"),
		},
		JWT: JWTConfig{
			Secret:             getEnv("JWT_SECRET", ""),
			AccessTokenExpiry:  time.Duration(getEnvAsInt("JWT_ACCESS_EXPIRY", 15)) * time.Minute,
			RefreshTokenExpiry: time.Duration(getEnvAsInt("JWT_REFRESH_EXPIRY", 720)) * time.Hour,
		},
		Log: LogConfig{
			Level:    getEnv("LOG_LEVEL", "info"),
			Format:   getEnv("LOG_FORMAT", "json"),
			Output:   getEnv("LOG_OUTPUT", "stdout"),
			FilePath: getEnv("LOG_FILE_PATH", "/logs/app.log"),
		},
	}

	// Validate critical configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate JWT secret in production
	if c.Server.Environment == "production" {
		if c.JWT.Secret == "" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
		if len(c.JWT.Secret) < 32 {
			return fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
		}
	}

	// Warn about weak secrets even in development
	if c.JWT.Secret == "" || c.JWT.Secret == "super-secret-key-change-in-production" {
		fmt.Println("⚠️  WARNING: Using default/weak JWT secret. This is INSECURE for production!")
	}

	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
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

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	// Simple comma-separated string parsing
	var result []string
	for i := 0; i < len(valueStr); {
		j := i
		for j < len(valueStr) && valueStr[j] != ',' {
			j++
		}
		if i < j {
			result = append(result, valueStr[i:j])
		}
		i = j + 1
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}
