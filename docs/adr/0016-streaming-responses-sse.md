# ADR 0016 — Streaming responses: Server-Sent Events, a separate handler contract

## Status
Accepted

## Context
Handler[In, Out] encodes its entire response to JSON in one shot
(respondJSON in handler.go) — the client waits until the full
output exists. This does not fit cases where output should reach
the client progressively as it's generated: a long-running task's
progress, tokens streaming from an LLM, or any response too large
or too slow to assemble in full before sending. This is Fase C.1,
the first piece of the final phase before v1.0.0.

Like WebSockets (ADR 0013), streaming does not fit Handler[In,
Out]'s contract — a function that decodes one input and returns one
output. Forcing it there would misrepresent what a streaming
response actually is, the same reasoning already applied in ADR
0013.

## Decision

**Server-Sent Events (SSE) first**, not generic byte streaming.
SSE has a simple, well-defined format
(`event: <name>\ndata: <payload>\n\n`over a normal HTTP response
with `Content-Type: text/event-stream`), is natively supported by
browsers via `EventSource`, and covers the most common real-world
streaming need (progress updates, incremental generation) without
requiring a new protocol the way WebSockets did. Generic
chunked-byte streaming (e.g. for serving large files) is a natural
extension of the same underlying mechanism and can be added later
without revisiting this decision — deferring it now avoids
designing for a use case not yet concretely needed.

**A new, separate handler type**, mirroring ADR 0013's WSHandler:

```go
type SSEHandler func(ctx context.Context, stream *SSEStream) error
```

`*SSEStream` wraps the underlying `http.ResponseWriter` and
`http.Flusher`, exposing:

```go
func (s *SSEStream) Send(event string, data any) error
```

`Send` marshals `data` to JSON, writes it in SSE wire format, and
calls `Flush()` immediately after every write. The explicit flush
is not optional or an optimization — without it, the underlying
`http.ResponseWriter`'s default buffering means the client receives
nothing until the handler returns, which defeats the entire purpose
of streaming. This mirrors why WSHandler in ADR 0013 required its
own context handling to actually behave as a long-lived connection
rather than accidentally reverting to request/response semantics.

**A new registration method, parallel to Register and RegisterWS:**

```go
func (r *Router) RegisterSSE(path string, handler SSEHandler)
```

Existing per-route middleware composability (auth, rate limiting)
is preserved the same way ADR 0013 preserved it for WebSockets —
RegisterSSE still routes through the same underlying
`*http.ServeMux`.

## Alternatives considered

- **Generic chunked-byte streaming as the first (or only) primitive**
  Rejected for now — SSE's structured, well-understood format
  (named events, JSON payloads) fits the concrete use cases already
  identified (progress, incremental generation) more directly than
  raw byte chunks would. Not rejected permanently: see Decision
  above on adding it later as an extension.

- **Extending Handler[In, Out] with a "streaming mode" flag**
  Rejected — same reasoning as ADR 0013's rejection of extending
  Handler[In, Out] for WebSockets: the request/response shape is
  fundamentally different from a connection that writes
  progressively over time, and forcing one contract to silently
  behave as the other produces a misleading API.

- **WebSockets (already built, B.1) as the streaming mechanism
  instead of SSE**
  Rejected — WebSockets are bidirectional and require a protocol
  upgrade; SSE is simpler for the common case of server-to-client-only
  streaming over plain HTTP, requires no special client-side
  library (EventSource is built into browsers), and is the more
  proportionate tool when the client never needs to talk back.

## Consequences

**Gained:**
- Long-running or incrementally-generated responses (progress
  updates, streaming generation) can be delivered to the client as
  they happen, not just after the fact.
- Browsers can consume SSE endpoints natively via EventSource, with
  no additional client-side dependency.

**Sacrificed:**
- SSE routes, like WebSocket routes (ADR 0013), do not appear in
  GenerateOpenAPI's output — OpenAPI 3.0 has no native
  representation for either. This is a known, accepted, and now
  recurring gap across both real-time mechanisms, documented rather
  than silently ignored.
- SSE is one-directional (server to client only) — a use case
  needing the client to send data mid-stream needs WebSockets (B.1)
  instead, not this.

**Risk accepted consciously:**
- An SSEHandler that never returns (e.g. an infinite event loop
  without checking ctx.Done()) will hold its connection and
  goroutine open indefinitely — the same inherent risk already
  accepted for WebSocket connections in ADR 0013. SSEHandler
  implementations are expected to respect context cancellation to
  terminate cleanly, the same as any well-behaved Go function
  accepting a context.