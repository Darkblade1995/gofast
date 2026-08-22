package gofast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-do-not-use-in-production")

// generateToken builds a signed JWT for test purposes. Passing a
// different secret than testSecret produces a token with an
// invalid signature; passing a past expiresAt produces an expired
// token.
func generateToken(t *testing.T, secret []byte, subject string, expiresAt time.Time) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub": subject,
		"exp": expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

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
	token := generateToken(t, wrongSecret, "user-123", time.Now().Add(time.Hour))

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

	token := generateToken(t, testSecret, "user-123", time.Now().Add(-time.Hour))

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

	token := generateToken(t, testSecret, "user-123", time.Now().Add(time.Hour))

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

func TestClaimsFromContext_NoClaimsPresent(t *testing.T) {
	_, ok := ClaimsFromContext(context.Background())
	if ok {
		t.Error("expected ok=false when no claims are present in context")
	}
}