package lockout

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// LockoutManager handles account lockout functionality
type LockoutManager struct {
	redisClient  *redis.Client
	maxAttempts  int
	lockDuration time.Duration
}

// NewLockoutManager creates a new lockout manager
func NewLockoutManager(redisClient *redis.Client) *LockoutManager {
	return &LockoutManager{
		redisClient:  redisClient,
		maxAttempts:  5,
		lockDuration: 15 * time.Minute,
	}
}

// LockoutConfig holds lockout configuration
type LockoutConfig struct {
	MaxAttempts  int
	LockDuration time.Duration
}

// SetConfig updates lockout configuration
func (lm *LockoutManager) SetConfig(config LockoutConfig) {
	if config.MaxAttempts > 0 {
		lm.maxAttempts = config.MaxAttempts
	}
	if config.LockDuration > 0 {
		lm.lockDuration = config.LockDuration
	}
}

// RecordFailedAttempt records a failed login attempt
func (lm *LockoutManager) RecordFailedAttempt(ctx context.Context, identifier string) error {
	// Key for tracking failed attempts
	key := fmt.Sprintf("lockout:failed:%s", identifier)

	// Increment failed attempts counter
	pipe := lm.redisClient.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, lm.lockDuration)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record failed attempt: %w", err)
	}

	return nil
}

// CheckLockout checks if an account is locked out
func (lm *LockoutManager) CheckLockout(ctx context.Context, identifier string) (bool, int, error) {
	// Key for tracking failed attempts
	key := fmt.Sprintf("lockout:failed:%s", identifier)

	// Get current failed attempts count
	countCmd := lm.redisClient.Get(ctx, key)
	count, err := countCmd.Int()
	if err != nil && err != redis.Nil {
		return false, 0, fmt.Errorf("failed to check lockout status: %w", err)
	}

	if err == redis.Nil {
		count = 0
	}

	remaining := lm.maxAttempts - count
	isLocked := count >= lm.maxAttempts

	return isLocked, remaining, nil
}

// ClearFailedAttempts clears failed attempts after successful login
func (lm *LockoutManager) ClearFailedAttempts(ctx context.Context, identifier string) error {
	// Key for tracking failed attempts
	key := fmt.Sprintf("lockout:failed:%s", identifier)

	// Delete the failed attempts counter
	err := lm.redisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to clear failed attempts: %w", err)
	}

	return nil
}

// GetLockoutInfo returns lockout information for an identifier
func (lm *LockoutManager) GetLockoutInfo(ctx context.Context, identifier string) (*LockoutInfo, error) {
	// Key for tracking failed attempts
	key := fmt.Sprintf("lockout:failed:%s", identifier)

	// Get current failed attempts count
	countCmd := lm.redisClient.Get(ctx, key)
	count, err := countCmd.Int()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get lockout info: %w", err)
	}

	if err == redis.Nil {
		count = 0
	}

	// Get TTL for lockout
	ttlCmd := lm.redisClient.TTL(ctx, key)
	ttl, err := ttlCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get lockout TTL: %w", err)
	}

	remaining := lm.maxAttempts - count
	isLocked := count >= lm.maxAttempts

	return &LockoutInfo{
		IsLocked:       isLocked,
		FailedAttempts: count,
		Remaining:      remaining,
		LockDuration:   lm.lockDuration,
		UnlockTime:     time.Now().Add(time.Duration(ttl) * time.Second),
	}, nil
}

// LockoutInfo contains lockout status information
type LockoutInfo struct {
	IsLocked       bool
	FailedAttempts int
	Remaining      int
	LockDuration   time.Duration
	UnlockTime     time.Time
}

// LockoutForUser locks out a specific user account
func (lm *LockoutManager) LockoutForUser(ctx context.Context, userID uuid.UUID, reason string) error {
	key := fmt.Sprintf("lockout:user:%s", userID.String())

	// Set lockout with TTL
	pipe := lm.redisClient.Pipeline()
	pipe.Set(ctx, key, reason, lm.lockDuration)
	pipe.Expire(ctx, key, lm.lockDuration)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to lockout user: %w", err)
	}

	return nil
}

// UnlockUser unlocks a specific user account
func (lm *LockoutManager) UnlockUser(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("lockout:user:%s", userID.String())

	err := lm.redisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to unlock user: %w", err)
	}

	return nil
}

// CheckUserLockout checks if a user is locked out
func (lm *LockoutManager) CheckUserLockout(ctx context.Context, userID uuid.UUID) (bool, string, error) {
	key := fmt.Sprintf("lockout:user:%s", userID.String())

	valCmd := lm.redisClient.Get(ctx, key)
	val, err := valCmd.Bytes()
	if err != nil && err != redis.Nil {
		return false, "", fmt.Errorf("failed to check user lockout: %w", err)
	}

	if err == redis.Nil {
		return false, "", nil
	}

	reason := string(val)

	return true, reason, nil
}
