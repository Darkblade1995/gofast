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

	generatedPath := tmpDir + "/fake_validate.gen.go"
	if err := os.WriteFile(generatedPath, []byte("package main\n"), 0600); err != nil {
		t.Fatalf("failed to write fake generated file: %v", err)
	}

	if err := m.Update(sourcePath, []string{generatedPath}); err != nil {
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

func TestManifest_RegeneratesIfGeneratedFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := tmpDir + "/fake.go"
	generatedPath := tmpDir + "/fake_validate.gen.go"

	if err := os.WriteFile(tmpDir+"/go.mod", []byte("module fake\n"), 0600); err != nil {
		t.Fatalf("failed to write fake go.mod: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("package main\n"), 0600); err != nil {
		t.Fatalf("failed to write fake source: %v", err)
	}
	if err := os.WriteFile(generatedPath, []byte("package main\n"), 0600); err != nil {
		t.Fatalf("failed to write fake generated file: %v", err)
	}

	m, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if err := m.Update(sourcePath, []string{generatedPath}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if err := m.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Source unchanged, but the generated file is deleted externally.
	if err := os.Remove(generatedPath); err != nil {
		t.Fatalf("failed to remove generated file: %v", err)
	}

	m2, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("second LoadManifest failed: %v", err)
	}

	matches, err := m2.HashMatches(sourcePath)
	if err != nil {
		t.Fatalf("HashMatches failed: %v", err)
	}
	if matches {
		t.Error("expected HashMatches to report no match when a recorded generated file is missing, even though the source hash is unchanged")
	}
}