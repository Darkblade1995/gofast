# ADR 0007 — Migration path from Gin/Echo/Huma

## Status
Accepted

## Context
Adopting GoFast in an existing production service running Gin,
Echo, or Huma should not require a rewrite. Requiring a "big bang"
migration is a real adoption blocker — this was identified early
in GoFast's design discussion as a critical requirement, not an
afterthought.

## Decision
GoFast requires no special migration mechanism because `Router`
already satisfies `http.Handler` (it implements `ServeHTTP`). This
means a `*gofast.Router` can be mounted as a sub-handler inside an
existing Gin, Echo, Chi, or stdlib `net/http` server, using
whatever sub-routing mechanism that framework already provides.

Example with stdlib `net/http`, mounting GoFast under a path
prefix alongside an existing Gin router:

```go
existingGinRouter := gin.Default()
// ... existing Gin routes, untouched ...

gofastRouter := gofast.NewRouter()
gofastRouter.Register("POST", "/webhooks", gofast.Handler(HandleWebhook))

mux := http.NewServeMux()
mux.Handle("/legacy/", existingGinRouter)
mux.Handle("/v2/", http.StripPrefix("/v2", gofastRouter))

http.ListenAndServe(":8080", mux)
```

Migration becomes incremental: new endpoints are added in GoFast,
one at a time, while existing Gin/Echo/Huma endpoints keep running
unchanged in the same process. There is no forced rewrite and no
point at which the whole service must switch at once.

## Alternatives considered

- **A dedicated GoFast migration CLI or codemod tool**
  Rejected for now. Building an automated Gin-to-GoFast or
  Huma-to-GoFast code transformer is a large, speculative effort
  with no evidence yet that manual mounting (the solution above)
  is insufficient for real migrations. Consistent with how GoFast
  has avoided building tooling ahead of demonstrated need elsewhere
  in this project.

- **A custom GoFast-specific router incompatible with stdlib
  http.Handler**
  Rejected outright, and not just for migration reasons — this was
  already decided in earlier design work (GoFast wraps
  `http.ServeMux` rather than inventing its own routing engine).
  This ADR simply documents the migration benefit that decision
  already provides.

## Consequences

**Gained:**
- Zero-effort compatibility with any framework built on
  `net/http`'s `http.Handler` interface — which includes Gin, Echo,
  Chi, and Huma.
- Migration risk is minimized to a single new endpoint at a time,
  not a full rewrite.

**Sacrificed:**
- No tooling to automatically convert existing Gin/Echo/Huma
  handlers into GoFast's `Handler[In, Out]` shape — that conversion
  remains manual, handler by handler, as a developer chooses to
  migrate each one.

**Risk accepted consciously:**
- This ADR describes a migration PATH, not a migration GUARANTEE.
  It has not yet been exercised against a real Gin or Huma
  codebase — the mounting mechanism relies on `http.Handler`
  compatibility, which is a solid guarantee from the Go standard
  library, but has not been validated end-to-end with an actual
  mixed Gin+GoFast service in this project.