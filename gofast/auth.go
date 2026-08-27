package gofast

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the minimal set of verified JWT claims GoFast
// cares about. Applications needing custom claims should embed
// this struct or parse the token themselves using the same secret.
type Claims struct {
	Subject string
	Expires int64
	JTI     string
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

// tokenType distinguishes access tokens from refresh tokens within
// the "type" JWT claim. See ADR 0009 for why this exists: without
// it, a leaked refresh token could be presented directly to
// protected routes as if it were an access token.
type tokenType string

const (
	tokenTypeAccess  tokenType = "access"
	tokenTypeRefresh tokenType = "refresh"
)

// generateJTI returns a random 128-bit hex-encoded token
// identifier, unique per issued token. Used so ADR 0010's
// TokenRevoker can target a specific token for revocation without
// needing the token's full signature as a lookup key.
func generateJTI() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// IssueAccessToken creates a signed access token for the given
// subject, valid for ttl. Access tokens are meant to be sent on
// every request to a protected route (see AuthMiddleware).
func IssueAccessToken(secret []byte, subject string, ttl time.Duration) (string, error) {
	return issueToken(secret, subject, tokenTypeAccess, ttl)
}

// IssueRefreshToken creates a signed refresh token for the given
// subject, valid for ttl (typically much longer than an access
// token's). A refresh token is only valid against RefreshHandler
// — AuthMiddleware rejects it. See ADR 0009 for the stateless,
// no-rotation design of A.1b, and ADR 0010 for how a TokenRevoker
// can invalidate a specific token by its jti before this
// expiration is reached.
func IssueRefreshToken(secret []byte, subject string, ttl time.Duration) (string, error) {
	return issueToken(secret, subject, tokenTypeRefresh, ttl)
}

func issueToken(secret []byte, subject string, typ tokenType, ttl time.Duration) (string, error) {
	jti, err := generateJTI()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":  subject,
		"type": string(typ),
		"jti":  jti,
		"exp":  time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// parseToken validates signature and expiration, confirms the
// "type" claim matches expected, and — if revoker is non-nil —
// confirms the token's jti has not been revoked. Shared by
// AuthMiddleware and RefreshHandler so both enforce identical
// checks and differ only in which token type they accept.
//
// See ADR 0010: a revocation-check error is treated as "invalid",
// not "valid" — fail-closed, not fail-open.
func parseToken(ctx context.Context, secret []byte, rawToken string, expected tokenType, revoker TokenRevoker) (Claims, error) {
	token, err := jwt.Parse(rawToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return Claims{}, jwt.ErrTokenInvalidClaims
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, jwt.ErrTokenInvalidClaims
	}

	typ, _ := mapClaims["type"].(string)
	if tokenType(typ) != expected {
		return Claims{}, jwt.ErrTokenInvalidClaims
	}

	claims := Claims{}
	if sub, ok := mapClaims["sub"].(string); ok {
		claims.Subject = sub
	}
	if exp, ok := mapClaims["exp"].(float64); ok {
		claims.Expires = int64(exp)
	}
	if jti, ok := mapClaims["jti"].(string); ok {
		claims.JTI = jti
	}

	if revoker != nil {
		revoked, err := revoker.IsRevoked(ctx, claims.JTI)
		if err != nil || revoked {
			return Claims{}, jwt.ErrTokenInvalidClaims
		}
	}

	return claims, nil
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if header == "" || !strings.HasPrefix(header, prefix) {
		return "", false
	}
	return strings.TrimPrefix(header, prefix), true
}

// AuthMiddleware returns an http middleware that validates access
// tokens signed with HMAC using the given secret. revoker is
// optional — pass nil to disable revocation checking entirely,
// preserving A.1a/A.1b's original stateless behavior. See ADR 0008
// for the failure-mode table, ADR 0009 for why refresh tokens are
// also rejected here, and ADR 0010 for revocation.
func AuthMiddleware(secret []byte, revoker TokenRevoker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := bearerToken(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "missing or malformed Authorization header", nil)
				return
			}

			claims, err := parseToken(r.Context(), secret, rawToken, tokenTypeAccess, revoker)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "invalid, expired, revoked, or wrong-type token", nil)
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RefreshInput is the request body RefreshHandler expects.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshOutput is the response body RefreshHandler returns: a new
// access token only. See ADR 0009 — the refresh token itself is
// not rotated or reissued in A.1b.
type RefreshOutput struct {
	AccessToken string `json:"access_token"`
}

// RefreshHandler returns a ready-to-register HandlerInfo that
// exchanges a valid, unexpired, non-revoked refresh token for a
// new access token. accessTTL controls the lifetime of the newly
// issued access token. revoker is optional — pass nil to disable
// revocation checking. See ADR 0009 for why this does not accept
// access tokens and does not rotate the refresh token, and ADR
// 0010 for revocation.
func RefreshHandler(secret []byte, accessTTL time.Duration, revoker TokenRevoker) HandlerInfo {
	return Handler(func(ctx context.Context, in RefreshInput) (RefreshOutput, error) {
		claims, err := parseToken(ctx, secret, in.RefreshToken, tokenTypeRefresh, revoker)
		if err != nil {
			return RefreshOutput{}, NewBusinessError(
				ErrCodeUnauthorized,
				http.StatusUnauthorized,
				"invalid, expired, revoked, or wrong-type refresh token",
			)
		}

		accessToken, err := IssueAccessToken(secret, claims.Subject, accessTTL)
		if err != nil {
			return RefreshOutput{}, NewBusinessError(
				ErrCodeInternal,
				http.StatusInternalServerError,
				"failed to issue access token",
			)
		}

		return RefreshOutput{AccessToken: accessToken}, nil
	})
}