// Package redis provides a production-ready gofast.TokenRevoker
// backed by Redis. It is a separate, optional package: importing
// gofast/gofast does not pull in this package or its Redis client
// dependency. See ADR 0010 for why this lives outside the core
// framework package.
package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// keyPrefix namespaces revocation entries in Redis, so this
// package's keys don't collide with other data an application
// might store in the same Redis instance.
const keyPrefix = "gofast:revoked:"

// TokenRevoker implements gofast.TokenRevoker using Redis. Revoke
// stores a key with a TTL equal to the token's remaining lifetime,
// so revocation records expire on their own — no manual cleanup
// job is needed, and the store does not grow without bound as
// tokens naturally expire.
type TokenRevoker struct {
	client *redis.Client
}

// New wraps an existing *redis.Client. The caller owns the
// client's lifecycle (creation, connection options, and closing
// it on shutdown) — this type does not manage that, consistent
// with GoFast's explicit-closures approach to dependencies (see
// ADR 0006).
func New(client *redis.Client) *TokenRevoker {
	return &TokenRevoker{client: client}
}

// IsRevoked reports whether jti has been revoked. A Redis error
// (including a connectivity failure) is returned as-is; callers
// in gofast/gofast treat any error here as "revoked" — fail
// closed, per ADR 0010.
func (r *TokenRevoker) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := r.client.Exists(ctx, keyPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Revoke marks jti as revoked until expiresAt. If expiresAt is
// already in the past, Revoke stores the key with Redis's minimum
// effective TTL rather than skipping the write, so a token revoked
// moments before its own expiration is still reliably blocked for
// the remainder of its (very short) validity window.
func (r *TokenRevoker) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}
	return r.client.Set(ctx, keyPrefix+jti, "1", ttl).Err()
}