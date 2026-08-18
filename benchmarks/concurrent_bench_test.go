
package benchmarks

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofast/gofast"
)

func BenchmarkGoFastConcurrent(b *testing.B) {
	router := gofast.NewRouter()
	router.Register("POST", "/login", gofast.Handler(gofastHTTPCycleLogin))

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

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status: %d, body: %s", rec.Code, rec.Body.String())
			}
			i++
		}
	})
}