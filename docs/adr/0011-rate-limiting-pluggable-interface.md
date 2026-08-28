# ADR 0011 — Rate limiting: pluggable interface, in-memory and Redis implementations

## Status
Accepted

## Context
GoFast has no protection today against a client sending unlimited
requests — most critically to `/login`, which is otherwise open to
brute-force credential attempts. Rate limiting is Fase A.2, the
next unaddressed gap for production readiness.

Two design questions: what key a limit is applied to, and where
the limiter's state lives.

**Keying is a policy decision, not mechanism** — see VISION.md:
"GoFast standardizes HTTP concerns... It does not decide what a
'valid session' means for your application." Whether to limit by
IP, by authenticated user, or by API key depends on the route and
the application's own concerns (a public `/login` endpoint has no
authenticated user to key on; an authenticated route may reasonably
prefer per-user limiting over per-IP). GoFast provides a sensible
default (by IP) so the common case requires no code from the
application, and an override point for cases that need something
else — consistent with how ADR 0006 already treats DI.

**State storage has the same shape as ADR 0010's revocation
decision**, and gets the same answer: build both a real in-memory
implementation and a real Redis implementation now, not one now and
a vague "later" for the other.

## Decision

**Interface, defined in the core package:**

```go
type RateLimiter interface {
    Allow(ctx context.Context, key string) (bool, error)
}
```

**Middleware:**

```go
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler
```

`keyFunc` may be `nil`, in which case GoFast uses a default that
keys on the request's remote IP (`r.RemoteAddr`, stripped of port).
Applications needing per-user or per-API-key limiting supply their
own `keyFunc` — typically a few lines reading `ClaimsFromContext`
or a header, not a rate-limiting algorithm.

**Two real implementations, both built now:**

- `gofast/ratelimit/memory` — token-bucket limiting via
  `golang.org/x/time/rate`, one bucket per key, held in a
  mutex-guarded map with periodic cleanup of stale entries. Correct
  and sufficient for a single-process deployment; not a stand-in
  for something better, a complete answer for that deployment
  shape.
- `gofast/ratelimit/redis` — the same token-bucket semantics
  enforced against Redis (via `INCR` + `EXPIRE`, a fixed-window
  counter — see Alternatives for why token-bucket-in-Redis was not
  chosen), so multiple GoFast instances behind a load balancer
  share one limit instead of each enforcing its own.

Both live outside `gofast/` (core), mirroring ADR 0010's dependency
structure: importing `gofast/gofast` pulls in neither
`golang.org/x/time/rate` nor a Redis client unless the application
imports the specific subpackage it wants.

**Response on limit exceeded:** `429 Too Many Requests`, using the
existing `BusinessError`/`writeError` machinery — a new
`ErrCodeRateLimited` is added to the error taxonomy, consistent
with `ErrCodeUnauthorized` from ADR 0008.

## Alternatives considered

- **Token bucket implemented from scratch against Redis (matching
  the in-memory algorithm exactly)**
  Rejected in favor of a Redis-native fixed-window counter
  (`INCR`/`EXPIRE`). A true distributed token bucket requires a Lua
  script or transaction to stay race-free under concurrent
  requests, adding real complexity for a marginal precision gain
  over a fixed window at the traffic volumes GoFast is likely to
  see. This mirrors the same pragmatism as ADR 0010's SETEX-based
  revocation store — correct and simple over theoretically-purer
  and complex.

- **No default keyFunc, always require the application to supply
  one**
  Rejected — this would make rate limiting require boilerplate for
  the common case (limit by IP), contradicting GoFast's purpose of
  eliminating exactly that kind of repetitive setup.

- **Baking a specific keying policy (e.g. always per-user) into
  GoFast itself**
  Rejected — see Decision above; this is a policy question VISION.md
  already reserves for the application.

## Consequences

**Gained:**
- `/login` and any other route can be protected from brute-force
  and flooding, with a working default requiring zero
  application code.
- Both single-process and multi-instance deployments have a real,
  non-toy implementation available.

**Sacrificed:**
- The in-memory limiter's state is per-process — a client
  distributed across many requests hitting different instances of
  a horizontally-scaled deployment is only limited per-instance
  unless the Redis implementation is used instead.
- The Redis fixed-window counter allows brief bursts at window
  boundaries larger than a true sliding window would — an accepted,
  standard trade-off for its simplicity (same class of trade-off
  already accepted for A.1c's revocation TTL semantics).

**Risk accepted consciously:**
- As with ADR 0010, a `RateLimiter.Allow` error should be handled
  deliberately by the caller. This ADR specifies fail-open here —
  unlike revocation, where fail-closed protects a security
  invariant, an unreachable rate-limit store failing open degrades
  gracefully to "temporarily unlimited" rather than blocking
  legitimate traffic entirely. This is the opposite choice from
  ADR 0010 and is made deliberately, not by default: rate limiting
  protects availability, revocation protects identity — the two
  have different failure-mode priorities.