package benchmarks

import (
	"fmt"
	"testing"

	"gofast/gofast"
)

type ComplexReceiverInfo struct {
	AccountID string
}

//go:noinline
func (in ComplexReceiverInfo) Validate() error {
	if in.AccountID == "" {
		return gofast.NewBusinessError(gofast.ErrCodeValidation, 400, "AccountID is required")
	}
	return nil
}

type ComplexTransferInput struct {
	Amount   int64
	Receiver *ComplexReceiverInfo
	Tags     []string
}

//go:noinline
func (in ComplexTransferInput) Validate() error {
	if in.Receiver != nil {
		if err := in.Receiver.Validate(); err != nil {
			return err
		}
	}
	for idx, v := range in.Tags {
		if len(v) < 3 {
			return gofast.NewBusinessError(gofast.ErrCodeValidation, 400, fmt.Sprintf("Tags[%d] must be at least 3 characters", idx))
		}
	}
	return nil
}

var errSinkComplex error

func BenchmarkGoFastValidateComplex(b *testing.B) {
	inputs := make([]ComplexTransferInput, 1000)
	for i := range inputs {
		inputs[i] = ComplexTransferInput{
			Amount:   int64(i),
			Receiver: &ComplexReceiverInfo{AccountID: fmt.Sprintf("acc-%d", i)},
			Tags:     []string{"tag1", "tag2", "tag3"},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errSinkComplex = inputs[i%len(inputs)].Validate()
	}
}