package gofast

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type taskRunnerContextKey struct{}

// taskRunner tracks in-flight background tasks so Router can wait
// for them to finish during graceful shutdown. See ADR 0014.
type taskRunner struct {
	ctx context.Context
	wg  sync.WaitGroup
}

// SpawnTask starts fn in its own goroutine, tracked so that
// Router.Run's graceful shutdown (see ADR 0012) waits for it to
// finish before completing, up to the timeout configured via
// EnableBackgroundTasks. fn receives a context independent of the
// request that called SpawnTask — the request's own context is
// canceled once its response is written, which would otherwise
// kill a just-started task immediately (the same class of bug
// fixed for WebSocket connections in ADR 0013).
//
// SpawnTask requires that ctx was produced by a request served
// through a Router with EnableBackgroundTasks called; otherwise it
// logs an error and does not run fn. An error returned by fn is
// logged; there is no retry — see ADR 0014's Sacrificed section
// for why durable, retryable work belongs in a real job queue
// instead.
func SpawnTask(ctx context.Context, fn func(ctx context.Context) error) {
	runner, ok := ctx.Value(taskRunnerContextKey{}).(*taskRunner)
	if !ok {
		log.Printf("[gofast] SpawnTask called without EnableBackgroundTasks configured on the Router; task not started")
		return
	}

	runner.wg.Add(1)
	go func() {
		defer runner.wg.Done()
		if err := fn(runner.ctx); err != nil {
			log.Printf("[gofast] background task error: %v", err)
		}
	}()
}

// EnableBackgroundTasks turns on SpawnTask for handlers registered
// on this Router: it injects a task tracker into every request's
// context, and registers an OnShutdown hook (see ADR 0012) that
// waits for in-flight tasks to finish, up to shutdownTimeout,
// before Router.Run proceeds with the rest of its shutdown
// sequence. Call this before Register-ing routes that use
// SpawnTask. See ADR 0014.
func (r *Router) EnableBackgroundTasks(shutdownTimeout time.Duration) {
	runner := &taskRunner{ctx: context.Background()}

	r.taskRunner = runner

	r.OnShutdown(func(ctx context.Context) error {
		done := make(chan struct{})
		go func() {
			runner.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-time.After(shutdownTimeout):
			return fmt.Errorf("background tasks did not finish within %s", shutdownTimeout)
		}
	})
}

func withTaskRunner(runner *taskRunner, next http.Handler) http.Handler {
	if runner == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), taskRunnerContextKey{}, runner)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}