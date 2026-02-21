package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryLimiter_Allow_UnderLimit(t *testing.T) {
	limiter := NewInMemoryLimiter()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, err := limiter.Allow(ctx, "test-key", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i)
		}
		if result.Remaining != 10-i-2 && i < 9 {
			t.Logf("remaining at request %d: %d", i, result.Remaining)
		}
	}
}

func TestInMemoryLimiter_Allow_OverLimit(t *testing.T) {
	limiter := NewInMemoryLimiter()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, err := limiter.Allow(ctx, "test-key", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}

	result, err := limiter.Allow(ctx, "test-key", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("request should be denied (over limit)")
	}
	if result.Remaining != 0 {
		t.Errorf("remaining should be 0, got %d", result.Remaining)
	}
}

func TestInMemoryLimiter_Allow_DifferentKeys(t *testing.T) {
	limiter := NewInMemoryLimiter()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		limiter.Allow(ctx, "key1", 5)
	}

	result, _ := limiter.Allow(ctx, "key1", 5)
	if result.Allowed {
		t.Error("key1 should be rate limited")
	}

	result, _ = limiter.Allow(ctx, "key2", 5)
	if !result.Allowed {
		t.Error("key2 should be allowed (different key)")
	}
}

func TestInMemoryLimiter_Allow_ResultFields(t *testing.T) {
	limiter := NewInMemoryLimiter()
	ctx := context.Background()

	result, err := limiter.Allow(ctx, "test-key", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Limit != 100 {
		t.Errorf("Limit = %d, want 100", result.Limit)
	}

	if result.Remaining != 99 {
		t.Errorf("Remaining = %d, want 99", result.Remaining)
	}

	if result.ResetAt.Before(time.Now()) {
		t.Error("ResetAt should be in the future")
	}

	if result.ResetAt.After(time.Now().Add(2 * time.Minute)) {
		t.Error("ResetAt should be within 2 minutes")
	}
}

func TestInMemoryLimiter_RateLimiterInterface(t *testing.T) {
	var _ RateLimiter = (*InMemoryLimiter)(nil)
	var _ RateLimiter = (*Limiter)(nil)
}
