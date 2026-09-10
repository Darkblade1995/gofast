package gofast

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testEmptyInput struct{}
type testEmptyOutput struct{}

func TestHandler_AcceptsValidMultipart(t *testing.T) {
	fn := func(ctx context.Context, in testEmptyInput) (testEmptyOutput, error) {
		return testEmptyOutput{}, nil
	}

	info := Handler(fn)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("title", "hello"); err != nil {
		t.Fatalf("failed to write multipart field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	info.Func(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for well-formed multipart, got %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestHandler_RejectsMalformedMultipart(t *testing.T) {
	fn := func(ctx context.Context, in testEmptyInput) (testEmptyOutput, error) {
		return testEmptyOutput{}, nil
	}

	info := Handler(fn)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not actually multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")

	rec := httptest.NewRecorder()
	info.Func(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for malformed multipart, got %d", http.StatusBadRequest, rec.Code)
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