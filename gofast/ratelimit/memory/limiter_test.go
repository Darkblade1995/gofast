package memory

import (
	"context"
	"testing"
	"time"
)

func TestLimiter_AllowsWithinBurst(t *testing.T) {
	l := New(1, 3) // 1 req/sec sustained, burst of 3

	for i := 0; i < 3; i++ {
		allowed, err := l.Allow(context.Background(), "client-a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should have been allowed within burst", i+1)
		}
	}
}

func TestLimiter_RejectsBeyondBurst(t *testing.T) {
	l := New(1, 2) // burst of 2

	for i := 0; i < 2; i++ {
		if allowed, _ := l.Allow(context.Background(), "client-b"); !allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	allowed, err := l.Allow(context.Background(), "client-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected the request beyond burst to be rejected")
	}
}

func TestLimiter_KeysAreIndependent(t *testing.T) {
	l := New(1, 1) // burst of 1

	if allowed, _ := l.Allow(context.Background(), "client-c"); !allowed {
		t.Fatal("first request for client-c should be allowed")
	}
	if allowed, _ := l.Allow(context.Background(), "client-c"); allowed {
		t.Fatal("second request for client-c should be rejected")
	}

	// A different key must have its own, unaffected bucket.
	if allowed, _ := l.Allow(context.Background(), "client-d"); !allowed {
		t.Error("client-d should be unaffected by client-c's limit")
	}
}

func TestLimiter_EvictStaleRemovesOldEntries(t *testing.T) {
	l := New(1, 1)
	l.Allow(context.Background(), "stale-client")

	l.mu.Lock()
	l.buckets["stale-client"].lastSeenAt = time.Now().Add(-time.Hour)
	l.mu.Unlock()

	l.evictStale(10 * time.Minute)

	l.mu.Lock()
	_, exists := l.buckets["stale-client"]
	l.mu.Unlock()
	if exists {
		t.Error("expected stale entry to be evicted")
	}
}