# GoFast

**A Go framework that generates request validation, binding, and
OpenAPI documentation at build time — via `go/ast` parsing, not
runtime reflection.**

Write a struct with tags. GoFast reads it once, at build time, and
generates real, readable Go code that validates, binds, and
documents your API. Nothing is decided at request time that could
have been decided at compile time.

---

## Table of contents

- [The problem](#the-problem)
- [The bet](#the-bet)
- [Quick start](#quick-start)
- [How it works](#how-it-works)
- [Architecture](#architecture)
- [Feature matrix](#feature-matrix)
- [Benchmarks](#benchmarks)
- [Design decisions (ADRs)](#design-decisions-adrs)
- [Project status](#project-status)
- [Why not just use Huma?](#why-not-just-use-huma)
- [Repository layout](#repository-layout)
- [License](#license)

---

## The problem

Go has strong HTTP routers — Gin, Echo, Chi — and at least one
framework that automates validation and OpenAPI generation the way
FastAPI does for Python: [Huma](https://github.com/danielgtaylor/huma).

None of them generate that automation **at build time**, as real
files you can open, read, and audit. They either require you to
write validation and binding by hand, or they resolve it through
reflection on every incoming request.


                Gin / Echo / Chi              Huma                GoFast
                ────────────────         ──────────────      ──────────────



request arrives │ │ │
▼ ▼ ▼
validation write it reflect over struct call generated
by hand on every request Validate()
│ │ │
▼ ▼ ▼
cost paid per per once, at
request request build time


## The bet

GoFast's bet is narrow and specific: eliminate the same boilerplate
Huma eliminates, but pay the reflection cost once — at build time —
instead of on every request, and keep the generated result fully
visible in your own repository instead of hidden inside the
framework.

type LoginInput struct {
Email string validate:"required,email"
}
│
▼
gofast generate reads your struct via go/ast,
never via reflection (ADR 0003)
│
▼
main_validate.gen.go real Go code, committed to your repo,
readable, auditable — nothing hidden
│
▼
func (in LoginInput) Validate() error {
if in.Email == "" {
return fmt.Errorf("Email is required")
}
...
}


This is not a theoretical claim — it is measured. See
[Benchmarks](#benchmarks).

## Quick start

```go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"gofast/gofast"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
	Token string `json:"token"`
}

func Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	return LoginOutput{Token: "token-for-" + in.Email}, nil
}

func main() {
	router := gofast.NewRouter()
	router.Register("POST", "/login", gofast.Handler(Login))

	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("server running on :8080")
	log.Fatal(server.ListenAndServe())
}
```

Generate the validation code for that struct:

```bash
go run ./cmd/gofast generate ./main.go
```

This writes a real `main_validate.gen.go` next to your code.
`LoginInput` now implements `Validate()` automatically, and
`gofast.Handler` picks it up without any extra wiring.

## How it works

### Three tags, three generated behaviors

| Tag | Generates | Example |
|---|---|---|
| `validate:"..."` | `Validate() error` | `validate:"required,email"` |
| `path:"..."` | `BindPath(map[string]string)` | `path:"id"` |
| `query:"..."` | `BindQuery(url.Values)` | `query:"page,default=1"` |

```go
type GetAccountInput struct {
	ID string `path:"id"`
}

type ListTransactionsInput struct {
	Page  int `query:"page,default=1"`
	Limit int `query:"limit,default=10"`
}
```

### Validation rules supported today

| Rule | Applies to | Behavior |
|---|---|---|
| `required` | any field | rejects empty/zero value |
| `email` | string | checks for `@` |
| `min=N` / `max=N` | string | length bounds |
| `dive,<rule>` | `[]string` | applies `<rule>` to every element |
| `dive_key,<rule>` | `map[string]V` | applies `<rule>` to every key |
| nested struct / `*struct` / `[]struct` / `map[K]struct` | any field whose type also has `Validate()` | recurses automatically, at any depth |

### The build-time verification loop

edit struct
│
▼
gofast generate writes/updates .gen.go files,
│ updates .gofast/manifest.json
│ (SHA256 hash of the source file)
▼
forget to regenerate?
│
▼
gofast generate --check exits non-zero, fails CI —
│ a stale generation cannot
│ silently ship
▼
go test ./... TestGeneratedCodeIsUpToDate
fails locally too, not just in CI


## Architecture

┌──────────────────────────────────────────────────────────────┐
│ Your application │
│ main.go — structs with json/validate/path/query tags │
└───────────────────────────┬────────────────────────────────────┘
│
gofast generate
│
┌──────────────┴──────────────┐
▼ ▼
internal/codegen/ cmd/gofast/
(not importable (the CLI:
outside this generate,
module) --check)
┌─────────────────┐
│ parse.go │ go/ast struct parser, no reflection
│ tag.go │ validate:/path:/query: tag parsing
│ generate.go │ Validate() generation
│ generate_bind.go │ BindPath()/BindQuery() generation
│ manifest.go │ SHA256 change detection
│ project.go │ go.mod-based root lookup
└─────────────────────────────────┘
│
▼ writes
*_validate.gen.go, *_bindpath.gen.go, *_bindquery.gen.go
(real files, committed to your repo)
│
▼ used by
┌──────────────────────────────────────────────────────────────┐
│ gofast/ (the public framework) │
│ │
│ HandlerIn, Out ─────► HandlerInfo{Func, InType, OutType}│
│ │ │
│ ▼ │
│ decode JSON → bind path → bind query → validate → call fn │
│ │ │
│ ▼ │
│ Router.Register() ──────► Route{Method, Path, InType, OutType}│
│ │ │
│ Router.ServeOpenAPI() ───► reads all Routes, generates spec │
│ Router.ServeSwaggerUI() ─► serves interactive docs │
│ │
│ Logger · Recovery · CORS · HealthCheck (middleware) │
│ BusinessError vs internal errors (safe-by-default) │
└──────────────────────────────────────────────────────────────┘



## Feature matrix

| Area | Capability | Status |
|---|---|---|
| Core contract | `Handler[In, Out]`, generics-based | done |
| Binding | path params, query params (with defaults) | done |
| Binding | request body via JSON, `DisallowUnknownFields` | done |
| Validation | required, email, min/max, nested, slice/map dive | done |
| Codegen | AST-based, zero reflection, hash-verified manifest | done |
| Codegen | build-time staleness detection (`--check`) | done |
| Middleware | Logger, Recovery (panic-safe), CORS | done |
| Middleware | Health checks (`router.HealthCheck`) | done |
| Errors | structured, typed, safe-by-default (`BusinessError`) | done |
| Docs | OpenAPI 3.0 generation, Swagger UI | done |
| Security | request body size limits, Content-Type validation | done |
| Engineering | `golangci-lint`, `govulncheck`, `-race`, all clean | done |
| Engineering | fuzz-tested AST parser (2M+ execs, 0 panics) | done |
| Engineering | CPU/memory profiling documented | done |
| Streaming responses | — | not built (see roadmap) |
| Multipart/form-data | — | not built (see roadmap) |
| Dependency injection container | — | intentionally not built (ADR 0006) |

Full detail, including everything explicitly out of scope and why:
[`docs/ROADMAP.md`](docs/ROADMAP.md).

## Benchmarks

Measured against Huma v2.39.1, four scenarios, full methodology
and reproduction commands in [`docs/BENCHMARKS.md`](docs/BENCHMARKS.md).

| Scenario | GoFast | Huma | Advantage |
|---|---|---|---|
| Isolated validation (simple struct) | 2.123 ns/op, 0 allocs | 79.53 ns/op, 3 allocs | ~37.5x |
| Isolated validation (nested + slice) | 4.367 ns/op, 0 allocs | 85.69 ns/op, 3 allocs | ~19.6x |
| Full HTTP cycle (sequential) | 4400 ns/op, 39 allocs | 6549 ns/op, 53 allocs | ~1.49x |
| Full HTTP cycle (concurrent, 12 threads) | 2715 ns/op, 39 allocs | 2850 ns/op, 53 allocs | ~1.05x |

The isolated-validation advantage narrows sharply in a real
request, because JSON decoding and Go's own runtime (allocator,
garbage collector) dominate the cost budget for both frameworks
equally. CPU profiling confirms this directly: GoFast's own
generated code does not appear among the top 15 CPU consumers in
a full HTTP cycle — the runtime and JSON encoding do.

The one advantage that holds at every level measured: roughly
**26% fewer allocations**, consistent from isolated validation
through concurrent HTTP load.

```bash
cd benchmarks
go test -bench=BenchmarkGoFastHTTPCycle -benchmem ./...
go test -bench=BenchmarkHumaHTTPCycle -benchmem ./...
```

## Design decisions (ADRs)

Every non-obvious architectural choice is documented as an ADR,
including the ones that constrain what GoFast can honestly claim:

| ADR | Decision |
|---|---|
| [0001](docs/adr/0001-build-time-codegen-vs-runtime-reflection.md) | Build-time codegen instead of runtime reflection |
| [0002](docs/adr/0002-router-registration-not-concurrency-safe.md) | Route registration is startup-only, not concurrency-safe |
| [0003](docs/adr/0003-ast-parsing-not-reflect-for-codegen.md) | The generator parses `go/ast`, never uses `reflect` |
| [0004](docs/adr/0004-reflection-allowed-for-openapi-not-request-validation.md) | Reflection is allowed once, at route registration, for OpenAPI — not per request |
| [0005](docs/adr/0005-semantic-versioning-policy-pre-v1.md) | Semantic versioning policy before v1.0.0 |
| [0006](docs/adr/0006-dependency-injection-via-explicit-closures.md) | Dependency injection via explicit closures, no container |
| [0007](docs/adr/0007-migration-path-from-gin-echo-huma.md) | Migration path from Gin/Echo/Huma via `http.Handler` mounting |

## Project status

GoFast is **pre-v1.0.0**. Its versioning policy (ADR 0005) is
explicit: minor versions may still break the core contract until
the API has proven stable across two consecutive releases. The
`Handler[In, Out]` contract has already changed twice during
development — once to add `context.Context`, once to carry
reflected type information for OpenAPI.

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the complete,
continuously updated state of every phase, including known risks
and design decisions still open.

## Why not just use Huma?

Huma already provides FastAPI-style automatic validation and
OpenAPI generation for Go, and is a mature, well-adopted project.
GoFast is not trying to replace it — it tests a narrower
architectural bet: that the same automation can cost nothing at
request time, because the work happens once, at build time, in
code that stays visible in your own repository.

The evidence for that bet — where it holds, and where it does
not — is in [`docs/BENCHMARKS.md`](docs/BENCHMARKS.md). The
explicit limits of what GoFast claims are in
[`docs/VISION.md`](docs/VISION.md).

## Repository layout

gofast/ the public framework package
handler.go Handler[In, Out], the core contract
router.go Router, functional options, config
middleware.go Logger, Recovery, CORS
errors.go error types, BusinessError
health.go HealthCheck
openapi.go OpenAPI 3.0 generation
docs.go Swagger UI serving

internal/codegen/ the code generator (not importable
outside this module)

cmd/gofast/ the CLI (gofast generate [--check])

benchmarks/ isolated module measuring GoFast
against Huma (own go.mod, replace
directive — Huma is never a
dependency of GoFast itself)

examples/minimal/ a working example, also used as the
generator's own integration test target

docs/
adr/ architecture decision records
VISION.md project philosophy and limits
ROADMAP.md live status of every phase
BENCHMARKS.md reproducible performance
evidence, methodology,
and profiling results


## License

MIT — see [LICENSE](LICENSE) for details.