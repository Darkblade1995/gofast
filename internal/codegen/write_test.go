package codegen

import (
	"os"
	"testing"
)

func TestGenerateAndWrite(t *testing.T) {
	sourcePath := "../../examples/minimal/main.go"

	outPaths, err := GenerateAndWrite(sourcePath)
	if err != nil {
		t.Fatalf("GenerateAndWrite failed: %v", err)
	}
	if len(outPaths) == 0 {
		t.Fatal("expected at least one generated file, got none")
	}

	for _, outPath := range outPaths {
		defer func(p string) {
			if err := os.Remove(p); err != nil {
				t.Logf("cleanup: failed to remove %s: %v", p, err)
			}
		}(outPath)

		// #nosec G304 -- outPath was just produced by
		// GenerateAndWrite in this same test, not from
		// untrusted input.
		content, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("failed to read generated file %s: %v", outPath, err)
		}

		t.Logf("generated file at %s:\n%s", outPath, content)
	}
}
