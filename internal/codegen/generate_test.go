package codegen

import (
	"go/format"
	"testing"
)

func TestGenerateValidate(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "LoginInput",
			Fields: []Field{
				{Name: "Email", Type: "string", Tag: "`json:\"email\" validate:\"required,email\"`"},
				{Name: "Password", Type: "string", Tag: "`json:\"password\" validate:\"required,min=8\"`"},
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
