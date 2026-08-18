# ADR 0004 — Reflection is allowed for OpenAPI generation, not for
per-request validation

## Status
Accepted

## Context
ADR 0001 established that GoFast avoids `reflect` for request
validation, path binding, and query binding — that work happens
once, at build time, via `go/ast` parsing (see ADR 0003), so no
reflection cost is paid per HTTP request.

Phase 4 introduces OpenAPI spec generation. To build a spec,
GoFast needs to know which `In`/`Out` types are associated with
each registered route. `Handler[In, Out any]` only carries this
type information at compile time inside its own generic
instantiation — by the time a route is registered on `Router`, the
concrete types are not otherwise recoverable without either:

- extending `go/ast` parsing to also read `main.go`'s `Register()`
  calls and match them against `Handler(...)` invocations at the
  source level, or
- using `reflect.TypeOf()` once, at route registration time, to
  capture the `In`/`Out` types directly from the generic
  instantiation.

The AST-parsing approach is possible but significantly more
complex: it would require resolving which function is passed to
`Handler(...)` in arbitrary developer code, including aliases,
wrapped functions, and indirect references — a much harder parsing
problem than reading tags off a struct definition.

## Decision
GoFast uses `reflect.TypeOf()` to capture `In` and `Out` type
information at route registration time (inside `Router.Register`,
when `Handler[In, Out]` is passed in). This reflection call happens
exactly once per registered route, during server startup — never
during request handling.

This does not contradict ADR 0001. ADR 0001's concern was
specifically the cost of reflection paid on every incoming HTTP
request. Route registration happens once, at startup, and its cost
is irrelevant to steady-state request latency — the same
distinction GoFast already draws between build-time code
generation and runtime request handling.

## Alternatives considered

- **Extend go/ast parsing to resolve Handler(...) call sites**
  Rejected for now. Technically possible, but requires resolving
  arbitrary Go expressions passed as arguments (not just reading
  struct tags), which is a much larger parsing surface. May be
  revisited if reflection-at-startup proves insufficient for a
  real use case (e.g. dynamically constructed handlers).

- **Require developers to manually register OpenAPI metadata**
  (e.g. `router.Register("POST", "/login", gofast.Handler(Login),
  gofast.WithOpenAPI(LoginInput{}, LoginOutput{}))`)
  Rejected as the default. This reintroduces exactly the kind of
  manual boilerplate GoFast exists to eliminate. It remains a
  possible escape hatch for edge cases, not the primary mechanism.

## Consequences

**Gained:**
- OpenAPI generation gets accurate type information without
  extending the AST parser's scope significantly.
- The reflection cost is paid once per route, at startup — not a
  meaningful runtime cost, and does not touch the request-handling
  hot path at all.

**Sacrificed:**
- A reader auditing the codebase will find `reflect` imported in
  `gofast/router.go` (or `handler.go`), which could look, at a
  glance, like it contradicts ADR 0001's "no reflection" framing.
  This ADR exists specifically to make that distinction explicit
  and prevent that misunderstanding.

**Risk accepted consciously:**
- If a developer constructs `Handler` calls in ways that defeat
  straightforward `reflect.TypeOf()` inspection (e.g. through
  significant indirection), OpenAPI generation may silently
  produce incomplete or incorrect type information for that route.
  This is not yet detected or warned about — tracked as a known
  limitation to revisit once Phase 4 has a working baseline.