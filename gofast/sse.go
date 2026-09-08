package gofast

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// SSEStream wraps an http.ResponseWriter for sending
// Server-Sent Events. Every Send flushes immediately — without
// that, the underlying ResponseWriter's default buffering means
// the client receives nothing until the handler returns, which
// would defeat the purpose of streaming. See ADR 0016.
type SSEStream struct {
	w http.ResponseWriter
	f http.Flusher
}

// Send writes one SSE event named event, with data marshaled to
// JSON as the event's payload, and flushes it to the client
// immediately.
func (s *SSEStream) Send(event string, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("gofast: failed to marshal SSE event data: %w", err)
	}

	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
		return fmt.Errorf("gofast: failed to write SSE event: %w", err)
	}

	s.f.Flush()
	return nil
}

// SSEHandler streams events to a client over a single HTTP
// response for as long as the connection stays open. Unlike
// Handler[In, Out], which encodes one response in full before
// sending it, SSEHandler is expected to call stream.Send
// repeatedly as data becomes available, and to return when done
// (naturally, or in response to ctx being canceled). See ADR 0016.
type SSEHandler func(ctx context.Context, stream *SSEStream) error

// RegisterSSE registers an SSE handler at path. It sets the
// headers required for Server-Sent Events (Content-Type:
// text/event-stream, Cache-Control: no-cache, Connection:
// keep-alive) before invoking handler. Like RegisterWS, RegisterSSE
// routes through the same underlying http.ServeMux, so existing
// per-route middleware (AuthMiddleware, RateLimitMiddleware) still
// applies. See ADR 0016.
//
// If the ResponseWriter passed to this route does not support
// http.Flusher (required for streaming), RegisterSSE responds with
// 500 and does not call handler.
func (r *Router) RegisterSSE(path string, handler SSEHandler) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "streaming not supported by this response writer", nil)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		stream := &SSEStream{w: w, f: flusher}
		if err := handler(req.Context(), stream); err != nil {
			log.Printf("[gofast] sse handler error on %s: %v", path, err)
		}
	}

	r.mux.HandleFunc("GET "+path, wrapped)
}