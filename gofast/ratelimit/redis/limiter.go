// Package redis provides a multi-instance gofast.RateLimiter
// backed by Redis, using a fixed-window counter (INCR + EXPIRE).
// It is a separate, optional package: importing gofast/gofast does
// not pull in this package or its Redis client dependency. See ADR
// 0011 for the full design and the trade-offs of fixed-window
// versus a true distributed token bucket.
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const keyPrefix = "gofast:ratelimit:"

// Limiter implements gofast.RateLimiter using a Redis-backed
// fixed-window counter, shared across every GoFast instance
// pointed at the same Redis. See ADR 0011.
type Limiter struct {
	client *goredis.Client
	limit  int64
	window time.Duration
}

// New wraps an existing *goredis.Client, allowing up to limit
// requests per key within each window-sized interval. The caller
// owns the client's lifecycle, consistent with GoFast's
// explicit-closures approach to dependencies (see ADR 0006).
func New(client *goredis.Client, limit int64, window time.Duration) *Limiter {
	return &Limiter{client: client, limit: limit, window: window}
}

// Allow reports whether key may proceed within its current window.
// A Redis error is returned as-is; RateLimitMiddleware treats any
// error here as fail-open — see ADR 0011.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	windowStart := time.Now().Truncate(l.window).Unix()
	redisKey := fmt.Sprintf("%s%s:%d", keyPrefix, key, windowStart)

	count, err := l.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		// First request in this window for this key — set the
		// expiration so the counter cleans itself up. A small
		// buffer beyond the window itself covers clock skew
		// between this check and Redis's own clock.
		if err := l.client.Expire(ctx, redisKey, l.window+time.Second).Err(); err != nil {
			return false, err
		}
	}

	return count <= l.limit, nil
}