# ADR 0009 — Refresh tokens: stateless, no rotation

## Status
Accepted

## Context
A.1a (ADR 0008) gives GoFast stateless access token validation,
but no way to renew an expired access token without forcing the
user through full credential re-authentication (email + password).
Almost every real client — web or mobile — expects a refresh flow
so a user's session survives beyond the access token's short
lifetime without repeatedly asking for a password.

Two designs were considered for how the refresh token itself is
validated: stateless (a signed JWT, same mechanism as the access
token) or stateful (a token tracked in external storage — a
database or Redis — so it can be looked up, rotated, and revoked
on demand).

## Decision

**The refresh token is a stateless, HMAC-signed JWT** — the same
signing mechanism as the access token, distinguished by a
`"type":"refresh"` claim, with a longer expiration (days, not
hours; the exact duration is left to the application via
`AuthConfig`, not hardcoded in GoFast).

```go
type refreshClaims struct {
    Subject string
    Type    string // always "refresh"
    Expires int64
}
```

**`POST /refresh` behavior:** the client sends its refresh token;
GoFast validates its signature, expiration, and `type` claim, then
issues a **new access token only**. The refresh token itself is
**not rotated** — the same refresh token remains valid, and usable
again, until its own expiration.

**Why no rotation in A.1b specifically:** rotation without
server-side storage cannot actually invalidate the previous
refresh token — there is nothing to check it against. Rotating the
token string while leaving the old one just as valid (because
nothing tracks which one is "current") would look like a security
improvement without being one. Real rotation — where reusing an
old, rotated-out refresh token is detectably wrong — requires
tracking issued tokens, which is explicitly A.1c's job (see ADR
0008's Scope section on why revocation gets its own ADR).

## Alternatives considered

- **Stateful refresh tokens with storage now, in A.1b**
  Rejected — this is A.1c's decision, not A.1b's. Bundling storage
  into the refresh flow would force the storage-backend question
  (Redis? SQL table? in-memory?) to be answered before a basic,
  usable refresh flow exists, blocking a working feature on an
  infrastructure decision that doesn't need to be made yet.

- **Refresh token rotation without storage**
  Rejected — see Decision above. A false sense of security is
  worse than an honestly-scoped stateless design; the limitation
  is documented instead of hidden behind rotation theater.

- **No distinguishing claim between access and refresh tokens
  (rely on expiration length alone)**
  Rejected — without a `type` claim, a leaked long-lived refresh
  token could be presented directly to protected routes expecting
  an access token, since both would otherwise be structurally
  identical JWTs signed with the same secret. The `type` claim
  lets `AuthMiddleware` (A.1a) and the new `/refresh` handler each
  reject tokens meant for the other's purpose.

## Consequences

**Gained:**
- Users stay logged in across access token expirations without
  re-entering credentials, with zero new infrastructure.
- Consistent mechanism with A.1a — same secret, same signing
  method, one mental model for both token types.

**Sacrificed:**
- A leaked refresh token remains valid for its full lifetime; naming
  it `type:"refresh"` prevents it from being *misused* as an access
  token, but does not let GoFast revoke it early. This is the same
  accepted risk A.1a already documented for access tokens, now
  extended to a longer-lived token — which makes the refresh
  token's expiration length a more consequential security
  parameter than the access token's.
- No detection of refresh token reuse after a hypothetical future
  rotation — because there is no rotation in A.1b, there is nothing
  to detect reuse against yet.

**Risk accepted consciously:**
- Applications adopting A.1b alone should choose a refresh token
  expiration deliberately short enough to bound the damage of a
  leak, understanding that A.1c (revocation) is the real fix for
  this — not a workaround, its documented successor.