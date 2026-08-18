// internal/codegen/generate_deep_nested_test.go
package codegen

import (
	"go/format"
	"strings"
	"testing"
)

func TestGenerateValidateDeepNesting(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "Level4",
			Fields: []Field{
				{Name: "Code", Type: "string", Tag: "`validate:\"required\"`"},
			},
		},
		{
			Name: "Level3",
			Fields: []Field{
				{Name: "Next", Type: "*Level4", Tag: ""},
			},
		},
		{
			Name: "Level2",
			Fields: []Field{
				{Name: "Next", Type: "*Level3", Tag: ""},
			},
		},
		{
			Name: "Level1",
			Fields: []Field{
				{Name: "Next", Type: "*Level2", Tag: ""},
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

	code := string(formatted)

	t.Logf("generated code:\n%s", code)

	mustContain := []string{
		"func (in Level1) Validate() error",
		"func (in Level2) Validate() error",
		"func (in Level3) Validate() error",
		"func (in Level4) Validate() error",
		"in.Next.Validate()",
	}

	for _, needle := range mustContain {
		if !strings.Contains(code, needle) {
			t.Errorf("expected generated code to contain %q, but it did not", needle)
		}
	}
}
