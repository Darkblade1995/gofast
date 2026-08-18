package benchmarks

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

type ComplexHumaReceiverInfo struct {
	AccountID string `json:"accountId" required:"true"`
}

type ComplexHumaTransferInput struct {
	Amount   int64                    `json:"amount"`
	Receiver *ComplexHumaReceiverInfo `json:"receiver,omitempty"`
	Tags     []string                 `json:"tags" minItems:"0" items:"required" uniqueItems:"false"`
}

var humaComplexRegistry = huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
var humaComplexSchema = humaComplexRegistry.Schema(reflect.TypeOf(ComplexHumaTransferInput{}), false, "ComplexHumaTransferInput")

var errSinkHumaComplex error

func BenchmarkHumaValidateComplex(b *testing.B) {
	inputs := make([]ComplexHumaTransferInput, 1000)
	for i := range inputs {
		inputs[i] = ComplexHumaTransferInput{
			Amount:   int64(i),
			Receiver: &ComplexHumaReceiverInfo{AccountID: fmt.Sprintf("acc-%d", i)},
			Tags:     []string{"tag1", "tag2", "tag3"},
		}
	}

	pb := huma.NewPathBuffer([]byte(""), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in := inputs[i%len(inputs)]
		errs := &huma.ValidateResult{}
		huma.Validate(humaComplexRegistry, humaComplexSchema, pb, huma.ModeWriteToServer, in, errs)
		if len(errs.Errors) > 0 {
			errSinkHumaComplex = errs.Errors[0]
		}
	}
}