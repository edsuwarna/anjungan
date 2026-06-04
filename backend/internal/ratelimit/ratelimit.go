package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Status represents the rate limit status for a login attempt.
type Status struct {
	Allowed     bool
	Remaining   int
	ResetAfter  time.Duration
	LockedUntil *time.Time
}

// RateLimiter provides Redis-backed rate limiting for login attempts with
// per-IP+email rate limiting and per-email lockout.
type RateLimiter struct {
	redis       *redis.Client
	maxAttempts int
	window      time.Duration
	lockoutDur  time.Duration
}

// New creates a new RateLimiter.
//
//   - rdb: Redis client
//   - maxAttempts: number of failed attempts allowed within the window
//   - window: sliding window duration for counting attempts
//   - lockoutDur: duration an account is locked after exceeding maxAttempts
func New(rdb *redis.Client, maxAttempts int, window, lockoutDur time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:       rdb,
		maxAttempts: maxAttempts,
		window:      window,
		lockoutDur:  lockoutDur,
	}
}

// CheckLogin checks rate limit for IP+email combo and account lockout.
// Returns status with remaining attempts or lockout info.
func (rl *RateLimiter) CheckLogin(ctx context.Context, ip, email string) (*Status, error) {
	// 1. Check account-level lockout first — if the account is locked,
	//    deny regardless of IP-level counters.
	locked, remainingLockout, err := rl.IsLocked(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("check lockout: %w", err)
	}
	if locked {
		now := time.Now()
		lockedUntil := now.Add(remainingLockout)
		return &Status{
			Allowed:     false,
			Remaining:   0,
			ResetAfter:  remainingLockout,
			LockedUntil: &lockedUntil,
		}, nil
	}

	// 2. Check IP+email rate counter
	key := fmt.Sprintf("rate:login:%s:%s", ip, email)
	count, err := rl.redis.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("get rate count: %w", err)
	}

	// Key does not exist yet — no attempts recorded in this window
	if err == redis.Nil {
		return &Status{
			Allowed:    true,
			Remaining:  rl.maxAttempts,
			ResetAfter: 0,
		}, nil
	}

	remaining := rl.maxAttempts - count
	if remaining < 0 {
		remaining = 0
	}

	ttl, err := rl.redis.TTL(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("get rate ttl: %w", err)
	}
	if ttl < 0 {
		ttl = 0
	}

	return &Status{
		Allowed:    count < rl.maxAttempts,
		Remaining:  remaining,
		ResetAfter: ttl,
	}, nil
}

// RecordFailed increments the attempt counter. Returns updated Status.
// If the counter reaches maxAttempts, the account is locked out for the
// configured lockout duration.
func (rl *RateLimiter) RecordFailed(ctx context.Context, ip, email string) (*Status, error) {
	key := fmt.Sprintf("rate:login:%s:%s", ip, email)

	count, err := rl.redis.Incr(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("increment rate count: %w", err)
	}

	// Set expiry on first increment so the key eventually auto-expires
	if count == 1 {
		rl.redis.Expire(ctx, key, rl.window)
	}

	remaining := rl.maxAttempts - int(count)
	if remaining < 0 {
		remaining = 0
	}

	// Lock the account if the maximum number of attempts has been reached
	if int(count) >= rl.maxAttempts {
		lockoutKey := fmt.Sprintf("lockout:%s", email)
		if err := rl.redis.Set(ctx, lockoutKey, "1", rl.lockoutDur).Err(); err != nil {
			return nil, fmt.Errorf("set lockout: %w", err)
		}
	}

	ttl, err := rl.redis.TTL(ctx, key).Result()
	if err != nil {
		ttl = 0
	} else if ttl < 0 {
		ttl = 0
	}

	return &Status{
		Allowed:    int(count) < rl.maxAttempts,
		Remaining:  remaining,
		ResetAfter: ttl,
	}, nil
}

// RecordSuccess clears rate limit counters on successful login. Both the
// IP+email rate counter and the account lockout key are removed.
func (rl *RateLimiter) RecordSuccess(ctx context.Context, ip, email string) {
	key := fmt.Sprintf("rate:login:%s:%s", ip, email)
	rl.redis.Del(ctx, key)

	lockoutKey := fmt.Sprintf("lockout:%s", email)
	rl.redis.Del(ctx, lockoutKey)
}

// IsLocked checks if an account is currently locked out.
// Returns whether the account is locked, the remaining lockout duration, and
// any error encountered.
func (rl *RateLimiter) IsLocked(ctx context.Context, email string) (bool, time.Duration, error) {
	lockoutKey := fmt.Sprintf("lockout:%s", email)

	exists, err := rl.redis.Exists(ctx, lockoutKey).Result()
	if err != nil {
		return false, 0, fmt.Errorf("check lockout exists: %w", err)
	}

	if exists == 0 {
		return false, 0, nil
	}

	ttl, err := rl.redis.TTL(ctx, lockoutKey).Result()
	if err != nil {
		return false, 0, fmt.Errorf("get lockout ttl: %w", err)
	}
	if ttl < 0 {
		ttl = 0
	}

	return true, ttl, nil
}
