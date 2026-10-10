// internal/codegen/generate_test.go
package codegen

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// testFset and testImporter are shared across tests: the "source"
// importer type-checks fmt/strings from GOROOT and caches the result,
// so only the first test pays that cost.
var (
	testFset     = token.NewFileSet()
	testImporter = importer.ForCompiler(testFset, "source", nil)
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

// structDecls rebuilds the type declarations the generated code needs
// in order to type-check (the Validate() methods hang off these structs).
func structDecls(structs []StructInfo) string {
	var b strings.Builder
	for _, s := range structs {
		fmt.Fprintf(&b, "type %s struct {\n", s.Name)
		for _, f := range s.Fields {
			fmt.Fprintf(&b, "\t%s %s %s\n", f.Name, f.Type, f.Tag)
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

// typeCheck runs go/types over the generated code and returns ALL the
// errors the Go compiler would report. Unlike format.Source, which only
// checks syntax, this catches unused imports and unused variables.
func typeCheck(generated string, structs []StructInfo) []error {
	src := generated + "\n" + structDecls(structs)

	file, err := parser.ParseFile(testFset, "generated.go", src, 0)
	if err != nil {
		return []error{err}
	}

	var errs []error
	conf := types.Config{
		Importer: testImporter,
		Error:    func(err error) { errs = append(errs, err) },
	}
	_, _ = conf.Check("generated", testFset, []*ast.File{file}, nil)

	return errs
}

func TestGeneratedValidateCompiles(t *testing.T) {
	tests := []struct {
		name    string
		structs []StructInfo
	}{
		{
			name: "control: required+email+min (every import is used)",
			structs: []StructInfo{{
				Name: "LoginInput",
				Fields: []Field{
					{Name: "Email", Type: "string", Tag: "`validate:\"required,email\"`"},
					{Name: "Password", Type: "string", Tag: "`validate:\"required,min=8\"`"},
				},
			}},
		},
		{
			name: "no email rule: strings must not be imported (friction #4)",
			structs: []StructInfo{{
				Name: "CreateTeamInput",
				Fields: []Field{
					{Name: "Name", Type: "string", Tag: "`validate:\"required,min=3\"`"},
				},
			}},
		},
		{
			name: "only unknown rules: no import must be emitted",
			structs: []StructInfo{{
				Name: "RoleInput",
				Fields: []Field{
					{Name: "Role", Type: "string", Tag: "`validate:\"oneof=admin member\"`"},
				},
			}},
		},
		{
			name: "dive with unknown inner rule: must not emit an empty for loop",
			structs: []StructInfo{{
				Name: "TagsInput",
				Fields: []Field{
					{Name: "Tags", Type: "[]string", Tag: "`validate:\"dive,email\"`"},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := GenerateValidate("main", tt.structs)
			if output == "" {
				t.Fatal("expected generated output, got empty string")
			}

			for _, err := range typeCheck(output, tt.structs) {
				t.Errorf("generated code does not compile: %v", err)
			}

			if t.Failed() {
				t.Logf("generated code:\n%s", output)
			}
		})
	}
}