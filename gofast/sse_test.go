package gofast

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterSSE_SendsCorrectFormat(t *testing.T) {
	router := NewRouter()
	router.RegisterSSE("/events", func(ctx context.Context, stream *SSEStream) error {
		return stream.Send("greeting", map[string]string{"message": "hello"})
	})

	req := httptest.NewRequest("GET", "/events", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "event: greeting\n") {
		t.Errorf("expected body to contain %q, got: %q", "event: greeting\n", body)
	}
	if !strings.Contains(body, `data: {"message":"hello"}`) {
		t.Errorf("expected body to contain the JSON data line, got: %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("expected event to end with a blank line, got: %q", body)
	}
}

func TestRegisterSSE_MultipleEvents(t *testing.T) {
	router := NewRouter()
	router.RegisterSSE("/events", func(ctx context.Context, stream *SSEStream) error {
		if err := stream.Send("first", "one"); err != nil {
			return err
		}
		return stream.Send("second", "two")
	})

	req := httptest.NewRequest("GET", "/events", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	firstIdx := strings.Index(body, "event: first")
	secondIdx := strings.Index(body, "event: second")

	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("expected both events present, got: %q", body)
	}
	if firstIdx > secondIdx {
		t.Errorf("expected 'first' event before 'second', got: %q", body)
	}
}

func TestRegisterSSE_SetsCorrectHeaders(t *testing.T) {
	router := NewRouter()
	router.RegisterSSE("/events", func(ctx context.Context, stream *SSEStream) error {
		return nil
	})

	req := httptest.NewRequest("GET", "/events", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type %q, got %q", "text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control %q, got %q", "no-cache", cc)
	}
	if conn := rec.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection %q, got %q", "keep-alive", conn)
	}
}

func TestRegisterSSE_HandlerReceivesRequestContext(t *testing.T) {
	router := NewRouter()

	type ctxKey struct{}
	req := httptest.NewRequest("GET", "/events", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, "marker"))

	var receivedValue any
	router.RegisterSSE("/events", func(ctx context.Context, stream *SSEStream) error {
		receivedValue = ctx.Value(ctxKey{})
		return nil
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if receivedValue != "marker" {
		t.Errorf("expected the handler to receive the request's own context (with the test marker value), got %v", receivedValue)
	}
}