// gofast/router.go
package gofast

import (
	"context"
	"net/http"
	"reflect"
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
	mux    *http.ServeMux
	routes []Route
	config routerConfig
}

// NewRouter creates a Router with the given options applied. Call
// Register to add routes before passing the Router (or a
// middleware chain wrapping it) to http.ListenAndServe.
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
