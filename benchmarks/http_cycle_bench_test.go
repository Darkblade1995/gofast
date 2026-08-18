package benchmarks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofast/gofast"
)

type HTTPCycleLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

//go:noinline
func (in HTTPCycleLoginInput) Validate() error {
	if in.Email == "" {
		return gofast.NewBusinessError(gofast.ErrCodeValidation, 400, "Email is required")
	}
	if in.Password == "" {
		return gofast.NewBusinessError(gofast.ErrCodeValidation, 400, "Password is required")
	}
	if len(in.Password) < 8 {
		return gofast.NewBusinessError(gofast.ErrCodeValidation, 400, "Password must be at least 8 characters")
	}
	return nil
}

type HTTPCycleLoginOutput struct {
	Token string `json:"token"`
}

func gofastHTTPCycleLogin(ctx context.Context, in HTTPCycleLoginInput) (HTTPCycleLoginOutput, error) {
	return HTTPCycleLoginOutput{Token: "token-for-" + in.Email}, nil
}

func BenchmarkGoFastHTTPCycle(b *testing.B) {
	router := gofast.NewRouter()
	router.Register("POST", "/login", gofast.Handler(gofastHTTPCycleLogin))

	bodies := make([][]byte, 1000)
	for i := range bodies {
		bodies[i] = []byte(fmt.Sprintf(`{"email":"user%d@example.com","password":"supersecret123"}`, i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(bodies[i%len(bodies)]))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d, body: %s", rec.Code, rec.Body.String())
		}
	}
}