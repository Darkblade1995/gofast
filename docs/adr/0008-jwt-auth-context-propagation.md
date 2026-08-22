# ADR 0008 — JWT auth: stateless validation and context propagation

## Status
Accepted

## Context
GoFast has no authentication mechanism today (see feature gap
audit — auth middleware is Fase A.1, the highest-priority gap for
production use). Almost no real service ships without some form
of request authentication, so this is the first Fase A item to be
built.

Two design questions have to be settled before any code is
written, because both are hard to change later without breaking
compatibility:

1. How does a handler access the verified identity, once a request
   has passed authentication?
2. What distinguishes "no token", "malformed token", and "expired
   or invalid signature" — these are different failure modes and
   collapsing them into one generic 401 hides useful information
   from API clients.

This ADR deliberately covers only stateless JWT validation: a
token is either cryptographically valid and unexpired, or it is
not. It does not cover refresh tokens or revocation/blacklisting —
see Scope below for why those are excluded on purpose.

## Decision

**Library:** `github.com/golang-jwt/jwt/v5` — the de facto standard
for JWT handling in Go.

**Identity propagation:** verified claims are stored in
`context.Context` using a package-private, typed key — never a
plain string key. This avoids context key collisions with other
middleware (a well-known Go footgun) and is consistent with how
GoFast already threads `context.Context` through
`Handler[In, Out]`.

```go
type claimsContextKey struct{}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(Claims)
	return claims, ok
}
```

**Failure modes, distinguished explicitly:**

| Condition                          | Response |
|-------------------------------------|----------|
| No `Authorization` header           | 401 — no credentials supplied |
| Malformed header (not `Bearer <t>`) | 401 — no credentials supplied |
| Token signature invalid             | 401 — invalid credentials |
| Token expired                       | 401 — invalid credentials |
| Token valid, but claims insufficient (future: role checks) | 403 — authenticated but not authorized |

401 vs 403 is a real distinction, not cosmetic: 401 means "I don't
know who you are," 403 means "I know who you are, and it's not
enough." Fase A.1a only produces 401s — 403 is reserved here for
future authorization logic (roles/scopes), not built yet.

All failures are returned as GoFast's existing `BusinessError`
type (see ADR on safe-by-default errors) — no new error type is
introduced.

## Scope

**Covers (A.1a, this ADR):**
- Parsing and validating a JWT from the `Authorization: Bearer`
  header
- Signature verification and expiration checking
- Propagating verified claims into `context.Context`

**Explicitly does NOT cover:**
- **Refresh tokens (A.1b):** requires a new `/refresh` endpoint,
  a distinct expiration policy from the access token, and its own
  storage question. Deferred to its own ADR so it does not get
  decided as a side effect of this one.
- **Revocation / blacklisting (A.1c):** JWT is stateless by design
  — checking "was this token revoked?" requires querying external
  state (e.g. Redis) on every request, which is the first time
  GoFast would depend on infrastructure outside the framework
  itself. That is a large enough decision to deserve its own ADR,
  not to be folded into basic auth.

Bundling all three into one change would force three unrelated
decisions (context propagation, refresh storage, revocation
storage) into a single review, and would block a working, usable
JWT middleware on decisions that don't need to be made yet.

## Alternatives considered

- **String-based context keys** (e.g. `ctx.Value("claims")`)
  Rejected — collides silently with any other middleware using the
  same string key, with no compile-time or runtime warning. A
  typed unexported key makes collision structurally impossible.

- **Returning a generic 401 for every failure mode**
  Rejected — collapses "you sent nothing" and "you sent something
  invalid" into the same signal, which is strictly less useful to
  API clients debugging integration issues, for no simplification
  benefit on GoFast's side.

- **Building refresh/revocation now, alongside basic validation**
  Rejected — see Scope above.

## Consequences

**Gained:**
- A working, production-usable JWT middleware with no external
  state dependency.
- Clear, typed access to identity from any handler via
  `ClaimsFromContext`.
- Precise failure signaling (401 vs 403 reserved for future use).

**Sacrificed:**
- No token revocation — a compromised token remains valid until
  it naturally expires. This is a real, accepted limitation of
  A.1a, not an oversight.
- No refresh flow — clients must handle re-authentication when the
  access token expires, until A.1b exists.

**Risk accepted consciously:**
- Because there is no revocation, access token expiration time
  becomes a meaningful security parameter for anyone adopting
  A.1a alone. This should be called out in the middleware's
  documentation, not just in this ADR.
