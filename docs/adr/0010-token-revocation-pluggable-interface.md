# ADR 0010 — Token revocation: pluggable interface, Redis reference implementation

## Status
Accepted

## Context
A.1a and A.1b (ADRs 0008, 0009) both explicitly deferred
revocation: a leaked access or refresh token remains valid until
its own natural expiration, with no way to invalidate it early.
That gap is now A.1c's job.

Revocation requires external state — something to check on every
token validation to answer "was this specific token invalidated
before its expiration?" This is the first point where GoFast would
depend on infrastructure outside the framework itself.

Two things need deciding: what granularity revocation operates at,
and how the dependency on external storage is structured so it
doesn't become an unconditional dependency of the core framework.

## Decision

**Granularity: per-token, keyed by JTI.** Every token issued by
`IssueAccessToken`/`IssueRefreshToken` now carries a `"jti"` claim
(a UUID, unique per token). Revocation targets a specific token by
its JTI — not "all of user X's tokens" in this ADR. Per-user
mass-revocation (e.g. "user changed their password, kill every
session") is a real, useful feature, but it composes on top of
per-token revocation rather than replacing it, and is left for a
future ADR to avoid scope creep here.

**Interface, defined in the core package:**

```go
type TokenRevoker interface {
    IsRevoked(ctx context.Context, jti string) (bool, error)
    Revoke(ctx context.Context, jti string, expiresAt time.Time) error
}
```

`AuthMiddleware` and `RefreshHandler` both accept an optional
`TokenRevoker`. A `nil` revoker preserves A.1a/A.1b's exact
existing behavior — this is strictly additive, not a breaking
change to the `Handler[In, Out]` contract or to either existing
auth entry point.

**Dependency structure: the interface lives in `gofast/` (core,
zero new dependencies). A concrete, production-ready Redis
implementation lives in the separate importable package
`gofast/revocation/redis`.** Anyone not using revocation pulls in
neither Redis nor any revocation code. Anyone who wants Redis-backed
revocation imports the subpackage explicitly. This mirrors the
standard Go pattern of `database/sql` plus separately-imported
drivers — not a new idea invented here, an established one applied
consistently with how the rest of GoFast already avoids imposing
dependencies (see `benchmarks/` as an isolated module in ADR 0007's
Repository layout, same principle).

**The Redis implementation is real, not illustrative.** It uses
`SETEX`-equivalent semantics: `Revoke` sets a key with a TTL equal
to the token's remaining lifetime, so revoked-token records expire
from Redis on their own — no unbounded growth, no manual cleanup
job required. `IsRevoked` is a single key lookup.

## Alternatives considered

- **Revocation state as an in-memory map, shipped as GoFast's only
  implementation**
  Rejected outright. An in-memory map does not survive a process
  restart and does not work across more than one server instance —
  it would not function correctly in any real production deployment
  with more than a single, never-restarted process. Shipping it as
  *the* answer, with real storage left to the reader, would be
  documenting a limitation instead of solving the problem GoFast
  set out to solve. If a trivial in-memory implementation is useful
  at all, it belongs in test helpers, not as the production-facing
  example.

- **Bundling Redis as a direct dependency of `gofast/` itself**
  Rejected — this would force every consumer of GoFast, including
  those never using revocation, to pull in a Redis client
  transitively. Directly contradicts VISION.md's "minimal
  abstractions" principle and ADR 0006's precedent of avoiding
  imposed infrastructure.

- **Per-user revocation instead of per-token**
  Rejected for this ADR specifically — see Decision above. Not
  rejected as a future feature, just out of scope here to keep this
  change reviewable and testable as one coherent unit.

## Consequences

**Gained:**
- Tokens can now be invalidated before their natural expiration,
  closing the gap A.1a/A.1b explicitly left open.
- The core framework gains zero new dependencies; Redis is opt-in.
- The Redis implementation is genuinely usable in production, not
  a stand-in that still leaves the real work to the adopter.

**Sacrificed:**
- Revocation checks now require a network round-trip to Redis on
  every protected request (when a revoker is configured) — a real
  latency cost, in direct tension with A.1a's original stateless,
  zero-external-calls design. This is an accepted, deliberate
  trade: revocation is opt-in specifically because not every
  deployment needs to pay this cost.
- Per-user mass revocation is not yet possible; a compromised
  password still requires revoking each active token individually
  until a future ADR adds that capability.

**Risk accepted consciously:**
- If `TokenRevoker.IsRevoked` fails (e.g. Redis is unreachable),
  the middleware must choose fail-open or fail-closed. This ADR
  specifies fail-closed (treat a revocation-check error as "token
  invalid", not "token valid") — availability of the revocation
  check is treated as a security-relevant dependency once
  configured, consistent with GoFast's existing "safe by default"
  posture for errors (see ADR on `BusinessError`).