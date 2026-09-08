package gofast

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClient dispatches requests to an http.Handler in-process,
// via httptest.NewRecorder — no real network socket is used. See
// ADR 0015. handler is typically a Router, or a Router wrapped in
// the same middleware chain (CORS, Recovery, Logger) an
// application would pass to Router.Run.
type TestClient struct {
	handler   http.Handler
	authToken string
}

// NewTestClient creates a TestClient dispatching requests to
// handler.
func NewTestClient(handler http.Handler) *TestClient {
	return &TestClient{handler: handler}
}

// SetAuthToken sets the token sent as "Authorization: Bearer
// <token>" on every subsequent request made by this client. Pass
// an empty string to stop sending the header.
func (c *TestClient) SetAuthToken(token string) {
	c.authToken = token
}

// GET sends a GET request to path with no body.
func (c *TestClient) GET(t *testing.T, path string) *TestResponse {
	t.Helper()
	return c.do(t, http.MethodGet, path, nil)
}

// POST sends a POST request to path, marshaling body to JSON. Pass
// nil for no body.
func (c *TestClient) POST(t *testing.T, path string, body any) *TestResponse {
	t.Helper()
	return c.do(t, http.MethodPost, path, body)
}

// PUT sends a PUT request to path, marshaling body to JSON. Pass
// nil for no body.
func (c *TestClient) PUT(t *testing.T, path string, body any) *TestResponse {
	t.Helper()
	return c.do(t, http.MethodPut, path, body)
}

// PATCH sends a PATCH request to path, marshaling body to JSON.
// Pass nil for no body.
func (c *TestClient) PATCH(t *testing.T, path string, body any) *TestResponse {
	t.Helper()
	return c.do(t, http.MethodPatch, path, body)
}

// DELETE sends a DELETE request to path with no body.
func (c *TestClient) DELETE(t *testing.T, path string) *TestResponse {
	t.Helper()
	return c.do(t, http.MethodDelete, path, nil)
}

func (c *TestClient) do(t *testing.T, method, path string, body any) *TestResponse {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("gofast.TestClient: failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)

	return &TestResponse{rec: rec}
}

// TestResponse wraps the result of a TestClient request.
type TestResponse struct {
	rec *httptest.ResponseRecorder
}

// StatusCode returns the response's HTTP status code.
func (r *TestResponse) StatusCode() int {
	return r.rec.Code
}

// Body returns the raw response body bytes.
func (r *TestResponse) Body() []byte {
	return r.rec.Body.Bytes()
}

// AssertStatus fails the test if the response's status code does
// not equal want.
func (r *TestResponse) AssertStatus(t *testing.T, want int) {
	t.Helper()
	if r.rec.Code != want {
		t.Errorf("expected status %d, got %d, body: %s", want, r.rec.Code, r.rec.Body.String())
	}
}

// DecodeJSON decodes the response body as JSON into out, failing
// the test if decoding fails.
func (r *TestResponse) DecodeJSON(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.rec.Body.Bytes(), out); err != nil {
		t.Fatalf("gofast.TestClient: failed to decode JSON response: %v, body: %s", err, r.rec.Body.String())
	}
}