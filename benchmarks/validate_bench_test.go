package benchmarks

import (
	"strconv"
	"testing"

	"gofast/gofast"
)

type GoFastLoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

//go:noinline
func (in GoFastLoginInput) Validate() error {
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

var errSink error

func BenchmarkGoFastValidate(b *testing.B) {
	inputs := make([]GoFastLoginInput, 1000)
	for i := range inputs {
		inputs[i] = GoFastLoginInput{
			Email:    "user" + strconv.Itoa(i) + "@example.com",
			Password: "supersecret123",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errSink = inputs[i%len(inputs)].Validate()
	}
}
