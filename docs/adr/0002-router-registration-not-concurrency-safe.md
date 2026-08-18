# ADR 0002 — Router.Register() is not safe for concurrent use

## Status
Accepted

## Context
`Router.Register()` appends to an internal slice (`r.routes`) and
reads `r.config` without any synchronization primitive (no mutex,
no atomic operations). This is safe as long as all calls to
`Register()` happen sequentially, before the server starts
accepting traffic — which is the typical usage pattern shown in
every example so far:

    router := gofast.NewRouter(...)
    router.Register("POST", "/login", ...)
    router.Register("GET", "/accounts/{id}", ...)
    http.ListenAndServe(":8080", handler)

If a developer called `Register()` from multiple goroutines, or
dynamically after the server had already started serving requests,
concurrent writes to `r.routes` would be a data race — detectable
by Go's `-race` flag, and a real source of corruption or panics
under load.

## Decision
GoFast does not add synchronization to `Router.Register()`.
Route registration is documented as a startup-only operation:
all routes must be registered before the router is passed to
`http.ListenAndServe()` (or any middleware chain that begins
serving traffic). Registering routes concurrently, or after the
server has started, is unsupported and will not be protected
against.

## Alternatives considered

- **Add a `sync.Mutex` around `Register()`**
  Rejected for V1. It would protect against the data race, but
  it does not solve the deeper problem: `http.ServeMux` itself
  does not support safely adding routes after `ServeHTTP` has
  begun handling requests in most usage patterns, and masking
  that with a mutex could give developers false confidence that
  dynamic route registration is a supported pattern, when it
  is not the design GoFast targets.

- **Support dynamic route registration as a first-class feature**
  Rejected for V1. This is a legitimate feature for some use
  cases (plugin systems, hot-reloading routes), but it is a
  significant design surface of its own — out of scope for the
  current core contract. May be revisited in a future ADR if a
  real use case demands it.

## Consequences

**Gained:**
- Simpler, faster `Register()` — no locking overhead on a path
  that, by design, only runs once at startup.
- A clear, honest contract: GoFast's core routing is startup-only,
  matching how nearly all Go HTTP frameworks (`net/http` itself
  included) expect routes to be configured.

**Sacrificed:**
- No built-in support for dynamic or hot-reloaded routes. A
  developer needing that must build their own layer on top of
  GoFast, or wait for a future ADR that addresses it directly.

**Risk accepted consciously:**
- If a developer ignores this documented constraint and calls
  `Register()` concurrently or after serving has begun, GoFast
  will not detect or prevent it. This is a silent risk unless the
  developer runs their own code with `go test -race` or similar
  tooling. This should be stated clearly in the package's public
  documentation (godoc), not just here.