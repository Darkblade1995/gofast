// benchmarks/huma_validate_bench_test.go
package benchmarks

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

type HumaLoginInput struct {
	Email    string `json:"email" required:"true" format:"email"`
	Password string `json:"password" required:"true" minLength:"8"`
}

var humaRegistry = huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
var humaSchema = humaRegistry.Schema(reflect.TypeOf(HumaLoginInput{}), false, "HumaLoginInput")

var errSinkHuma error

func BenchmarkHumaValidate(b *testing.B) {
	inputs := make([]HumaLoginInput, 1000)
	for i := range inputs {
		inputs[i] = HumaLoginInput{
			Email:    "user" + strconv.Itoa(i) + "@example.com",
			Password: "supersecret123",
		}
	}

	pb := huma.NewPathBuffer([]byte(""), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in := inputs[i%len(inputs)]
		errs := &huma.ValidateResult{}
		huma.Validate(humaRegistry, humaSchema, pb, huma.ModeWriteToServer, in, errs)
		if len(errs.Errors) > 0 {
			errSinkHuma = errs.Errors[0]
		}
	}
}
