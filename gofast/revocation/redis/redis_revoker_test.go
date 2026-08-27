// These are integration tests: they require a real Redis instance
// reachable at the address below. They are not run as pure unit
// tests because TokenRevoker's entire purpose is to talk to Redis
// correctly — mocking that away would test nothing meaningful.
package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func newTestRevoker(t *testing.T) (*TokenRevoker, *goredis.Client) {
	t.Helper()

	client := goredis.NewClient(&goredis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping: no Redis reachable at localhost:6379: %v", err)
	}

	return New(client), client
}

func TestTokenRevoker_NotRevokedByDefault(t *testing.T) {
	revoker, client := newTestRevoker(t)
	defer client.Close()

	jti := "test-jti-not-revoked"
	defer client.Del(context.Background(), keyPrefix+jti)

	revoked, err := revoker.IsRevoked(context.Background(), jti)
	if err != nil {
		t.Fatalf("IsRevoked returned an error: %v", err)
	}
	if revoked {
		t.Error("expected a never-revoked jti to report revoked=false")
	}
}

func TestTokenRevoker_RevokeThenIsRevoked(t *testing.T) {
	revoker, client := newTestRevoker(t)
	defer client.Close()

	jti := "test-jti-revoke-flow"
	defer client.Del(context.Background(), keyPrefix+jti)

	err := revoker.Revoke(context.Background(), jti, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	revoked, err := revoker.IsRevoked(context.Background(), jti)
	if err != nil {
		t.Fatalf("IsRevoked returned an error: %v", err)
	}
	if !revoked {
		t.Error("expected jti to be revoked after Revoke, got revoked=false")
	}
}

func TestTokenRevoker_RevokeWithPastExpiryDoesNotFail(t *testing.T) {
	// See the ttl <= 0 handling in Revoke: a token revoked at or
	// after its own expiration must still succeed, using a minimum
	// TTL instead of an invalid non-positive one.
	revoker, client := newTestRevoker(t)
	defer client.Close()

	jti := "test-jti-past-expiry"
	defer client.Del(context.Background(), keyPrefix+jti)

	err := revoker.Revoke(context.Background(), jti, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Revoke with a past expiresAt should not fail, got: %v", err)
	}

	revoked, err := revoker.IsRevoked(context.Background(), jti)
	if err != nil {
		t.Fatalf("IsRevoked returned an error: %v", err)
	}
	if !revoked {
		t.Error("expected jti to be revoked even with a past expiresAt")
	}
}

func TestTokenRevoker_KeyHasTTLSet(t *testing.T) {
	// Confirms the revocation record expires on its own — no
	// unbounded growth in Redis, no manual cleanup job required.
	revoker, client := newTestRevoker(t)
	defer client.Close()

	jti := "test-jti-ttl-check"
	defer client.Del(context.Background(), keyPrefix+jti)

	err := revoker.Revoke(context.Background(), jti, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	ttl, err := client.TTL(context.Background(), keyPrefix+jti).Result()
	if err != nil {
		t.Fatalf("TTL check failed: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("expected a positive TTL on the revocation key, got %v", ttl)
	}
	if ttl > time.Hour {
		t.Errorf("expected TTL to be at most 1 hour, got %v", ttl)
	}
}