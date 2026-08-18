package gofast

import (
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Logger wraps an http.Handler, logging method, path, status
// code, and duration for every request.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		method := sanitizeForLog(r.Method)
		path := sanitizeForLog(r.URL.Path)

		// #nosec G706 -- method and path are sanitized by
		// sanitizeForLog (strips \n, \r, and quotes the result),
		// which gosec's taint analysis does not recognize as a
		// known sanitizer.
		log.Printf("[gofast] %s %s -> %d (%s)",
			method,
			path,
			rec.status,
			time.Since(start),
		)
	})
}

func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return strconv.Quote(s)
}

// Recovery wraps an http.Handler, catching panics from the
// wrapped handler and responding with a 500 error instead of
// crashing the process. Panics are logged with a full stack
// trace.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[gofast] panic recovered: %v\n%s", err, debug.Stack())
				writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS returns a middleware that applies CORS headers based on
// the Router's configured allowed origins. Call it with the
// Router used to register routes, then wrap the resulting
// middleware around the rest of the handler chain.
func CORS(router *Router) func(http.Handler) http.Handler {
	origins := router.config_().allowedOrigins

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if originAllowed(origin, origins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(origins) == 1 && origins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func originAllowed(origin string, allowed []string) bool {
	for _, o := range allowed {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}
