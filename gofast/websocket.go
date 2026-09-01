package gofast

import (
	"context"
	"log"
	"net/http"

	"github.com/coder/websocket"
)

// WSHandler handles a single WebSocket connection for its entire
// lifetime. Unlike Handler[In, Out], which decodes one request and
// encodes one response, WSHandler receives an open connection and
// is expected to read and write messages for as long as the
// connection stays open, returning when it is done (the connection
// closed by the client, an application-level decision to stop, or
// an error).
//
// conn is coder/websocket's own connection type, used directly —
// GoFast does not wrap or reimplement the WebSocket protocol. See
// ADR 0013.
type WSHandler func(ctx context.Context, conn *websocket.Conn) error

// RegisterWS registers a WebSocket handler at path, performing the
// HTTP-to-WebSocket upgrade and then running handler in its own
// goroutine for the lifetime of the connection. Like Register,
// RegisterWS routes through the same underlying http.ServeMux, so
// existing per-route middleware (AuthMiddleware, RateLimitMiddleware)
// can still run before the upgrade happens — see ADR 0013.
//
// If handler returns an error, it is logged; RegisterWS always
// closes the connection when handler returns, whether it returned
// an error or not.
func (r *Router) RegisterWS(path string, handler WSHandler) {
	wrapped := func(w http.ResponseWriter, req *http.Request) {
		conn, err := websocket.Accept(w, req, nil)
		if err != nil {
			// websocket.Accept already wrote an appropriate HTTP
			// error response on failure; nothing further to do.
			return
		}

		go func() {
			defer conn.CloseNow()

			if err := handler(context.Background(), conn); err != nil {
				log.Printf("[gofast] websocket handler error on %s: %v", path, err)
			}
		}()
	}

	r.mux.HandleFunc("GET "+path, wrapped)
}