package gofast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("test-secret-do-not-use-in-production")

// newProtectedHandler returns a handler wrapped by AuthMiddleware
// that reports whether it was reached, and captures the Claims it
// saw via ClaimsFromContext.
func newProtectedHandler(reached *bool, gotClaims *Claims) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		if claims, ok := ClaimsFromContext(r.Context()); ok {
			*gotClaims = claims
		}
		w.WriteHeader(http.StatusOK)
	})
	return AuthMiddleware(testSecret)(inner)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	reached := false
	var claims Claims
	handler := newProtectedHandler(&reached, &claims)

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
	handler := newProtectedHandler(&reached, &claims)

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
	handler := newProtectedHandler(&reached, &claims)

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
	handler := newProtectedHandler(&reached, &claims)

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
	handler := newProtectedHandler(&reached, &claims)

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
	handler := newProtectedHandler(&reached, &claims)

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

	handler := RefreshHandler(testSecret, time.Hour)

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

	handler := RefreshHandler(testSecret, time.Hour)

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

	handler := RefreshHandler(testSecret, time.Hour)

	body := `{"refresh_token":"` + expiredRefreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Func(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with an expired refresh token, got %d, body: %s", rec.Code, rec.Body.String())
	}
}