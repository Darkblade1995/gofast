package gofast_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gofast/gofast"
)

func TestHealthCheckHealthy(t *testing.T) {
	router := gofast.NewRouter()
	router.HealthCheck("/health", func(ctx context.Context) error {
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("expected ok status in body, got: %s", rec.Body.String())
	}
}

func TestHealthCheckUnhealthy(t *testing.T) {
	router := gofast.NewRouter()
	router.HealthCheck("/health", func(ctx context.Context) error {
		return errors.New("database unreachable")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "database unreachable") {
		t.Errorf("expected error detail in body, got: %s", rec.Body.String())
	}
}
