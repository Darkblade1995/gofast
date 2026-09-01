package gofast

import (
	"context"
	"testing"
	"time"
)

func TestRouter_OnStartup_RunsBeforeServing(t *testing.T) {
	r := NewRouter()

	started := false
	r.OnStartup(func(ctx context.Context) error {
		started = true
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- r.Run(ctx, ":0", nil)
	}()

	// Give Run a moment to execute the startup hook and begin
	// serving, then trigger a clean shutdown.
	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}
	if !started {
		t.Error("expected the startup hook to have run")
	}
}

func TestRouter_OnStartup_ErrorPreventsServing(t *testing.T) {
	r := NewRouter()

	wantErr := context.DeadlineExceeded // any distinct sentinel error
	r.OnStartup(func(ctx context.Context) error {
		return wantErr
	})

	err := r.Run(context.Background(), ":0", nil)
	if err != wantErr {
		t.Fatalf("expected Run to return the startup hook's error, got: %v", err)
	}
}

func TestRouter_OnShutdown_RunsInLIFOOrder(t *testing.T) {
	r := NewRouter()

	var order []int
	r.OnShutdown(func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})
	r.OnShutdown(func(ctx context.Context) error {
		order = append(order, 2)
		return nil
	})
	r.OnShutdown(func(ctx context.Context) error {
		order = append(order, 3)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- r.Run(ctx, ":0", nil)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run returned an error: %v", err)
	}

	want := []int{3, 2, 1}
	if len(order) != len(want) {
		t.Fatalf("expected %d shutdown hooks to run, got %d", len(want), len(order))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("shutdown hook order mismatch at index %d: want %d, got %d", i, want[i], order[i])
		}
	}
}