# GoFast — Roadmap

Living document. Updated as new gaps are discovered through design
review. Items move between sections as they get resolved.

## Status legend
- [ ] Not started
- [~] In progress
- [x] Done

---

## Phase 0 — Foundation -  COMPLETE
- [x] Module structure (cmd/, internal/, public package)
- [x] Handler[In, Out] core contract
- [x] Router wrapping http.ServeMux
- [x] Minimal working example (examples/minimal)

## Phase 1 — Critical contract fixes - COMPLETE
- [x] Add context.Context to Handler signature
- [x] Path parameters (/accounts/{id})
- [x] Query parameters (?page=2&limit=20)
- [x] Request body size limit
- [x] DisallowUnknownFields on JSON decoding

## Phase 2 — Middleware layer - COMPLETE
- [x] Logger middleware
- [x] Recovery middleware
- [x] CORS middleware — fully configurable via WithAllowedOrigins
- [x] Standardized error response format — ErrorBody, BusinessError
- [x] Configurable max body size
- [x] Content-Type validation — 415 UNSUPPORTED_MEDIA_TYPE
- [x] Health check helper — router.HealthCheck(path, checkFunc)

## Phase 3 — Code generation - COMPLETE
- [x] internal/codegen — AST-based struct parser
- [x] Generate Validate() from `validate:"..."` tags
- [x] Generate BindPath() from `path:"..."` tags
- [x] Generate BindQuery() from `query:"..."` tags, with defaults
- [x] Nested struct validation — 4-level recursion verified
- [x] Primitive slice validation via `dive` tag
- [x] Map key validation via `dive_key` tag
- [x] cmd/gofast CLI — `gofast generate [--check] <file>`
- [x] Hash-based manifest (.gofast/manifest.json)
- [x] Manifest location via FindProjectRoot()
- [x] Build-time verification — TestGeneratedCodeIsUpToDate
- [x] CI check logic: `gofast generate --check`

## Phase 4 — OpenAPI / Swagger - COMPLETE
- [x] ADR 0004 — reflection at route registration
- [x] HandlerInfo carrying reflect.Type for In/Out
- [x] Route extended with InType/OutType
- [x] Generate OpenAPI 3.0 spec from Router.Routes()
- [x] ServeOpenAPI() + ServeSwaggerUI() (CDN-loaded)
- [x] Path/query params in spec — fixed via splitParamsAndBody()
- [x] Verified: GET/POST/PATCH/PUT/DELETE, including combined
      path-param+body case

## Phase 5 — Engineering standards - COMPLETE
- [x] golangci-lint (v2.11.4, pinned), configured via .golangci.yml
- [x] All lint issues resolved with evidence — 0 issues
- [x] Wired into .github/workflows/ci.yml
- [x] govulncheck — 0 vulnerabilities, wired into CI
- [x] -race enabled and verified clean across all tests
- [x] Benchmarks: GoFast vs Huma — GoFast 2.123 ns/op (0 allocs)
      vs Huma 79.53 ns/op (3 allocs), ~37x faster on isolated
      struct validation
- [x] Example tests — ExampleHandler, ExampleNewBusinessError
- [x] Public godoc comments on all core exported types/functions
- [x] English-only across code, errors, logs, docs
- [x] Fuzz testing — FuzzParseFile on internal/codegen's AST
      parser, VERIFIED clean across 2,072,795 executions in a
      60-second run (0 panics), seeded with valid structs, empty
      structs, nested/tagged structs, and edge cases (empty file,
      package-only file)

## Phase 6 — Design decisions to formalize - COMPLETE
- [x] ADR 0005 — Semantic versioning policy pre-v1.0.0
- [x] ADR 0006 — Dependency injection via explicit closures
- [x] ADR 0007 — Migration path from Gin/Echo/Huma

## Phase 7 — Production auth & operability - IN PROGRESS
- [x] ADR 0008 — JWT stateless validation, context propagation
- [x] AuthMiddleware — HMAC JWT validation (missing/malformed/
      invalid-signature/expired all return 401 UNAUTHORIZED)
- [x] ClaimsFromContext — typed, collision-safe context key
- [x] ErrCodeUnauthorized added to error taxonomy
- [x] HandlerInfo.Wrap — per-route middleware composition, used
      to apply AuthMiddleware to individual routes without
      affecting others registered on the same Router
- [x] Verified end-to-end in examples/minimal (no token → 401,
      /login issues real signed JWT, valid token → 200 with
      correct path-bound data)
- [x] A.1b — Refresh tokens (ADR 0009): stateless, HMAC-signed,
      distinguished by a "type" claim (access vs refresh).
      IssueAccessToken, IssueRefreshToken, RefreshHandler.
      AuthMiddleware now rejects refresh tokens; RefreshHandler
      rejects access tokens — verified in both directions, unit
      tests and end-to-end via examples/minimal (/refresh).
      No rotation, no revocation — explicitly deferred to A.1c.
- [x] A.1c — Token revocation / blacklist (ADR 0010): TokenRevoker
      interface in gofast/ (jti-based, per-token), zero new core
      dependencies. Production Redis implementation in
      gofast/revocation/redis (separate importable package —
      SETEX-equivalent TTL, no manual cleanup needed).
      AuthMiddleware and RefreshHandler both accept an optional
      revoker; nil preserves A.1a/A.1b's exact original stateless
      behavior. Fail-closed on revocation-check errors, verified
      live (Redis stopped → 401, not 200). Verified end-to-end via
      examples/minimal (/logout revokes the active access token; a
      second request with the same token is rejected). Full test
      coverage: unit tests with an in-memory fake revoker
      (auth_test.go), integration tests against real Redis
      (redis_revoker_test.go), and manual end-to-end verification.
- [ ] A.2 — Rate limiting (golang.org/x/time/rate)
- [ ] A.3 — Lifespan hooks (startup/shutdown)



## Future enhancements (revisit if conditions change)
- [ ] HTTP QUERY method support — IETF draft, not finalized

## Backlog — explicitly out of scope for V1
## Backlog — explicitly out of scope for V1
- [ ] Streaming responses
- [ ] Content negotiation (multipart, form-data)
- [ ] Built-in TestClient
- [ ] `any`/`interface{}` field support in codegen
- [ ] WebSockets
- [ ] Background tasks

## Open decisions (deferred, not avoided)
- [ ] Vendor Swagger UI assets (go:embed) vs CDN-loaded (current)
- [ ] API-diff enforcement in CI (golang.org/x/exp/cmd/apidiff or
      gorelease) — blocked on having at least two tagged releases
      to compare against; cannot be meaningfully built before that
      exists

## Known risks (tracked, not yet resolved)
- Manifest race condition if `gofast generate` runs concurrently
  in a monorepo with multiple packages
- Codegen complexity may exceed initial estimate — if so, ADR 0001
  gets revisited via a new ADR, not edited
- Route registration is not concurrency-safe — documented via
  ADR 0002 as an intentional startup-only design constraint
- Manifest does not verify that its recorded "generated" files
  still exist on disk before skipping regeneration — if a
  .gen.go file is deleted externally (manually, by another tool,
  or by test cleanup) but the source .go file is unchanged, `gofast
  generate` reports "unchanged, skipping" instead of noticing the
  output is missing and regenerating it. Discovered while fixing
  TestGenerateAndWrite writing into examples/minimal instead of a
  temp directory (see commit history).
