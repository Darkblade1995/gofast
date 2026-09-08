# ADR 0015 — Built-in TestClient: httptest.Recorder, not a real server

## Status
Accepted

## Context
Every test written so far in this project — in gofast/, and every
manual verification pattern used against examples/minimal — repeats
the same boilerplate: construct an httptest server or recorder,
build a request by hand, marshal/unmarshal JSON, check the status
code. FastAPI's TestClient (built on httpx) exists to eliminate
exactly this repetition. This is Fase B.3, closing out Fase B.

## Decision

**TestClient wraps any http.Handler** (a bare Router, or a Router
wrapped in middleware like CORS/Recovery/Logger — the same handler
an application would actually pass to Router.Run), and dispatches
requests using httptest.NewRecorder() internally rather than
httptest.NewServer(). This means no real network socket, no real
port — requests are served in-process via Handler.ServeHTTP,
exactly as Go's own httptest package is designed to be used for
this purpose. This is strictly faster and avoids port-allocation
flakiness in CI, with no loss of fidelity: the same ServeHTTP
codepath runs either way.

**API shape**, mirroring the manual pattern already used throughout
this project's own tests:

```go
client := gofast.NewTestClient(handler)
resp := client.POST(t, "/login", loginBody)
resp.AssertStatus(t, http.StatusOK)
var out LoginOutput
resp.DecodeJSON(t, &out)
```

`GET`, `POST`, `PUT`, `PATCH`, `DELETE` are provided; body is
accepted as any value and marshaled to JSON automatically (matching
how every handler in this project already expects JSON request
bodies).

**Authenticated flows are a first-class case**, not an
afterthought — this project's own auth work (ADRs 0008–0010) makes
"log in, then call a protected route" one of the most common test
scenarios GoFast handlers will have:

```go
client.SetAuthToken(accessToken)
```

Once set, every subsequent request through that client automatically
carries `Authorization: Bearer <token>`, until changed or cleared.

**TestClient does not become a new assertion framework.** It
returns raw status codes and bodies plus a small number of
convenience helpers (AssertStatus, DecodeJSON) — it does not compete
with testify or attempt rich matcher semantics. This keeps
TestClient's own scope narrow and avoids GoFast quietly growing a
second, competing assertion library.

## Alternatives considered

- **httptest.NewServer (real network server) as the basis**
  Rejected — slower (real socket setup/teardown per test), and
  introduces port-allocation as a source of test flakiness under
  parallel test execution, for no fidelity benefit: ServeHTTP is
  called either way, whether through a real listener or a recorder.

- **A full assertion/matcher DSL** (e.g. fluent chains for JSON path
  assertions, header matchers, etc.)
  Rejected — scope creep well beyond "eliminate HTTP boilerplate."
  An application wanting richer assertions can already decode the
  response body with DecodeJSON and use whatever assertion library
  it prefers (testify, or none at all) on the resulting Go value.

- **Requiring a real *http.Server, mirroring production exactly
  (TLS, timeouts, etc.)**
  Rejected for the same reason as the first alternative: test speed
  and simplicity matter more here than exercising *http.Server's own
  configuration, which is not what a handler-level test is
  verifying in the first place.

## Consequences

**Gained:**
- New GoFast-based projects get a fast, in-process test client with
  zero additional dependencies (built entirely on net/http/httptest,
  already in the standard library).
- Testing an authenticated flow — login, then call a protected
  route — no longer requires re-deriving the Authorization header
  wiring in every test file.

**Sacrificed:**
- TestClient does not exercise the real network stack (TLS
  handshakes, connection timeouts, actual socket behavior) — tests
  using it verify handler and middleware logic, not
  transport-layer behavior. Applications needing to test the latter
  should use httptest.NewServer directly, which remains fully
  available and is not replaced by this addition.

**Risk accepted consciously:**
- None significant — this is a pure test-time addition with no
  production code path affected, and no new dependency introduced
  (all built on the Go standard library's own httptest package).