package gofast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testEmptyInput struct{}
type testEmptyOutput struct{}

func TestHandlerRejectsMultipart(t *testing.T) {
	fn := func(ctx context.Context, in testEmptyInput) (testEmptyOutput, error) {
		return testEmptyOutput{}, nil
	}

	info := Handler(fn)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("fake multipart body"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")

	rec := httptest.NewRecorder()
	info.Func(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status %d, got %d", http.StatusUnsupportedMediaType, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "UNSUPPORTED_MEDIA_TYPE") {
		t.Errorf("expected UNSUPPORTED_MEDIA_TYPE in response body, got: %s", rec.Body.String())
	}
}

func TestHandlerAcceptsJSON(t *testing.T) {
	fn := func(ctx context.Context, in testEmptyInput) (testEmptyOutput, error) {
		return testEmptyOutput{}, nil
	}

	info := Handler(fn)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	info.Func(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}
