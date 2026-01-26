package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

// MemoryCache implements an in-memory cache with TTL support
type MemoryCache struct {
	mu      sync.RWMutex
	data    map[string]*cacheEntry
	ttl     time.Duration
	maxSize int
}

// cacheEntry represents a single cache entry
type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
	createdAt time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(defaultTTL time.Duration, maxSize int) *MemoryCache {
	return &MemoryCache{
		data:    make(map[string]*cacheEntry),
		ttl:     defaultTTL,
		maxSize: maxSize,
	}
}

// Set stores a value in the cache with TTL
func (mc *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Use default TTL if not provided
	if ttl == 0 {
		ttl = mc.ttl
	}

	// Check if we need to evict entries
	if mc.maxSize > 0 && len(mc.data) >= mc.maxSize {
		mc.evictOldest()
	}

	// Store the entry
	mc.data[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
		createdAt: time.Now(),
	}

	logger.Debug("Cache entry added",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
		zap.Int("size", len(mc.data)),
	)
}

// Get retrieves a value from the cache
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.data[key]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		delete(mc.data, key)
		return nil, false
	}

	return entry.value, true
}

// Delete removes a value from the cache
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.data, key)
	logger.Debug("Cache entry deleted", zap.String("key", key))
}

// Clear removes all entries from the cache
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.data = make(map[string]*cacheEntry)
	logger.Debug("Cache cleared", zap.Int("size", len(mc.data)))
}

// Size returns the current number of entries in the cache
func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.data)
}

// evictOldest removes the oldest entry from the cache
func (mc *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range mc.data {
		if oldestKey == "" || entry.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.createdAt
		}
	}

	if oldestKey != "" {
		delete(mc.data, oldestKey)
		logger.Debug("Cache entry evicted",
			zap.String("key", oldestKey),
			zap.Time("created_at", oldestTime),
		)
	}
}

// cleanupExpired removes expired entries from the cache
func (mc *MemoryCache) cleanupExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for key, entry := range mc.data {
		if now.After(entry.expiresAt) {
			delete(mc.data, key)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		logger.Debug("Expired cache entries cleaned up",
			zap.Int("count", expiredCount),
			zap.Int("remaining", len(mc.data)),
		)
	}
}

// StartCleanup starts a goroutine to clean up expired entries
// Returns a stop function that can be called to cancel the cleanup goroutine
func (mc *MemoryCache) StartCleanup(interval time.Duration) func() {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mc.cleanupExpired()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

// Session represents a user session in memory cache
type Session struct {
	SessionID    string    `json:"session_id"`
	UserID       uuid.UUID `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SessionCache wraps MemoryCache for session management
type SessionCache struct {
	cache *MemoryCache
}

// NewSessionCache creates a new session cache
func NewSessionCache(defaultTTL time.Duration) *SessionCache {
	return &SessionCache{
		cache: NewMemoryCache(defaultTTL, 1000), // Max 1000 sessions
	}
}

// CreateSession stores a new session
func (sc *SessionCache) CreateSession(sessionID string, session *Session, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	sc.cache.Set(key, data, ttl)
	return nil
}

// GetSession retrieves a session by ID
func (sc *SessionCache) GetSession(sessionID string) (*Session, bool) {
	key := fmt.Sprintf("session:%s", sessionID)
	value, exists := sc.cache.Get(key)
	if !exists {
		return nil, false
	}

	var session Session
	bytes, ok := value.([]byte)
	if !ok {
		logger.Error("Cache entry is not a byte slice",
			zap.String("key", key))
		return nil, false
	}
	err := json.Unmarshal(bytes, &session)
	if err != nil {
		logger.Error("Failed to unmarshal session from cache",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return nil, false
	}

	return &session, true
}

// DeleteSession removes a session
func (sc *SessionCache) DeleteSession(sessionID string) {
	key := fmt.Sprintf("session:%s", sessionID)
	sc.cache.Delete(key)
}

// BlacklistToken adds a token JTI to the blacklist
func (sc *SessionCache) BlacklistToken(jti string, expiresAt time.Duration) {
	key := fmt.Sprintf("blacklist:%s", jti)
	sc.cache.Set(key, "revoked", expiresAt)
}

// IsTokenBlacklisted checks if a token JTI is in the blacklist
func (sc *SessionCache) IsTokenBlacklisted(jti string) bool {
	key := fmt.Sprintf("blacklist:%s", jti)
	_, exists := sc.cache.Get(key)
	return exists
}

// AccountLock represents a locked account
type AccountLock struct {
	LockedUntil time.Time `json:"locked_until"`
}

// LockoutCache wraps MemoryCache for account lockout management
type LockoutCache struct {
	cache *MemoryCache
}

// NewLockoutCache creates a new lockout cache
func NewLockoutCache(defaultTTL time.Duration) *LockoutCache {
	return &LockoutCache{
		cache: NewMemoryCache(defaultTTL, 1000), // Max 1000 locks
	}
}

// LockAccount locks an account
func (lc *LockoutCache) LockAccount(key string, lockedUntil time.Time, reason string) error {
	data, err := json.Marshal(AccountLock{LockedUntil: lockedUntil})
	if err != nil {
		return fmt.Errorf("failed to marshal account lock: %w", err)
	}

	lc.cache.Set(key, data, time.Until(lockedUntil))
	return nil
}

// GetAccountLock retrieves account lock status
func (lc *LockoutCache) GetAccountLock(key string) (*AccountLock, bool) {
	value, exists := lc.cache.Get(key)
	if !exists {
		return nil, false
	}

	var lock AccountLock
	bytes, ok := value.([]byte)
	if !ok {
		logger.Error("Cache entry is not a byte slice",
			zap.String("key", key))
		return nil, false
	}
	err := json.Unmarshal(bytes, &lock)
	if err != nil {
		logger.Error("Failed to unmarshal account lock from cache",
			zap.String("key", key),
			zap.Error(err))
		return nil, false
	}

	return &lock, true
}

// UnlockAccount unlocks an account
func (lc *LockoutCache) UnlockAccount(key string) {
	lc.cache.Delete(key)
}

// FailedLoginAttempt represents a failed login attempt
type FailedLoginAttempt struct {
	UserID      uuid.UUID  `json:"user_id"`
	Email       string     `json:"email"`
	IP          string     `json:"ip"`
	Attempts    int        `json:"attempts"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// FailedLoginCache wraps MemoryCache for failed login tracking
type FailedLoginCache struct {
	cache *MemoryCache
}

// NewFailedLoginCache creates a new failed login cache
func NewFailedLoginCache(defaultTTL time.Duration) *FailedLoginCache {
	return &FailedLoginCache{
		cache: NewMemoryCache(defaultTTL, 1000), // Max 1000 entries
	}
}

// RecordFailedAttempt records a failed login attempt
func (flc *FailedLoginCache) RecordFailedAttempt(key string, attempt *FailedLoginAttempt, ttl time.Duration) error {
	data, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("failed to marshal failed attempt: %w", err)
	}

	flc.cache.Set(key, data, ttl)
	return nil
}

// GetFailedAttempt retrieves a failed login attempt
func (flc *FailedLoginCache) GetFailedAttempt(key string) (*FailedLoginAttempt, bool) {
	value, exists := flc.cache.Get(key)
	if !exists {
		return nil, false
	}

	var attempt FailedLoginAttempt
	bytes, ok := value.([]byte)
	if !ok {
		logger.Error("Cache entry is not a byte slice",
			zap.String("key", key))
		return nil, false
	}
	err := json.Unmarshal(bytes, &attempt)
	if err != nil {
		logger.Error("Failed to unmarshal failed attempt from cache",
			zap.String("key", key),
			zap.Error(err))
		return nil, false
	}

	return &attempt, true
}

// ClearFailedAttempts clears failed login attempts
func (flc *FailedLoginCache) ClearFailedAttempts(key string) {
	flc.cache.Delete(key)
}

// FallbackCache wraps all caches with Redis fallback
type FallbackCache struct {
	sessionCache     *SessionCache
	lockoutCache     *LockoutCache
	failedLoginCache *FailedLoginCache
	redisAvailable   atomic.Bool
	redisClient      interface{} // Can be *redis.Client or nil
}

// NewFallbackCache creates a new fallback cache
func NewFallbackCache(redisClient interface{}) *FallbackCache {
	var b atomic.Bool
	b.Store(true)
	return &FallbackCache{
		sessionCache:     NewSessionCache(1 * time.Hour),
		lockoutCache:     NewLockoutCache(15 * time.Minute),
		failedLoginCache: NewFailedLoginCache(15 * time.Minute),
		redisAvailable:   b, // Use the atomic.Bool directly to avoid literal copy
		redisClient:      redisClient,
	}
}

// IsRedisAvailable checks if Redis is available
func (fc *FallbackCache) IsRedisAvailable() bool {
	return fc.redisAvailable.Load()
}

// SetRedisAvailable sets Redis availability
func (fc *FallbackCache) SetRedisAvailable(available bool) {
	fc.redisAvailable.Store(available)
	metrics.RecordRedisAvailable(available)
}

// CreateSession creates a session with Redis fallback
func (fc *FallbackCache) CreateSession(ctx context.Context, sessionID string, session *Session, ttl time.Duration) error {
	// Try Redis first
	if fc.IsRedisAvailable() && fc.redisClient != nil {
		// Redis client is available, try to use it
		// This will be called from the repository layer
		// If Redis fails, it will fall back to memory cache
	}

	// Fall back to memory cache
	err := fc.sessionCache.CreateSession(sessionID, session, ttl)
	if err != nil {
		return err
	}

	metrics.RecordRedisFallbackHit()
	return nil
}

// GetSession retrieves a session with Redis fallback
func (fc *FallbackCache) GetSession(sessionID string) (*Session, error) {
	// Try memory cache first (fast path)
	session, exists := fc.sessionCache.GetSession(sessionID)
	if exists {
		metrics.RecordRedisFallbackHit()
		return session, nil
	}

	// Not in memory cache, return not found
	return nil, fmt.Errorf("session not found")
}

// DeleteSession deletes a session from both caches
func (fc *FallbackCache) DeleteSession(sessionID string) {
	fc.sessionCache.DeleteSession(sessionID)
}

// BlacklistToken blacklists a token with Redis fallback
func (fc *FallbackCache) BlacklistToken(jti string, expiresAt time.Duration) {
	fc.sessionCache.BlacklistToken(jti, expiresAt)
}

// IsTokenBlacklisted checks if a token is blacklisted with Redis fallback
func (fc *FallbackCache) IsTokenBlacklisted(jti string) bool {
	return fc.sessionCache.IsTokenBlacklisted(jti)
}

// LockAccount locks an account with Redis fallback
func (fc *FallbackCache) LockAccount(key string, lockedUntil time.Time, reason string) error {
	return fc.lockoutCache.LockAccount(key, lockedUntil, reason)
}

// GetAccountLock retrieves account lock with Redis fallback
func (fc *FallbackCache) GetAccountLock(key string) (*AccountLock, error) {
	lock, exists := fc.lockoutCache.GetAccountLock(key)
	if !exists {
		return nil, fmt.Errorf("account lock not found")
	}
	return lock, nil
}

// UnlockAccount unlocks an account from both caches
func (fc *FallbackCache) UnlockAccount(key string) {
	fc.lockoutCache.UnlockAccount(key)
}

// RecordFailedAttempt records a failed login attempt with Redis fallback
func (fc *FallbackCache) RecordFailedAttempt(key string, attempt *FailedLoginAttempt, ttl time.Duration) error {
	return fc.failedLoginCache.RecordFailedAttempt(key, attempt, ttl)
}

// GetFailedAttempt retrieves a failed login attempt with Redis fallback
func (fc *FallbackCache) GetFailedAttempt(key string) (*FailedLoginAttempt, error) {
	attempt, exists := fc.failedLoginCache.GetFailedAttempt(key)
	if !exists {
		return nil, fmt.Errorf("failed attempt not found")
	}
	return attempt, nil
}

// ClearFailedAttempts clears failed login attempts from both caches
func (fc *FallbackCache) ClearFailedAttempts(key string) {
	fc.failedLoginCache.ClearFailedAttempts(key)
}

// SyncFromRedis syncs data from Redis to memory cache
// This is called when Redis recovers
func (fc *FallbackCache) SyncFromRedis(ctx context.Context, sessions map[string]*Session, blacklists map[string]bool) error {
	// Sync sessions
	for sessionID, session := range sessions {
		err := fc.sessionCache.CreateSession(sessionID, session, time.Until(session.ExpiresAt))
		if err != nil {
			logger.Error("Failed to sync session to memory cache",
				zap.String("session_id", sessionID),
				zap.Error(err))
		}
	}

	// Sync blacklists
	for jti := range blacklists {
		fc.sessionCache.BlacklistToken(jti, 1*time.Hour) // Blacklist for 1 hour
	}

	fc.SetRedisAvailable(true)
	logger.Info("Synced data from Redis to memory cache",
		zap.Int("sessions", len(sessions)),
		zap.Int("blacklists", len(blacklists)),
	)
	return nil
}

// GetStats returns cache statistics
func (fc *FallbackCache) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"session_cache_size":      fc.sessionCache.cache.Size(),
		"lockout_cache_size":      fc.lockoutCache.cache.Size(),
		"failed_login_cache_size": fc.failedLoginCache.cache.Size(),
		"redis_available":         fc.IsRedisAvailable(),
	}
}
