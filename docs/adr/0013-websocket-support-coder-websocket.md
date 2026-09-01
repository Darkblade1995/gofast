# ADR 0013 — WebSocket support via coder/websocket, a separate handler contract

## Status
Accepted

## Context
GoFast's entire request model to date — Handler[In, Out],
generated via codegen, decode → bind → validate → call → encode —
assumes a single request produces a single response, then the
connection ends. WebSockets do not fit that shape: after an HTTP
upgrade, the connection stays open indefinitely and either side may
send messages at any time, with no fixed request/response pairing.
Forcing Handler[In, Out] onto that shape would misrepresent what a
WebSocket connection actually is.

Two things need deciding: which WebSocket library to depend on, and
what shape a WebSocket handler takes in GoFast's API.

**Library choice, based on current (2026) state, not assumption:**
gorilla/websocket, historically the de facto standard, is archived
and no longer actively maintained. coder/websocket (the maintained
continuation of nhooyr.io/websocket, taken over by Coder in 2024)
is actively maintained, has zero dependencies of its own, and uses
context.Context natively for cancellation — the same idiom already
threading through every other part of GoFast (Handler, auth,
revocation, rate limiting, lifespan hooks).

## Decision

**Dependency: `github.com/coder/websocket`.**

**A new, separate handler type**, not a variant of
`Handler[In, Out]`:

```go
type WSHandler func(ctx context.Context, conn *websocket.Conn) error
```

Where `*websocket.Conn` is coder/websocket's own connection type,
re-exported as-is — GoFast does not wrap or reimplement the
WebSocket protocol itself; it only integrates the library into
GoFast's existing route-registration pattern.

**A new registration method, parallel to `Register`, not replacing
it:**

```go
func (r *Router) RegisterWS(path string, handler WSHandler)
```

Internally, `RegisterWS` performs the HTTP upgrade via
`websocket.Accept`, then hands the resulting connection to
`handler`, running in its own goroutine per connection (idiomatic
Go concurrency, one goroutine blocks on reads for that connection's
lifetime — no callback chains).

**Middleware composability is preserved**: because `RegisterWS`
still registers against the same underlying `*http.ServeMux` via
`Router`, existing per-route middleware composition
(`HandlerInfo.Wrap`, `AuthMiddleware`, `RateLimitMiddleware`) can
still run *before* the upgrade happens — a WebSocket endpoint can
require authentication or rate-limit the upgrade attempt using
tools GoFast already has, without new middleware machinery.

## Alternatives considered

- **gorilla/websocket**
  Rejected — archived, no longer actively maintained. Depending on
  it would mean building new functionality on top of an
  unmaintained foundation, the same category of risk GoFast has
  avoided elsewhere (e.g. choosing golang-jwt/v5 over the older,
  less current v3).

- **Extending Handler[In, Out] to somehow support WebSockets**
  Rejected — the fundamental request/response shape doesn't apply
  to a connection that stays open and exchanges messages in both
  directions indefinitely. Forcing it would either produce an
  awkward, misleading API, or require compromising the contract
  that makes Handler[In, Out] correct for everything it already
  handles.

- **A generic "long-lived connection" abstraction covering both
  WebSockets and future streaming (Fase C)**
  Rejected for now — premature generalization before a second
  concrete case (streaming) exists to generalize from. Fase C's
  streaming responses may or may not share enough shape with
  WebSocket handling to justify a shared abstraction; that
  decision is deferred to when Fase C is actually designed, not
  guessed at here.

## Consequences

**Gained:**
- Real-time, bidirectional communication (chat, live dashboards,
  notifications) is possible without any GoFast-side abstraction
  standing between the developer and coder/websocket's own API.
- Existing per-route middleware (auth, rate limiting) composes
  with WebSocket routes for free, at the upgrade step.

**Sacrificed:**
- WebSocket routes do not appear in GenerateOpenAPI's output —
  OpenAPI 3.0 has no native representation for a WebSocket
  endpoint's message protocol. This is a known, accepted gap, not
  silently ignored: documented here and to be reflected in the
  roadmap.
- No codegen involvement for WebSocket message payloads — unlike
  Handler[In, Out], there is no generated Validate()/BindPath() for
  messages read off a WebSocket connection. An application needing
  that must decode and validate manually, or use coder/websocket's
  own `wsjson` helpers.

**Risk accepted consciously:**
- Each open WebSocket connection holds a goroutine for its
  lifetime. An application with many concurrent long-lived
  connections needs to size its deployment accordingly — this is
  an inherent property of WebSockets in Go's concurrency model, not
  something GoFast can abstract away, and is worth stating
  explicitly so it isn't discovered by surprise in production.