package ratelimit

import (
	"context"
	"sync"
	"time"

	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/redis/go-redis/v9"
)

// Limiter is a fixed-window counter compatible with the existing Redis gateway pattern.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) error
}

// RedisLimiter uses INCR + EXPIRE, matching the gateway sliding-window counter.
type RedisLimiter struct {
	client *redis.Client
}

func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	if client == nil {
		return nil
	}
	return &RedisLimiter{client: client}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	if l == nil || l.client == nil {
		return domainErr.New(domainErr.ErrInternal, "authentication rate limiter unavailable", nil)
	}
	if limit <= 0 {
		return nil
	}
	if window <= 0 {
		window = time.Minute
	}
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "authentication rate limiter unavailable", err)
	}
	if count == 1 {
		if err := l.client.Expire(ctx, key, window).Err(); err != nil {
			return domainErr.New(domainErr.ErrInternal, "authentication rate limiter unavailable", err)
		}
	}
	if count > int64(limit) {
		return limited()
	}
	return nil
}

type memoryBucket struct {
	count int
	exp   time.Time
}

// MemoryLimiter is a process-local fallback used by tests and Redis-less development.
type MemoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]memoryBucket
	now     func() time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{buckets: make(map[string]memoryBucket), now: func() time.Time { return time.Now() }}
}

func (l *MemoryLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) error {
	if l == nil {
		return domainErr.New(domainErr.ErrInternal, "authentication rate limiter unavailable", nil)
	}
	if limit <= 0 {
		return nil
	}
	if window <= 0 {
		window = time.Minute
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, ok := l.buckets[key]
	if !ok || !now.Before(bucket.exp) {
		l.buckets[key] = memoryBucket{count: 1, exp: now.Add(window)}
		return nil
	}
	bucket.count++
	l.buckets[key] = bucket
	if bucket.count > limit {
		return limited()
	}
	return nil
}

func limited() error {
	return domainErr.NewWithCode(domainErr.ErrRateLimited, "authentication_rate_limited", "too many authentication requests", nil)
}
