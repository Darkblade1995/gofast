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

## Phase 7 — Production auth & operability - COMPLETE
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
- [x] A.2 — Rate limiting (ADR 0011): RateLimiter interface in
      gofast/ (zero new core dependencies), default IP-based
      keyFunc with override point for policy decisions (e.g.
      per-user). Two real implementations: gofast/ratelimit/memory
      (token bucket via golang.org/x/time/rate, single-process) and
      gofast/ratelimit/redis (fixed-window counter via INCR+EXPIRE,
      multi-instance). Fail-open on limiter errors (opposite of
      A.1c's fail-closed — availability vs identity have different
      failure priorities). 429 Too Many Requests via the existing
      BusinessError machinery. Verified end-to-end via
      examples/minimal (/login protected, burst of 5: 200×5, then
      429). Full test coverage: unit tests (memory) and integration
      tests against real Redis.
- [x] A.3 — Lifespan hooks (ADR 0012): Router.OnStartup/OnShutdown
      for registering hooks, and an optional Router.Run(ctx, addr,
      handler) that orchestrates the full lifecycle on top of
      net/http's own graceful shutdown — no custom server, no
      custom shutdown logic. Startup hooks run before traffic is
      accepted (fail-fast if any errors); shutdown hooks run in
      LIFO order after in-flight requests drain. handler is
      optional — pass nil to serve the Router directly, or a
      middleware chain (CORS, Recovery, Logger) to keep using those
      globally with Run's lifecycle management. Verified end-to-end
      via examples/minimal: startup hook checks Redis connectivity
      before serving, shutdown hook closes the Redis client
      cleanly on SIGINT. Unit tests cover startup ordering,
      fail-fast on startup error, and LIFO shutdown order.

**Fase A — COMPLETE.**


## Phase 8 — Real-time & operability extensions - COMPLETE
- [x] B.1 — WebSockets (ADR 0013): github.com/coder/websocket
      (gorilla/websocket is archived; coder/websocket is actively
      maintained, zero dependencies, native context.Context).
      New WSHandler type and Router.RegisterWS, deliberately
      separate from Handler[In, Out] — the request/response
      contract does not fit a long-lived, bidirectional
      connection. Existing per-route middleware (auth, rate
      limiting) still composes at the upgrade step. Verified
      end-to-end via examples/minimal (/ws/echo) through Postman's
      WebSocket client, with the full middleware chain (CORS,
      Recovery, Logger) active. Found and fixed a real bug in the
      process: Logger's statusRecorder wrapper did not implement
      http.Hijacker, silently breaking WebSocket upgrades for any
      route served through Logger — now fixed by having
      statusRecorder delegate Hijack() to the underlying
      ResponseWriter. Known gap: WebSocket routes do not appear in
      GenerateOpenAPI's output (OpenAPI 3.0 has no native
      WebSocket representation) — documented, not silently
      ignored. Unit test coverage via a real client/server round
      trip (dial, write, read, close).
- [x] B.2 — Background tasks (ADR 0014): SpawnTask(ctx, fn),
      accessed via context injection — mirrors ClaimsFromContext's
      pattern (ADR 0008) rather than changing Handler[In, Out]'s
      signature, which would have broken every existing handler.
      Router.EnableBackgroundTasks(timeout) opts a Router in,
      injecting a taskRunner into each request's context and
      registering an OnShutdown hook (reusing A.3's mechanism
      directly) that waits for in-flight tasks before shutdown
      proceeds. Spawned tasks run with an independent context, not
      the request's — the same fix already applied to WebSocket
      connections in ADR 0013, since a request's context is
      canceled once its response is written. No retry, no
      persistence — deliberately out of scope; durable background
      work belongs in a real job queue. Verified end-to-end via
      examples/minimal (/login spawns a simulated notification
      task; response returns in ~120µs, the task logs ~500ms
      later, confirmed via server timing). Unit tests cover
      spawn-and-complete, no-op without EnableBackgroundTasks, and
      shutdown waiting for in-flight tasks.
- [x] B.3 — Built-in TestClient (ADR 0015): TestClient wraps any
      http.Handler (Router, or Router plus middleware chain) and
      dispatches requests via httptest.NewRecorder — in-process,
      no real network socket, faster and free of port-allocation
      flakiness. GET/POST/PUT/PATCH/DELETE marshal bodies to JSON
      automatically. SetAuthToken carries Authorization: Bearer
      across subsequent requests, targeting the login-then-call-
      protected-route flow common to GoFast's own auth work
      (ADRs 0008–0010). Deliberately narrow scope: no new assertion
      DSL, just AssertStatus/DecodeJSON on top of the raw response
      — richer assertions are left to whatever library (or none)
      the application already uses. Verified via its own test
      suite exercising a real authenticated flow: unauthenticated
      request rejected, token issued and set, authenticated request
      succeeds with correct claims round-tripped.

**Fase B — COMPLETE.**



## Phase 9 — Streaming & content negotiation - COMPLETE
- [x] C.1 — Streaming responses via SSE (ADR 0016): new SSEHandler
      type and Router.RegisterSSE, deliberately separate from
      Handler[In, Out] (same reasoning as WebSockets, ADR 0013).
      SSEStream.Send marshals to JSON, writes SSE wire format, and
      flushes immediately — without the explicit flush, the
      response writer's default buffering means the client gets
      nothing until the handler returns. Server-Sent Events chosen
      over generic byte streaming as the first primitive: simpler
      format, native browser support via EventSource, covers the
      common progress/incremental-generation case directly.
      Handler receives req.Context() (unlike RegisterWS/SpawnTask,
      which deliberately use an independent context) — SSE lives
      within a single HTTP request's lifecycle, so request
      cancellation is exactly the right signal to stop streaming.
      Found and fixed a second real bug in the same class as B.1's
      Hijacker fix: Logger's statusRecorder also silently dropped
      http.Flusher, breaking SSE for any route served through
      Logger; statusRecorder now delegates Flush() to the
      underlying ResponseWriter too. Verified end-to-end via
      examples/minimal (/sse/progress) through Postman — 5 events,
      ~500ms apart, ~2.5s total, confirmed via response timestamps
      and server-side request duration logging. Unit tests cover
      wire format, event ordering, headers, and context
      propagation. Known gap: SSE routes, like WebSocket routes,
      don't appear in OpenAPI output (documented, consistent with
      B.1's same gap).
- [x] C.2 — Multipart/form-data (ADR 0017): new `form:"..."` tag,
      following the exact same pattern as `path:"..."` and
      `query:"..."` — extractSimpleTag/tagFields reused unchanged,
      new GenerateBindMultipart in internal/codegen mirroring
      GenerateBindPath/GenerateBindQuery. New MultipartBinder
      interface and UploadedFile type (thin wrapper over
      *multipart.FileHeader) in gofast/. Handler[In, Out] parses
      the form (bounded by the Router's existing maxBodySize) and
      calls BindMultipart when the input implements it — the 415
      rejection for multipart/form-data is removed;
      x-www-form-urlencoded remains rejected (no concrete need
      identified). Chose build-time codegen over runtime reflection
      specifically to preserve ADR 0001/0003's zero-reflection
      guarantee for this final roadmap item — the simpler
      alternative (reflection) was explicitly rejected as
      inconsistent with the project's own foundational bet.
      Verified end-to-end via examples/minimal (/upload): a real
      9752-byte file uploaded via Postman form-data, correctly
      read back with matching filename and size. Unit tests cover
      generated code shape (with/without form-tagged fields) and
      Handler's runtime behavior (valid multipart accepted,
      malformed multipart rejected with 400 — updated from the
      prior test verifying the now-obsolete 415 rejection).

**Fase C — COMPLETE.**

**ROADMAP COMPLETE — all of Fase A, B, and C are done. v1.0.0
is now unblocked per the versioning criteria in ADR 0005 (API
stability across two consecutive releases still applies before
tagging).**




## Future enhancements (revisit if conditions change)
- [ ] HTTP QUERY method support — IETF draft, not finalized


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

## Resolved risks
- ~~Manifest race condition if `gofast generate` runs concurrently
  in a monorepo with multiple packages~~ — RESOLVED (ADR 0018):
  `Manifest` gains a `Version` counter, incremented on every
  successful `Save`. New `UpdateAndSave(startDir, sourcePath,
  generatedFiles)` performs the full load-update-save cycle
  under a package-level mutex (serializing goroutines within one
  process) plus optimistic version-conflict detection (catching
  writes from other processes), retrying up to 5 times on
  conflict. `cmd/gofast/main.go` now uses `UpdateAndSave` instead
  of separate `Update`+`Save` calls. Initial mutex placement
  (guarding only `Save`, not the full cycle) was insufficient —
  discovered via `TestUpdateAndSave_ConcurrentWritesDoNotLoseEntries`
  (10 concurrent goroutines), which failed until the mutex was
  moved to wrap the entire `UpdateAndSave` cycle. Verified clean
  under `-race`.

- ~~`gofast generate` emitted generated files that did not
  compile (unused imports, empty loops)~~ — RESOLVED:
  `GenerateValidate` wrote `import ("fmt"; "strings")`
  unconditionally, so any file whose rules did not need both
  packages failed with `imported and not used`. The most common
  case: any struct validated without the `email` rule (only
  `required,min=3`) did not compile after `gofast generate`.
  Found while consuming v0.2.1 as an external dependency from
  TaskForge (friction #4, blocking). Investigating it exposed two
  more bugs of the same family: `"fmt"` was also imported
  unconditionally (a file with only unknown rules, such as
  `oneof=...`, produced `imported and not used`), and
  `dive`/`dive_key` with no emittable inner rule emitted an empty
  `for idx, v := range x { }` loop, which fails with `declared and
  not used`. Fix: emitters now record the packages they actually
  use in an `importSet`, and the import block is built after
  generation from that set (nothing is imported when nothing
  needs it); a `dive` loop is only opened when its body is
  non-empty. The pre-existing test only ran `format.Source`, which
  checks syntax, not types, so it could never have caught this.
  New test `TestGeneratedValidateCompiles` runs `go/types` over
  the generated code (what the compiler checks) and covers four
  cases: a control, no `email` rule, only unknown rules, and
  `dive` with an unknown inner rule. Verified: the three failing
  cases failed before the fix and pass after; `go test -race
  ./...` clean; end-to-end with a temporary module reproducing
  TaskForge's `CreateTeamInput`.



## Known risks (tracked, not yet resolved)
- Codegen complexity may exceed initial estimate — if so, ADR 0001
  gets revisited via a new ADR, not edited (this is a governance
  note, not a bug — nothing to fix unless it happens)
- Route registration is not concurrency-safe — documented via
  ADR 0002 as an intentional startup-only design constraint
