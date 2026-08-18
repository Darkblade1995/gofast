package codegen

import (
	"os"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := tmpDir + "/fake.go"

	if err := os.WriteFile(tmpDir+"/go.mod", []byte("module fake\n"), 0600); err != nil {
		t.Fatalf("failed to write fake go.mod: %v", err)
	}

	if err := os.WriteFile(sourcePath, []byte("package main\n"), 0600); err != nil {
		t.Fatalf("failed to write fake source: %v", err)
	}

	m, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	matches, err := m.HashMatches(sourcePath)
	if err != nil {
		t.Fatalf("HashMatches failed: %v", err)
	}
	if matches {
		t.Fatal("expected no match on first run, got match")
	}

	if err := m.Update(sourcePath, []string{"fake_validate.gen.go"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if err := m.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	m2, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("second LoadManifest failed: %v", err)
	}

	matches, err = m2.HashMatches(sourcePath)
	if err != nil {
		t.Fatalf("second HashMatches failed: %v", err)
	}
	if !matches {
		t.Fatal("expected match after save+reload, got no match")
	}

	if err := os.WriteFile(sourcePath, []byte("package main\n\n// changed\n"), 0600); err != nil {
		t.Fatalf("failed to modify fake source: %v", err)
	}

	matches, err = m2.HashMatches(sourcePath)
	if err != nil {
		t.Fatalf("third HashMatches failed: %v", err)
	}
	if matches {
		t.Fatal("expected no match after source changed, got match")
	}
}
