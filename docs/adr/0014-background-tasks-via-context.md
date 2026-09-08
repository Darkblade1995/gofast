# ADR 0014 — Background tasks: TaskRunner injected via context, coordinated with lifespan shutdown

## Status
Accepted

## Context
GoFast handlers today have no supported way to do work that
shouldn't block the response to the client — sending a
confirmation email, generating a report, processing an uploaded
file. The only option is spawning a bare goroutine by hand inside
a handler, with no way to know if it failed, no bound on how many
run concurrently, and — critically, now that A.3 (ADR 0012) added
Router.Run with graceful shutdown — no coordination with shutdown
at all. A background goroutine started by a handler could be
mid-flight when the process exits, cut off silently.

This is Fase B.2, following B.1 (WebSockets). This mirrors
FastAPI's BackgroundTasks, adapted to fit GoFast's existing
patterns rather than copying its API shape.

## Decision

**Access via context, not a Handler[In, Out] signature change.**
Adding a parameter to Handler[In, Out] would break every existing
handler written against GoFast so far (Login, GetAccount, all of
examples/minimal, and any adopter's own code) — an unforced
breaking change ADR 0005's semver policy does not require here,
since there is a non-breaking alternative available.

```go
func SpawnTask(ctx context.Context, fn func(ctx context.Context) error)
```

This mirrors ClaimsFromContext (ADR 0008): a handler that doesn't
need background tasks never calls SpawnTask and pays no cost; one
that does calls it exactly where it's needed, using the same
context-based access pattern already established for auth claims.

**The task's own context is independent of the request's**, for
the same reason RegisterWS's handler context is independent of the
HTTP request context (see ADR 0013's fix): a request's context is
canceled once the response is written, which would kill a
just-started background task immediately. SpawnTask derives the
task's context from the TaskRunner's own lifetime, not from the
request that spawned it.

**A TaskRunner is injected into the request context** (same
mechanism as routerConfig's context injection in Router.Register),
tracking in-flight tasks via a WaitGroup internally.

**Coordination with shutdown reuses A.3's mechanism directly.**
`Router` gains a method:

```go
func (r *Router) EnableBackgroundTasks(shutdownTimeout time.Duration)
```

which registers the TaskRunner's own OnShutdown hook — the exact
same mechanism ADR 0012 already built and proved end-to-end via
Redis connection cleanup. On shutdown, the hook waits (up to
shutdownTimeout) for in-flight tasks to finish before returning,
so Router.Run's existing shutdown sequence naturally waits for
background work too, without inventing a second shutdown pathway.

## Alternatives considered

- **Handler[In, Out] gains a `tasks TaskRunner` parameter**
  Rejected — breaks every existing handler for no necessary reason;
  see Decision above.

- **A fully separate goroutine pool with its own lifecycle, outside
  Router entirely**
  Rejected — this would duplicate the graceful-shutdown
  coordination A.3 already solved, creating two shutdown
  mechanisms instead of one. Reusing OnShutdown is not the
  "easier" choice for its own sake; it's the choice that avoids a
  second, parallel piece of infrastructure doing almost the same
  job as the first.

- **Bare goroutines remain the documented recommendation, GoFast
  adds nothing**
  Rejected — this is the status quo the ADR exists to fix. It
  offers no failure visibility, no concurrency bound, and no
  shutdown coordination, which is precisely the gap A.3's Run
  makes newly relevant (before Run existed, there was no graceful
  shutdown to coordinate with in the first place).

## Consequences

**Gained:**
- Handlers can offload non-blocking work with an API mirroring the
  pattern already used for auth (ClaimsFromContext).
- Background tasks are no longer silently at risk of being cut off
  mid-flight during graceful shutdown — Router.Run's existing
  shutdown sequence now waits for them too.

**Sacrificed:**
- No built-in retry, persistence, or distributed execution — a
  spawned task that fails simply returns an error, logged; there
  is no queue, no re-delivery. Applications needing durable,
  retryable background work should reach for a real job queue
  (e.g. the author's own SmartQueue project, or similar), not
  treat SpawnTask as a substitute for one. This is a deliberate
  scope boundary, not an oversight.
- EnableBackgroundTasks must be called explicitly (opt-in) — a
  Router that never calls it has no TaskRunner in context, and
  SpawnTask on such a Router is a caller error (documented, not
  silently swallowed — see implementation).

**Risk accepted consciously:**
- If shutdownTimeout elapses before all tasks finish, Router.Run
  proceeds with shutdown anyway rather than hanging indefinitely —
  an application whose background tasks routinely need longer than
  its configured timeout should either raise the timeout or
  reconsider whether that work belongs in a durable queue instead
  of SpawnTask, per the Sacrificed note above.