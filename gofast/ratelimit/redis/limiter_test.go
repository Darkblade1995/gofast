// These are integration tests: they require a real Redis instance
// reachable at the address below.
package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T, limit int64, window time.Duration) (*Limiter, *goredis.Client) {
	t.Helper()

	client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping: no Redis reachable at localhost:6379: %v", err)
	}

	return New(client, limit, window), client
}

func TestLimiter_AllowsWithinLimit(t *testing.T) {
	limiter, client := newTestLimiter(t, 3, time.Minute)
	defer client.Close()

	key := "test-key-within-limit"
	defer client.Del(context.Background(), keyPrefix+key+":*")

	for i := 0; i < 3; i++ {
		allowed, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should have been allowed within limit", i+1)
		}
	}
}

func TestLimiter_RejectsBeyondLimit(t *testing.T) {
	limiter, client := newTestLimiter(t, 2, time.Minute)
	defer client.Close()

	key := "test-key-beyond-limit"
	defer client.Del(context.Background(), keyPrefix+key+":*")

	for i := 0; i < 2; i++ {
		if allowed, _ := limiter.Allow(context.Background(), key); !allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	allowed, err := limiter.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected the request beyond the limit to be rejected")
	}
}