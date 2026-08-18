# GoFast

**A Go framework that generates request validation, binding, and
OpenAPI documentation at build time — not on every request.**

Write a struct with tags. GoFast reads it once, at build time, and
generates real, readable Go code that validates, binds, and
documents your API — no reflection at request time, nothing hidden.

## The problem

Go has strong HTTP routers (Gin, Echo, Chi) and at least one
framework that automates validation and OpenAPI generation the way
FastAPI does for Python: Huma. What none of them do is generate
that automation at **build time**, as real files you can open and
audit, instead of reflecting over your structs on every incoming
request.

GoFast's bet is specific: the same boilerplate elimination Huma
provides, paid for once at build time instead of on every request,
with the generated result fully visible in your own repository.
See [`docs/adr/0001`](docs/adr/0001-build-time-codegen-vs-runtime-reflection.md)
for the full reasoning and the trade-offs accepted to get there.

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

This writes a real `main_validate.gen.go` file next to your code —
no runtime reflection, no hidden framework machinery. `LoginInput`
now implements `Validate()` automatically, and `gofast.Handler`
picks it up without any extra wiring.

## How it works

type LoginInput struct {
Email string validate:"required,email"
}
|
v
gofast generate (reads your struct via go/ast,
not reflection — see ADR 0003)
|
v
main_validate.gen.go (real Go code, committed to your repo,
readable, auditable — nothing hidden)
|
v
func (in LoginInput) Validate() error {
if in.Email == "" {
return fmt.Errorf("Email is required")
}
...
}


The same pattern generates `BindPath()` from `path:"..."` tags and
`BindQuery()` from `query:"..."` tags — including default values
(`query:"page,default=1"`), nested struct validation, and
per-element validation on slices (`validate:"dive,min=3"`).

A hash-based manifest (`.gofast/manifest.json`) tracks what has
been generated. If you change a struct and forget to regenerate,
`gofast generate --check` fails loudly, and is designed to be
wired into CI so a stale generation cannot silently ship.

## OpenAPI, generated from the same source of truth

Every registered route is documented automatically:

```go
router.ServeOpenAPI("/openapi.json", "My API", "1.0.0")
router.ServeSwaggerUI("/docs", "/openapi.json")
```

Visiting `/docs` renders an interactive Swagger UI, generated from
the same `reflect.Type` information captured once at route
registration. See [`docs/adr/0004`](docs/adr/0004-reflection-allowed-for-openapi-not-request-validation.md)
for why that specific reflection call does not contradict ADR 0001.

## Measured, not assumed

GoFast validates an isolated struct roughly 37.5x faster than Huma,
with zero heap allocations against three per call on Huma's side.
The full methodology, hardware, and exact commands to reproduce
this are documented in [`docs/BENCHMARKS.md`](docs/BENCHMARKS.md) —
including what this measurement does and does not claim.

## Project status

GoFast is pre-v1.0.0 and under active development. Its versioning
policy is documented in [`docs/adr/0005`](docs/adr/0005-semantic-versioning-policy-pre-v1.md):
minor versions may still break the core contract until the API has
proven stable across at least two consecutive releases.

What is built and verified today:
- Core HTTP contract: generics-based handlers, path/query
  binding, context propagation, configurable body limits
- Middleware: logging, panic recovery, CORS, health checks
- Structured, safe-by-default error handling (`BusinessError` vs
  internal errors, never leaked to clients)
- A complete code generator: validation, path binding, query
  binding with defaults, nested structs, slice/map element
  validation, all backed by a hash-based manifest and build-time
  staleness checks
- OpenAPI 3.0 spec generation and Swagger UI
- `golangci-lint` clean, zero `govulncheck` findings, `-race`
  clean across the full test suite, all wired into CI
- Measured performance evidence against Huma

See [`docs/ROADMAP.md`](docs/ROADMAP.md) for the complete, honest
state of every phase, including documented limitations and design
decisions still open.

## Architecture

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
parse.go go/ast struct parser
tag.go validate:/path:/query: tags
generate.go Validate() generation
generate_bind.go BindPath()/BindQuery()
manifest.go change detection
project.go go.mod root lookup

cmd/gofast/ the CLI (gofast generate [--check])

benchmarks/ isolated module measuring GoFast
against Huma

examples/minimal/ a working example, also used as the
generator's own integration test target

docs/
adr/ architecture decision records
VISION.md project philosophy and limits
ROADMAP.md live status of every phase
BENCHMARKS.md reproducible performance
evidence

## Why not just use Huma?

[Huma](https://github.com/danielgtaylor/huma) already provides
FastAPI-style automatic validation and OpenAPI generation for Go,
and is a mature, well-adopted project. GoFast is not trying to
replace it — it tests a specific, narrower architectural bet: that
the same automation can cost nothing at request time, because the
work is done once, at build time, in code that stays visible in
your own repository rather than inside the framework.

The evidence for that bet is in [`docs/BENCHMARKS.md`](docs/BENCHMARKS.md).
If it stops holding up under broader measurement, that will be
documented too — see [`docs/VISION.md`](docs/VISION.md) for the
full reasoning and the explicit limits of what GoFast claims.

## License

MIT — see [LICENSE](LICENSE) for details.

