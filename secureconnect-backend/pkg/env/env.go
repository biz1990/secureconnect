package env

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// GetStringFromFile reads the environment variable value or from a file if FILE suffix is used
// This is used for Docker secrets support
func GetStringFromFile(key, defaultValue string) string {
	fileKey := key + "_FILE"
	filePath := os.Getenv(fileKey)

	if filePath != "" {
		// Read from file (Docker secret)
		content, err := os.ReadFile(filepath.Clean(filePath))
		if err == nil {
			// Trim whitespace and newlines
			return string(bytes.TrimSpace(content))
		}
		// If file read fails, fall back to env var
	}

	// Fall back to regular env var
	return GetString(key, defaultValue)
}

// GetString returns the environment variable value or the default value if not set
func GetString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetInt returns the environment variable value as an integer or the default value if not set
func GetInt(key string, defaultValue int) int {
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

// GetBool returns the environment variable value as a boolean or the default value if not set
func GetBool(key string, defaultValue bool) bool {
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

// GetDuration returns the environment variable value as a duration or the default value if not set
func GetDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// MustGetString returns the environment variable value or panics if not set
func MustGetString(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("required environment variable " + key + " is not set")
	}
	return value
}

// MustGetInt returns the environment variable value as an integer or panics if not set
func MustGetInt(key string) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		panic("required environment variable " + key + " is not set")
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		panic("environment variable " + key + " is not a valid integer: " + err.Error())
	}
	return value
}
