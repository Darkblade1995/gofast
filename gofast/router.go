// gofast/router.go
package gofast

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"reflect"
	"syscall"
	"time"
)

type contextKey string

const configKey contextKey = "gofast-router-config"

type routerConfig struct {
	maxBodySize    int64
	allowedOrigins []string
}

// Route describes a single registered HTTP route, including the
// reflected input and output types captured from the handler at
// registration time. GenerateOpenAPI uses this information to
// build the OpenAPI spec.
type Route struct {
	Method  string
	Path    string
	InType  reflect.Type
	OutType reflect.Type
}

// Option configures a Router at construction time. Use the
// WithXxx functions to build Options; the Option type itself is
// not meant to be implemented directly.
type Option func(*Router)

// WithMaxBodySize sets the maximum allowed size, in bytes, of an
// incoming request body. Requests exceeding this limit are
// rejected before JSON decoding begins. The default is 1 MiB.
func WithMaxBodySize(n int64) Option {
	return func(r *Router) {
		r.config.maxBodySize = n
	}
}

// WithAllowedOrigins sets the list of origins permitted by the
// CORS middleware. Passing no origins leaves the current
// configuration unchanged. The default is []string{"*"}.
func WithAllowedOrigins(origins ...string) Option {
	return func(r *Router) {
		if len(origins) == 0 {
			return
		}
		r.config.allowedOrigins = origins
	}
}

// Router wraps http.ServeMux, adding route tracking for OpenAPI
// generation and per-request configuration (body size limits,
// CORS origins) injected via context.
//
// Router is not safe for concurrent Register calls, or for
// registering routes after the server has started serving
// traffic. See ADR 0002 for the reasoning behind this constraint.
type Router struct {
	mux           *http.ServeMux
	routes        []Route
	config        routerConfig
	startupHooks  []func(context.Context) error
	shutdownHooks []func(context.Context) error
}

// NewRouter creates a Router with the given options applied. Call
// Register to add routes before passing the Router (or a
// middleware chain wrapping it) to http.ListenAndServe, or to
// Router.Run.
func NewRouter(opts ...Option) *Router {
	r := &Router{
		mux:    http.NewServeMux(),
		routes: []Route{},
		config: routerConfig{
			maxBodySize:    1 << 20,
			allowedOrigins: []string{"*"},
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Register adds a route for the given HTTP method and path,
// serving the given HandlerInfo. Routes must be registered before
// the Router begins serving traffic; see the Router type doc for
// details.
func (r *Router) Register(method, path string, info HandlerInfo) {
	r.routes = append(r.routes, Route{
		Method:  method,
		Path:    path,
		InType:  info.InType,
		OutType: info.OutType,
	})

	wrapped := func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), configKey, r.config)
		info.Func(w, req.WithContext(ctx))
	}

	r.mux.HandleFunc(method+" "+path, wrapped)
}

// Routes returns all routes registered so far, in registration
// order.
func (r *Router) Routes() []Route {
	return r.routes
}

func (r *Router) config_() routerConfig {
	return r.config
}

// ServeHTTP implements http.Handler, dispatching each request to
// its registered route.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// OnStartup registers a hook to run before Run begins accepting
// traffic. Hooks run in registration order. If any hook returns an
// error, Run returns that error immediately without starting the
// server. See ADR 0012.
func (r *Router) OnStartup(hook func(ctx context.Context) error) {
	r.startupHooks = append(r.startupHooks, hook)
}

// OnShutdown registers a hook to run after Run has finished
// draining in-flight requests. Hooks run in reverse registration
// order (LIFO), mirroring defer semantics. See ADR 0012.
func (r *Router) OnShutdown(hook func(ctx context.Context) error) {
	r.shutdownHooks = append(r.shutdownHooks, hook)
}

// Run is an optional convenience that orchestrates a Router's full
// lifecycle: it runs OnStartup hooks, serves HTTP on addr, blocks
// until ctx is canceled or the process receives SIGINT/SIGTERM,
// then gracefully drains in-flight requests via http.Server.Shutdown
// and runs OnShutdown hooks. handler is what actually serves each
// request — pass nil to serve the Router directly, or pass a
// middleware chain wrapping the Router (e.g. CORS, Recovery,
// Logger) to apply those globally while still using Run's
// lifecycle management. Using Run at all remains optional —
// constructing an *http.Server manually is fully supported too.
// See ADR 0012.
func (r *Router) Run(ctx context.Context, addr string, handler http.Handler) error {
	for _, hook := range r.startupHooks {
		if err := hook(ctx); err != nil {
			return err
		}
	}

	if handler == nil {
		handler = r
	}
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	signalCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return err
		}
	case <-signalCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	for i := len(r.shutdownHooks) - 1; i >= 0; i-- {
		if err := r.shutdownHooks[i](shutdownCtx); err != nil {
			return err
		}
	}

	return nil
}

func configFromContext(ctx context.Context) routerConfig {
	cfg, ok := ctx.Value(configKey).(routerConfig)
	if !ok {
		return routerConfig{
			maxBodySize:    1 << 20,
			allowedOrigins: []string{"*"},
		}
	}
	return cfg
}