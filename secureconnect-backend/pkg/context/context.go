package context

import (
	"context"
	"time"
)

// Default timeouts for different operations
const (
	// DefaultTimeout is the default timeout for most operations
	DefaultTimeout = 30 * time.Second

	// ShortTimeout is for quick operations like cache lookups
	ShortTimeout = 5 * time.Second

	// MediumTimeout is for database queries
	MediumTimeout = 10 * time.Second

	// LongTimeout is for complex operations or batch processing
	LongTimeout = 60 * time.Second

	// VeryLongTimeout is for operations that may take significant time
	VeryLongTimeout = 5 * time.Minute
)

// WithDefaultTimeout creates a context with the default timeout
func WithDefaultTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, DefaultTimeout)
}

// WithShortTimeout creates a context with a short timeout
func WithShortTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, ShortTimeout)
}

// WithMediumTimeout creates a context with a medium timeout
func WithMediumTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, MediumTimeout)
}

// WithLongTimeout creates a context with a long timeout
func WithLongTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, LongTimeout)
}

// WithVeryLongTimeout creates a context with a very long timeout
func WithVeryLongTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, VeryLongTimeout)
}

// WithTimeout creates a context with a custom timeout
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// WithDeadline creates a context with a custom deadline
func WithDeadline(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}
