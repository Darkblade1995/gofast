// internal/codegen/project_test.go
package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module fake\n"), 0600); err != nil {
		t.Fatalf("failed to create fake go.mod: %v", err)
	}

	nested := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0750); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	root, err := FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot failed: %v", err)
	}

	absTmpDir, _ := filepath.Abs(tmpDir)
	if root != absTmpDir {
		t.Fatalf("expected root %s, got %s", absTmpDir, root)
	}
}

func TestFindProjectRootNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := FindProjectRoot(tmpDir)
	if err != ErrProjectRootNotFound {
		t.Fatalf("expected ErrProjectRootNotFound, got %v", err)
	}
}
