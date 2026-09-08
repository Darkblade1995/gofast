package gofast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSpawnTask_RunsAndCompletes(t *testing.T) {
	router := NewRouter()
	router.EnableBackgroundTasks(2 * time.Second)

	done := make(chan struct{})
	router.Register("GET", "/spawn", HandlerInfo{
		Func: func(w http.ResponseWriter, req *http.Request) {
			SpawnTask(req.Context(), func(ctx context.Context) error {
				close(done)
				return nil
			})
			w.WriteHeader(http.StatusOK)
		},
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/spawn")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("spawned task did not run within timeout")
	}
}

func TestSpawnTask_WithoutEnableBackgroundTasks_DoesNotRun(t *testing.T) {
	router := NewRouter()
	// Note: EnableBackgroundTasks is NOT called.

	ran := false
	router.Register("GET", "/spawn", HandlerInfo{
		Func: func(w http.ResponseWriter, req *http.Request) {
			SpawnTask(req.Context(), func(ctx context.Context) error {
				ran = true
				return nil
			})
			w.WriteHeader(http.StatusOK)
		},
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/spawn")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Give a hypothetical (incorrectly) spawned goroutine a moment
	// to run, so this test would actually catch the bug if SpawnTask
	// ran despite the missing EnableBackgroundTasks call.
	time.Sleep(50 * time.Millisecond)

	if ran {
		t.Error("expected SpawnTask to be a no-op without EnableBackgroundTasks")
	}
}

func TestRouter_WaitsForBackgroundTasksOnShutdown(t *testing.T) {
	router := NewRouter()
	router.EnableBackgroundTasks(2 * time.Second)

	taskFinished := false
	router.Register("GET", "/spawn", HandlerInfo{
		Func: func(w http.ResponseWriter, req *http.Request) {
			SpawnTask(req.Context(), func(ctx context.Context) error {
				time.Sleep(100 * time.Millisecond)
				taskFinished = true
				return nil
			})
			w.WriteHeader(http.StatusOK)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() {
		runDone <- router.Run(ctx, ":0", nil)
	}()

	// Give Run a moment to start serving, then trigger the task and
	// immediately cancel — the shutdown must still wait for it.
	time.Sleep(50 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		router.ServeHTTP(w, req)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/spawn", nil)
	handler.ServeHTTP(rec, req)

	cancel()

	if err := <-runDone; err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	if !taskFinished {
		t.Error("expected the background task to finish before shutdown completed")
	}
}