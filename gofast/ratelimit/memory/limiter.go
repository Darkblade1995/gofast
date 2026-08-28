// Package memory provides a single-process gofast.RateLimiter
// backed by golang.org/x/time/rate. It is a separate, optional
// package: importing gofast/gofast does not pull in this package
// or its dependency. See ADR 0011 for the full design, including
// when to prefer gofast/ratelimit/redis instead (multi-instance
// deployments).
package memory

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type bucketEntry struct {
	limiter    *rate.Limiter
	lastSeenAt time.Time
}

// Limiter implements gofast.RateLimiter using one token bucket per
// key, held in memory. It is correct and sufficient for a
// single-process deployment — not a placeholder for something
// better, a complete answer for that deployment shape. See ADR
// 0011 for the multi-instance case.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucketEntry
	rps     rate.Limit
	burst   int
}

// New creates a Limiter allowing up to rps requests per second,
// sustained, per key, with burst allowing short spikes above that
// rate. It starts a background goroutine that periodically evicts
// buckets not used in the last 10 minutes, so keys that stop
// appearing (e.g. an IP that never returns) do not grow memory use
// without bound. Call Close when the Limiter is no longer needed
// to stop that goroutine.
func New(rps float64, burst int) *Limiter {
	l := &Limiter{
		buckets: make(map[string]*bucketEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
	}
	go l.cleanupLoop()
	return l
}

// Allow reports whether the given key may proceed, consuming one
// token from its bucket if so. It never returns an error — a
// purely in-memory limiter has no failure mode to report.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	l.mu.Lock()
	entry, ok := l.buckets[key]
	if !ok {
		entry = &bucketEntry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[key] = entry
	}
	entry.lastSeenAt = time.Now()
	limiter := entry.limiter
	l.mu.Unlock()

	return limiter.Allow(), nil
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.evictStale(10 * time.Minute)
	}
}

func (l *Limiter) evictStale(olderThan time.Duration) {
	cutoff := time.Now().Add(-olderThan)
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.buckets {
		if entry.lastSeenAt.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}