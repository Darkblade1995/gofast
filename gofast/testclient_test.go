package gofast

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestTestClient_BasicGET(t *testing.T) {
	router := NewRouter()
	router.Register("GET", "/ping", Handler(func(ctx context.Context, in struct{}) (map[string]string, error) {
		return map[string]string{"message": "pong"}, nil
	}))

	client := NewTestClient(router)
	resp := client.GET(t, "/ping")

	resp.AssertStatus(t, http.StatusOK)

	var out map[string]string
	resp.DecodeJSON(t, &out)
	if out["message"] != "pong" {
		t.Errorf("expected message %q, got %q", "pong", out["message"])
	}
}

func TestTestClient_POSTWithJSONBody(t *testing.T) {
	type EchoInput struct {
		Text string `json:"text"`
	}
	type EchoOutput struct {
		Echoed string `json:"echoed"`
	}

	router := NewRouter()
	router.Register("POST", "/echo", Handler(func(ctx context.Context, in EchoInput) (EchoOutput, error) {
		return EchoOutput{Echoed: in.Text}, nil
	}))

	client := NewTestClient(router)
	resp := client.POST(t, "/echo", EchoInput{Text: "hello"})

	resp.AssertStatus(t, http.StatusOK)

	var out EchoOutput
	resp.DecodeJSON(t, &out)
	if out.Echoed != "hello" {
		t.Errorf("expected echoed %q, got %q", "hello", out.Echoed)
	}
}

func TestTestClient_AuthenticatedFlow(t *testing.T) {
	secret := []byte("test-client-secret")

	type ProtectedOutput struct {
		Subject string `json:"subject"`
	}

	router := NewRouter()
	router.Register("GET", "/protected", Handler(func(ctx context.Context, in struct{}) (ProtectedOutput, error) {
		claims, _ := ClaimsFromContext(ctx)
		return ProtectedOutput{Subject: claims.Subject}, nil
	}).Wrap(AuthMiddleware(secret, nil)))

	client := NewTestClient(router)

	// Without a token: rejected.
	resp := client.GET(t, "/protected")
	resp.AssertStatus(t, http.StatusUnauthorized)

	// Issue a token and set it on the client — mirrors a real
	// login flow.
	token, err := IssueAccessToken(secret, "user-42", time.Hour)
	if err != nil {
		t.Fatalf("failed to issue test token: %v", err)
	}
	client.SetAuthToken(token)

	resp = client.GET(t, "/protected")
	resp.AssertStatus(t, http.StatusOK)

	var out ProtectedOutput
	resp.DecodeJSON(t, &out)
	if out.Subject != "user-42" {
		t.Errorf("expected subject %q, got %q", "user-42", out.Subject)
	}
}