package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndWrite(t *testing.T) {
	// This test copies main.go into a temporary directory before
	// generating, rather than running GenerateAndWrite directly
	// against examples/minimal/main.go. Generating in place would
	// write real .gen.go files into the example — files meant to
	// be committed to the repository — and then this test's
	// cleanup would delete them, corrupting the example for
	// anyone else who runs `go test ./...` afterward.
	srcContent, err := os.ReadFile("../../examples/minimal/main.go")
	if err != nil {
		t.Fatalf("failed to read source fixture: %v", err)
	}

	tmpDir := t.TempDir()
	tmpMainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(tmpMainPath, srcContent, 0o644); err != nil {
		t.Fatalf("failed to write temp source file: %v", err)
	}

	outPaths, err := GenerateAndWrite(tmpMainPath)
	if err != nil {
		t.Fatalf("GenerateAndWrite failed: %v", err)
	}
	if len(outPaths) == 0 {
		t.Fatal("expected at least one generated file, got none")
	}

	for _, outPath := range outPaths {
		// #nosec G304 -- outPath was just produced by
		// GenerateAndWrite in this same test, not from
		// untrusted input.
		content, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("failed to read generated file %s: %v", outPath, err)
		}
		t.Logf("generated file at %s:\n%s", outPath, content)
	}
	// No manual cleanup needed — t.TempDir() removes tmpDir (and
	// everything GenerateAndWrite wrote inside it) automatically
	// when the test finishes.
}