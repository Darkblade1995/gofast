// internal/codegen/generate_dive_test.go
package codegen

import (
	"go/format"
	"strings"
	"testing"
)

func TestGenerateValidateDiveSlice(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "TagsInput",
			Fields: []Field{
				{Name: "Tags", Type: "[]string", Tag: "`validate:\"dive,min=3\"`"},
			},
		},
	}

	output := GenerateValidate("main", structs)
	formatted, err := format.Source([]byte(output))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n%s", err, output)
	}

	code := string(formatted)
	t.Logf("generated code:\n%s", code)

	if !strings.Contains(code, "for idx, v := range in.Tags") {
		t.Error("expected a range loop over Tags")
	}
	if !strings.Contains(code, "len(v) < 3") {
		t.Error("expected a min length check inside the loop")
	}
}

func TestGenerateValidateDiveMapKey(t *testing.T) {
	structs := []StructInfo{
		{
			Name: "ConfigInput",
			Fields: []Field{
				{Name: "Roles", Type: "map[string]string", Tag: "`validate:\"dive_key,min=3\"`"},
			},
		},
	}

	output := GenerateValidate("main", structs)
	formatted, err := format.Source([]byte(output))
	if err != nil {
		t.Fatalf("generated code is not valid Go: %v\n\n%s", err, output)
	}

	code := string(formatted)
	t.Logf("generated code:\n%s", code)

	if !strings.Contains(code, "for k := range in.Roles") {
		t.Error("expected a range loop over Roles keys")
	}
	if !strings.Contains(code, "len(k) < 3") {
		t.Error("expected a min length check on the key")
	}
}
