# GoFast — Vision

## What GoFast is

GoFast brings FastAPI's developer experience to Go: write a
struct with tags, get automatic validation, request binding, and
(eventually) OpenAPI documentation — without writing that
boilerplate by hand.

## Why this, and not another framework

Go already has a framework aiming at the same goal: Huma. Huma
achieves automatic validation and OpenAPI generation using
reflection at request time — the cost is paid on every single
HTTP request, for the life of the server.

GoFast pays that cost once, at build time, using `go/ast` to parse
source code and generate real, readable Go files (see ADR 0001 and
ADR 0003). The generated code is not hidden behind a framework
abstraction — it is a file in the developer's own repository, in
their own git diff, auditable line by line.

## Principles

- **Compile-time safety over runtime reflection.** Validation,
  path binding, and query binding are generated once, at build
  time. The framework does not inspect struct tags on every
  request.
- **No magic.** Generated code is real Go, committed to the
  developer's repository. Nothing happens that cannot be read.
- **Minimal abstractions.** GoFast wraps `net/http` directly. No
  custom router engine, no proprietary request/response types.
- **The framework provides structure, not opinions about your
  domain.** GoFast standardizes HTTP concerns (error format,
  status codes, middleware chaining). It does not decide what a
  "valid session" or a "healthy database" means for your
  application — you do.
- **Errors are safe by default.** Internal errors are logged and
  never leak implementation details to API clients unless a
  developer explicitly marks an error as safe to expose
  (`BusinessError`).

## Why GoFast should exist alongside Gin, Echo, and Huma

Gin and Echo solve routing and middleware well, but leave
validation, binding, and documentation entirely manual — the exact
boilerplate FastAPI eliminated for Python.

Huma solves that boilerplate, but pays for it in runtime
reflection on every request, and its generated behavior lives
inside the framework rather than in code the developer can read.

GoFast's bet is narrow and specific: the same boilerplate
elimination as Huma, paid for once at build time instead of on
every request, with the generated result fully visible and
auditable. If that bet does not hold up under real benchmarks
(see Phase 5 of the roadmap), this document — and ADR 0001 — will
be revisited honestly, not defended past the evidence.

## What GoFast is not (yet, and maybe not ever)

- Not a full application framework (no ORM, no templating, no
  admin panel).
- Not a claim of maximum performance in the abstract — it is a
  specific bet about where reflection cost is paid.
- Not API-stable yet. GoFast is pre-v1.0.0; breaking changes to
  the core contract (`Handler[In, Out]`) have already happened
  twice during development, and may happen again before v1.0.0.