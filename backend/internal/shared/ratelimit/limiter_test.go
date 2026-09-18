package ratelimit

import (
	"context"
	"testing"
	"time"

	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestMemoryLimiterAllowsThenBlocks(t *testing.T) {
	limiter := NewMemoryLimiter()
	ctx := context.Background()
	if err := limiter.Allow(ctx, "challenge:ip:127.0.0.1", 2, time.Minute); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	if err := limiter.Allow(ctx, "challenge:ip:127.0.0.1", 2, time.Minute); err != nil {
		t.Fatalf("second allow: %v", err)
	}
	err := limiter.Allow(ctx, "challenge:ip:127.0.0.1", 2, time.Minute)
	if err == nil || !isRateLimited(err) {
		t.Fatalf("third allow = %v, want rate limited", err)
	}
	if err := limiter.Allow(ctx, "challenge:ip:10.0.0.2", 2, time.Minute); err != nil {
		t.Fatalf("other key should remain independent: %v", err)
	}
}

func TestMemoryLimiterWindowReset(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	limiter := NewMemoryLimiter()
	limiter.now = func() time.Time { return now }
	ctx := context.Background()
	if err := limiter.Allow(ctx, "verify:ip:1", 1, time.Minute); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	if err := limiter.Allow(ctx, "verify:ip:1", 1, time.Minute); err == nil || !isRateLimited(err) {
		t.Fatalf("expected limit inside window, got %v", err)
	}
	limiter.now = func() time.Time { return now.Add(time.Minute) }
	if err := limiter.Allow(ctx, "verify:ip:1", 1, time.Minute); err != nil {
		t.Fatalf("window reset should allow: %v", err)
	}
}

func isRateLimited(err error) bool {
	return err != nil && domainErr.HTTPStatusCode(err) == 429 && domainErr.ErrorCode(err) == "authentication_rate_limited"
}
