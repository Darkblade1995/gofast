# ADR 0006 — Dependency injection via explicit closures

## Status
Accepted

## Context
Business functions registered with `gofast.Handler` need access to
dependencies — database connections, caches, external clients —
that don't fit into the `func(context.Context, In) (Out, error)`
signature `Handler` requires. GoFast needs an explicit, documented
answer for how a developer supplies these dependencies, rather
than leaving it as an unaddressed gap.

## Decision
GoFast does not provide a dependency injection container or
mechanism. The supported pattern is an explicit closure: the
developer writes a constructor function that takes the
dependencies and returns a function matching Handler's expected
signature.

```go
func NewLoginHandler(db *sql.DB) func(context.Context, LoginInput) (LoginOutput, error) {
    return func(ctx context.Context, in LoginInput) (LoginOutput, error) {
        // db is available here via closure capture
        ...
    }
}

router.Register("POST", "/login", gofast.Handler(NewLoginHandler(db)))
```

This requires no changes to `Handler`, `Router`, or any other part
of the core contract.

## Alternatives considered

- **A DI container (wire, fx, dig-style)**
  Rejected. This is the kind of hidden resolution mechanism
  VISION.md explicitly argues against — "no magic," generated code
  visible in the developer's own repository. A DI container
  resolves dependencies through reflection or generated wiring
  code that lives outside the developer's direct view. It also
  adds a real dependency and a new concept surface GoFast doesn't
  need to own.

- **context.WithValue for dependencies**
  Rejected as the primary pattern. It preserves Handler's current
  signature, but requires a type assertion (`ctx.Value(dbKey).(*sql.DB)`)
  in every handler that needs a dependency — this loses Go's
  compile-time type safety, which VISION.md lists as a core
  principle. It remains available as a standard library mechanism
  developers can use directly if they choose, but GoFast does not
  build around it or recommend it.

## Consequences

**Gained:**
- Zero changes to the core `Handler`/`Router` contract.
- Full compile-time type safety — no type assertions, no runtime
  surprises about missing dependencies.
- Consistent with Go idiom: this is how dependencies are typically
  wired in stdlib-based Go services without a framework.

**Sacrificed:**
- No automatic dependency resolution — the developer writes the
  constructor function by hand for every handler that needs
  dependencies. This is boilerplate GoFast does not eliminate,
  unlike the validation/binding boilerplate the code generator
  removes.

**Risk accepted consciously:**
- If a project ends up with many handlers needing many shared
  dependencies, this pattern can become repetitive. If that proves
  to be a real, common pain point once GoFast has real usage, it
  may be revisited with a new ADR — but is not solved speculatively
  here, consistent with how GoFast has avoided building
  unrequested abstractions elsewhere (see the plugin system and
  RFC process discussion earlier in the project).