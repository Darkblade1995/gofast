package gofast

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

var testSecret = []byte("test-secret-do-not-use-in-production")

// fakeRevoker is a minimal, in-memory TokenRevoker test double.
// It exists only to exercise AuthMiddleware/RefreshHandler's
// revocation logic in unit tests — it is not presented anywhere
// as a production implementation. The real one lives in
// gofast/revocation/redis (see ADR 0010).
type fakeRevoker struct {
	mu       sync.Mutex
	revoked  map[string]bool
	forceErr bool
}

func newFakeRevoker() *fakeRevoker {
	return &fakeRevoker{revoked: map[string]bool{}}
}

func (f *fakeRevoker) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if f.forceErr {
		return false, errors.New("simulated revocation-check failure")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revoked[jti], nil
}

func (f *fakeRevoker) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked[jti] = true
	return nil
}

// newProtectedHandler returns a handler wrapped by AuthMiddleware
// that reports whether it was reached, and captures the Claims it
// saw via ClaimsFromContext.
func newProtectedHandler(reached *bool, gotClaims *Claims, revoker TokenRevoker) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		if claims, ok := ClaimsFromContext(r.Context()); ok {
			*gotClaims = claims
		}
		w.WriteHeader(http.StatusOK)
	})
	return AuthMiddleware(testSecret, revoker)(inner)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, nil)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached without a token")
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, nil)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token abc123") // wrong scheme
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached with a malformed header")
	}
}

func TestAuthMiddleware_InvalidSignature(t *testing.T) {
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, nil)

	wrongSecret := []byte("a-different-secret-entirely")
	token, err := IssueAccessToken(wrongSecret, "user-123", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached with an invalid signature")
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, nil)

	token, err := IssueAccessToken(testSecret, "user-123", -time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached with an expired token")
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, nil)

	token, err := IssueAccessToken(testSecret, "user-123", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !reached {
		t.Fatal("handler should have been reached with a valid token")
	}
	if claims.Subject != "user-123" {
		t.Errorf("expected claims.Subject %q, got %q", "user-123", claims.Subject)
	}
}

func TestAuthMiddleware_RejectsRefreshToken(t *testing.T) {
	// A refresh token is validly signed and unexpired, but must
	// still be rejected by AuthMiddleware — see ADR 0009.
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, nil)

	token, err := IssueRefreshToken(testSecret, "user-123", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test refresh token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when presenting a refresh token as an access token, got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached with a refresh token")
	}
}

func TestAuthMiddleware_RevokedToken(t *testing.T) {
	// See ADR 0010. A validly signed, unexpired, correctly-typed
	// token must still be rejected once its jti is revoked.
	revoker := newFakeRevoker()

	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, revoker)

	token, err := IssueAccessToken(testSecret, "user-123", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test token: %v", err)
	}

	parsed, err := parseToken(context.Background(), testSecret, token, tokenTypeAccess, nil)
	if err != nil {
		t.Fatalf("failed to parse test token to extract jti: %v", err)
	}
	if err := revoker.Revoke(context.Background(), parsed.JTI, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to revoke test token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a revoked token, got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached with a revoked token")
	}
}

func TestAuthMiddleware_RevocationCheckErrorFailsClosed(t *testing.T) {
	// See ADR 0010's fail-closed decision: if the revocation store
	// is unreachable, the token must be treated as invalid, not
	// valid.
	revoker := newFakeRevoker()
	revoker.forceErr = true

	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims, revoker)

	token, err := IssueAccessToken(testSecret, "user-123", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when the revocation check itself errors (fail-closed), got %d", rec.Code)
	}
	if reached {
		t.Error("handler should not have been reached when the revocation check errors")
	}
}

func TestClaimsFromContext_NoClaimsPresent(t *testing.T) {
	_, ok := ClaimsFromContext(context.Background())
	if ok {
		t.Error("expected ok=false when no claims are present in context")
	}
}

func TestRefreshHandler_ValidRefreshToken(t *testing.T) {
	refreshToken, err := IssueRefreshToken(testSecret, "user-123", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test refresh token: %v", err)
	}

	handler := RefreshHandler(testSecret, time.Hour, nil)

	body := `{"refresh_token":"` + refreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Func(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "access_token") {
		t.Errorf("expected response to contain access_token, got: %s", rec.Body.String())
	}
}

func TestRefreshHandler_RejectsAccessToken(t *testing.T) {
	// Presenting an access token where a refresh token is expected
	// must fail — same principle as TestAuthMiddleware_RejectsRefreshToken,
	// in the opposite direction.
	accessToken, err := IssueAccessToken(testSecret, "user-123", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test access token: %v", err)
	}

	handler := RefreshHandler(testSecret, time.Hour, nil)

	body := `{"refresh_token":"` + accessToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Func(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when presenting an access token as a refresh token, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshHandler_ExpiredRefreshToken(t *testing.T) {
	expiredRefreshToken, err := IssueRefreshToken(testSecret, "user-123", -time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test refresh token: %v", err)
	}

	handler := RefreshHandler(testSecret, time.Hour, nil)

	body := `{"refresh_token":"` + expiredRefreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Func(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with an expired refresh token, got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshHandler_RevokedRefreshToken(t *testing.T) {
	// See ADR 0010 — revocation applies to refresh tokens too.
	revoker := newFakeRevoker()

	refreshToken, err := IssueRefreshToken(testSecret, "user-123", 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test refresh token: %v", err)
	}

	parsed, err := parseToken(context.Background(), testSecret, refreshToken, tokenTypeRefresh, nil)
	if err != nil {
		t.Fatalf("failed to parse test refresh token to extract jti: %v", err)
	}
	if err := revoker.Revoke(context.Background(), parsed.JTI, time.Now().Add(7*24*time.Hour)); err != nil {
		t.Fatalf("failed to revoke test refresh token: %v", err)
	}

	handler := RefreshHandler(testSecret, time.Hour, revoker)

	body := `{"refresh_token":"` + refreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Func(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a revoked refresh token, got %d, body: %s", rec.Code, rec.Body.String())
	}
}