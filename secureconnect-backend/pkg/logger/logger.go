package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log is the global logger instance
	Log *zap.Logger
	// Sugar is the sugared logger for easier use
	Sugar *zap.SugaredLogger
)

// Config holds logger configuration
type Config struct {
	Level    string // debug, info, warn, error
	Format   string // json, text
	Output   string // stdout, file
	FilePath string
}

// Init initializes the global logger with configuration
func Init(cfg *Config) error {
	var zapConfig zap.Config

	// Set log level
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// Configure based on format
	if cfg.Format == "json" {
		zapConfig = zap.NewProductionConfig()
		zapConfig.EncoderConfig.TimeKey = "timestamp"
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapConfig.Level = zap.NewAtomicLevelAt(level)

	// Configure output
	if cfg.Output == "file" && cfg.FilePath != "" {
		zapConfig.OutputPaths = []string{cfg.FilePath}
		zapConfig.ErrorOutputPaths = []string{cfg.FilePath}
	} else {
		zapConfig.OutputPaths = []string{"stdout"}
		zapConfig.ErrorOutputPaths = []string{"stderr"}
	}

	// Build logger
	var err error
	Log, err = zapConfig.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return err
	}

	Sugar = Log.Sugar()

	return nil
}

// InitDefault initializes logger with default settings
func InitDefault() {
	cfg := &Config{
		Level:    getEnv("LOG_LEVEL", "info"),
		Format:   getEnv("LOG_FORMAT", "json"),
		Output:   getEnv("LOG_OUTPUT", "stdout"),
		FilePath: getEnv("LOG_FILE_PATH", "/logs/app.log"),
	}

	if err := Init(cfg); err != nil {
		// Fallback to basic logger
		Log, _ = zap.NewProduction()
		Sugar = Log.Sugar()
	}
}

// Context key for request ID
type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// FromContext creates a logger with context fields
func FromContext(ctx context.Context) *zap.Logger {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return Log.With(zap.String("request_id", requestID))
	}
	return Log
}

// Convenience functions

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// With creates a child logger with additional fields
func With(fields ...zap.Field) *zap.Logger {
	return Log.With(fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	return Log.Sync()
}

// Helper function
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
