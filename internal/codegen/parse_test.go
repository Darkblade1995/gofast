package codegen

import (
	"fmt"
	"testing"
)

func TestParseFile(t *testing.T) {
	parsed, err := ParseFile("../../examples/minimal/main.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	fmt.Printf("package %s\n", parsed.PackageName)
	for _, s := range parsed.Structs {
		fmt.Printf("struct %s\n", s.Name)
		for _, f := range s.Fields {
			fmt.Printf("  %s %s %s\n", f.Name, f.Type, f.Tag)
		}
	}

	if parsed.PackageName == "" {
		t.Fatal("expected a package name, got empty string")
	}
	if len(parsed.Structs) == 0 {
		t.Fatal("expected at least one struct, found none")
	}
}
