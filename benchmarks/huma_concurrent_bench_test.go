package benchmarks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func BenchmarkHumaConcurrent(b *testing.B) {
	_, api := humatest.New(b)

	huma.Register(api, huma.Operation{
		OperationID: "login-concurrent",
		Method:      http.MethodPost,
		Path:        "/login",
	}, func(ctx context.Context, in *HTTPCycleHumaLoginInput) (*HTTPCycleHumaLoginOutput, error) {
		out := &HTTPCycleHumaLoginOutput{}
		out.Body.Token = "token-for-" + in.Body.Email
		return out, nil
	})

	bodies := make([][]byte, 1000)
	for i := range bodies {
		bodies[i] = []byte(fmt.Sprintf(`{"email":"user%d@example.com","password":"supersecret123"}`, i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(bodies[i%len(bodies)]))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			api.Adapter().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status: %d, body: %s", rec.Code, rec.Body.String())
			}
			i++
		}
	})
}