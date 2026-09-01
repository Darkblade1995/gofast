# ADR 0012 — Lifespan hooks: Router.Run built on net/http's graceful shutdown

## Status
Accepted

## Context
GoFast has no place today for a developer to run code at startup
(verify a database connection before accepting traffic) or at
shutdown (drain in-flight requests, close a Redis client cleanly).
This gap is visible concretely in examples/minimal: redisClient is
a package-level variable that is never explicitly closed, and there
is no fail-fast check that Redis is reachable before the server
starts serving.

Go's net/http already provides real graceful shutdown via
http.Server.Shutdown(ctx) — it stops accepting new connections and
waits for in-flight requests to finish. GoFast does not need to
reimplement that. What is missing is a place for the *application*
to hook into that lifecycle: run its own setup before traffic is
accepted, and its own cleanup after the server stops accepting new
requests.

## Decision

**Two registration methods on Router**, symmetric to the existing
Register():

```go
func (r *Router) OnStartup(hook func(ctx context.Context) error)
func (r *Router) OnShutdown(hook func(ctx context.Context) error)
```

Startup hooks run, in registration order, before the server begins
accepting traffic. If any startup hook returns an error, the server
never starts — this is a deliberate fail-fast: an application whose
database is unreachable should not accept requests it cannot serve
correctly.

Shutdown hooks run, in reverse registration order (LIFO — the last
resource acquired is the first released, mirroring defer semantics
developers already know from Go), after http.Server.Shutdown(ctx)
has finished draining in-flight requests.

**A new optional convenience method:**

```go
func (r *Router) Run(ctx context.Context, addr string) error
```

`Run` orchestrates the full lifecycle: run startup hooks, start
http.Server in a goroutine, block until ctx is canceled or an
OS interrupt/terminate signal is received, call
http.Server.Shutdown(ctx) to drain in-flight requests, then run
shutdown hooks.

**Using Run is optional.** An application may continue constructing
its own *http.Server and calling ListenAndServe manually, exactly
as examples/minimal already does — Run does not replace that path,
it adds a second one for applications that want the lifecycle
handled for them. This is consistent with VISION.md: GoFast wraps
net/http directly and does not invent a custom server; Run is a
thin orchestration convenience over the standard library's own
Shutdown, not a new server implementation.

## Alternatives considered

- **Building a custom graceful-shutdown mechanism instead of using
  http.Server.Shutdown**
  Rejected outright — the standard library already solves this
  correctly. Reimplementing it would be exactly the kind of
  "custom abstraction over net/http" ADR 0001 and VISION.md already
  reject.

- **Making Run mandatory (removing the manual http.Server path)**
  Rejected — this would be a breaking change to how every existing
  example and adopter already wires GoFast, for no benefit; the
  manual path already works and some applications legitimately need
  more control over server configuration (custom TLS config,
  multiple listeners) than a single Run helper can reasonably
  expose.

- **Running shutdown hooks in registration order instead of LIFO**
  Rejected — LIFO matches the mental model Go developers already
  have from defer, and is the correct order when hooks have
  dependencies (a hook that closes a connection pool should run
  before, not after, a hook that assumes that pool is still open).

## Consequences

**Gained:**
- Applications can fail fast at startup instead of accepting
  traffic they cannot correctly serve.
- Resources (database connections, Redis clients, background
  workers) have a defined, ordered place to be cleaned up on
  shutdown.
- Router.Run gives applications that want it a one-line path to
  correct graceful shutdown, without forcing it on those that don't.

**Sacrificed:**
- None of substance — this is purely additive; no existing
  Router method's behavior changes.

**Risk accepted consciously:**
- If a startup hook hangs (e.g. an unreachable database with no
  timeout in the hook's own context), Run will hang too — GoFast
  does not impose a timeout on startup hooks, matching context.Context's own idiom: the caller passes a context with whatever
  deadline is appropriate, and hooks are expected to respect it.