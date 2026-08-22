package gofast

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the minimal set of verified JWT claims GoFast
// cares about. Applications needing custom claims should embed
// this struct or parse the token themselves using the same secret.
type Claims struct {
	Subject string
	Expires int64
}

// claimsContextKey is an unexported type used as a context key.
// This makes key collisions with other middleware structurally
// impossible (see ADR 0008).
type claimsContextKey struct{}

// ClaimsFromContext retrieves the verified JWT claims stored by
// AuthMiddleware. The second return value is false if no claims
// are present — e.g. if called outside an authenticated route.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(Claims)
	return claims, ok
}

// AuthMiddleware returns an http middleware that validates JWTs
// signed with HMAC using the given secret. See ADR 0008 for the
// failure-mode table (401 for missing/malformed/invalid/expired
// tokens; no revocation, no refresh — stateless validation only).
func AuthMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "missing Authorization header", nil)
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				writeError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "Authorization header must use Bearer scheme", nil)
				return
			}
			rawToken := strings.TrimPrefix(header, prefix)

			token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "invalid or expired token", nil)
				return
			}

			mapClaims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "invalid token claims", nil)
				return
			}

			claims := Claims{}
			if sub, ok := mapClaims["sub"].(string); ok {
				claims.Subject = sub
			}
			if exp, ok := mapClaims["exp"].(float64); ok {
				claims.Expires = int64(exp)
			}

			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}