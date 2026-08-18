package gofast

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

// HealthCheckFunc reports whether the service is healthy. Return
// nil for healthy, or a descriptive error to indicate a failure
// (e.g. a database ping timeout). GoFast does not inspect the
// error's content, it only uses its presence to decide the
// response status.
type HealthCheckFunc func(ctx context.Context) error

type healthStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HealthCheck registers a GET route at path that reports service
// health. check is called on every request; if it returns nil the
// route responds 200 with {"status":"ok"}, otherwise it responds
// 503 with {"status":"unhealthy","error":"<check error>"}.
//
// GoFast does not decide what "healthy" means — check is entirely
// the caller's responsibility (e.g. pinging a database or cache).
func (r *Router) HealthCheck(path string, check HealthCheckFunc) {
	r.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {
		if err := check(req.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if encErr := json.NewEncoder(w).Encode(healthStatus{
				Status: "unhealthy",
				Error:  err.Error(),
			}); encErr != nil {
				log.Printf("[gofast] failed to encode health check response: %v", encErr)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(healthStatus{Status: "ok"}); err != nil {
			log.Printf("[gofast] failed to encode health check response: %v", err)
		}
	})
}
