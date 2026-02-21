package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Result contains the rate limit check result.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// Limiter implements rate limiting using Redis sliding window.
type Limiter struct {
	client *redis.Client
	window time.Duration
}

// NewLimiter creates a new rate limiter.
func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{
		client: client,
		window: time.Minute,
	}
}

// Allow checks if a request is allowed under the rate limit.
// Uses a sliding window algorithm with Redis sorted sets.
func (l *Limiter) Allow(ctx context.Context, key string, limit int) (*Result, error) {
	now := time.Now()
	windowStart := now.Add(-l.window)

	pipe := l.client.Pipeline()

	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	countCmd := pipe.ZCard(ctx, key)

	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	pipe.Expire(ctx, key, l.window+time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("redis pipeline failed: %w", err)
	}

	count := countCmd.Val()
	remaining := limit - int(count) - 1
	if remaining < 0 {
		remaining = 0
	}

	result := &Result{
		Allowed:   count < int64(limit),
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   now.Add(l.window),
	}

	if !result.Allowed {
		l.client.ZPopMin(ctx, key)
		result.Remaining = 0
	}

	return result, nil
}

// InMemoryLimiter provides a simple in-memory rate limiter for testing
// or when Redis is not available.
type InMemoryLimiter struct {
	requests map[string][]time.Time
	window   time.Duration
}

// NewInMemoryLimiter creates a new in-memory rate limiter.
func NewInMemoryLimiter() *InMemoryLimiter {
	return &InMemoryLimiter{
		requests: make(map[string][]time.Time),
		window:   time.Minute,
	}
}

// Allow checks if a request is allowed under the rate limit.
func (l *InMemoryLimiter) Allow(ctx context.Context, key string, limit int) (*Result, error) {
	now := time.Now()
	windowStart := now.Add(-l.window)

	timestamps := l.requests[key]
	var validTimestamps []time.Time
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	count := len(validTimestamps)
	allowed := count < limit

	if allowed {
		validTimestamps = append(validTimestamps, now)
	}

	l.requests[key] = validTimestamps

	remaining := limit - count - 1
	if remaining < 0 {
		remaining = 0
	}
	if !allowed {
		remaining = 0
	}

	return &Result{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   now.Add(l.window),
	}, nil
}

// RateLimiter is the interface for rate limiters.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int) (*Result, error)
}
