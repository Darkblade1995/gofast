package gofast

import (
	"context"
	"net"
	"net/http"
)

// RateLimiter decides whether a request identified by key is
// allowed to proceed. GoFast's core package defines only this
// interface — no rate-limiting backend is a dependency of gofast/
// itself. See ADR 0011 for real implementations:
// gofast/ratelimit/memory (single-process, token bucket) and
// gofast/ratelimit/redis (multi-instance, fixed-window counter).
type RateLimiter interface {
	// Allow reports whether the request identified by key may
	// proceed. An error here is handled as fail-open by
	// RateLimitMiddleware — see ADR 0011 for why this differs from
	// TokenRevoker's fail-closed behavior (ADR 0010).
	Allow(ctx context.Context, key string) (bool, error)
}

// defaultRateLimitKey extracts the client's IP address (without
// port) from the request as the default rate-limiting key. Used
// when RateLimitMiddleware is given a nil keyFunc. See ADR 0011 —
// keying by anything other than IP (e.g. authenticated user) is a
// policy decision left to the application via a custom keyFunc.
func defaultRateLimitKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware returns an http middleware that rejects
// requests with 429 Too Many Requests once limiter denies them.
// keyFunc determines what identifies a client for limiting
// purposes; pass nil to use defaultRateLimitKey (the request's
// remote IP). See ADR 0011.
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	if keyFunc == nil {
		keyFunc = defaultRateLimitKey
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)

			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				// Fail-open: an unreachable rate-limit store
				// degrades to "temporarily unlimited" rather than
				// blocking legitimate traffic. See ADR 0011.
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				writeError(w, http.StatusTooManyRequests, ErrCodeRateLimited, "rate limit exceeded", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}