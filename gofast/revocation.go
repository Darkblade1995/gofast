package gofast

import (
	"context"
	"time"
)

// TokenRevoker allows GoFast's auth middleware and refresh handler
// to check whether a specific token has been invalidated before
// its natural expiration, and to record a revocation. GoFast's
// core package defines only this interface — no revocation
// backend is a dependency of gofast/ itself. See ADR 0010 for a
// production-ready Redis implementation in gofast/revocation/redis,
// imported separately by applications that need revocation.
//
// A nil TokenRevoker passed to AuthMiddleware or RefreshHandler
// disables revocation checking entirely, preserving the exact
// stateless behavior of A.1a/A.1b (ADRs 0008, 0009).
type TokenRevoker interface {
	// IsRevoked reports whether the token identified by jti has
	// been revoked. An error here is treated as "revoked" by
	// callers — see ADR 0010's fail-closed decision.
	IsRevoked(ctx context.Context, jti string) (bool, error)

	// Revoke records jti as revoked. expiresAt should match the
	// token's own expiration, so implementations backed by a
	// TTL-capable store (e.g. Redis) can let the revocation record
	// expire on its own instead of growing without bound.
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
}