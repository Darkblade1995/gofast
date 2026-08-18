// internal/codegen/generate_nested_test.go
package codegen

import (
	"go/format"
	"testing"
)

func TestGenerateValidateNested(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "ReceiverInfo",
			Fields: []Field{
				{Name: "AccountID", Type: "string", Tag: "`validate:\"required\"`"},
			},
		},
		{
			Name: "TransferInput",
			Fields: []Field{
				{Name: "Amount", Type: "int64", Tag: ""},
				{Name: "Receiver", Type: "*ReceiverInfo", Tag: ""},
				{Name: "Backups", Type: "[]ReceiverInfo", Tag: ""},
				{Name: "ByCurrency", Type: "map[string]ReceiverInfo", Tag: ""},
			},
		},
	}

	output := GenerateValidate("main", structs)
	if output == "" {
		t.Fatal("expected generated output, got empty string")
	}

	formatted, err := format.Source([]byte(output))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n%s", err, output)
	}

	t.Logf("generated code:\n%s", formatted)
}
