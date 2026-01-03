package config

import (
    "os"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/stdlib"
)

// Struct Config chứa các biến môi trường
type Config struct {
    Env            string        `env:"ENV"`
    DBHost         string        `env:"DB_HOST"`
    DBPort         string        `env:"DB_PORT"`
    DBUser         string        `env:"DB_USER"`
    DBName         string        `env:"DB_NAME"`
    DBSSLMode      string        `env:"DB_SSLMODE"`
    RedisHost      string        `env:"REDIS_HOST"`
    MinIOEndpoint string        `env:"MINIO_ENDPOINT"`
    MinIOAccessKey string        `env:"MINIO_ACCESS_KEY"`
    MinIOSecretKey string        `env:"MINIO_SECRET_KEY"`
    JWTSecret      string        `env:"JWT_SECRET"`
}

// LoadConfig đọc các biến môi trường từ OS (hoặc Docker)
func LoadConfig() *Config {
    return &Config{
        Env:            getEnv("ENV", "development"),
        DBHost:         getEnv("DB_HOST", "localhost"),
        DBPort:         getEnv("DB_PORT", "26257"),
        DBUser:         getEnv("DB_USER", "root"),
        DBName:         getEnv("DB_NAME", "secureconnect_poc"),
        DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
        RedisHost:      getEnv("REDIS_HOST", "localhost"),
        MinIOEndpoint: getEnv("MINIO_ENDPOINT", "http://localhost:9000"),
        MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
        MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
        JWTSecret:      getEnv("JWT_SECRET", "secret"),
    }
}

// GetDBConnectionString trả về chuỗi kết nối cho CockroachDB
func (c *Config) GetDBConnectionString() string {
    connStr := "host=" + c.DBHost + " port=" + c.DBPort + " user=" + c.DBUser + " dbname=" + c.DBName + " sslmode=" + c.DBSSLMode
    return connStr
}

// GetRedisAddr trả về địa chỉ Redis
func (c *Config) GetRedisAddr() string {
    return c.RedisHost + ":6379"
}

// Hàm helper để lấy env var, nếu không có thì dùng giá trị mặc định
func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
}