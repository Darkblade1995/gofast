package codegen

import (
	"os"
	"sync"
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

func TestUpdateAndSave_ConcurrentWritesDoNotLoseEntries(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/go.mod", []byte("module fake\n"), 0600); err != nil {
		t.Fatalf("failed to write fake go.mod: %v", err)
	}

	const numConcurrent = 10
	sourcePaths := make([]string, numConcurrent)
	generatedPaths := make([]string, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		sourcePaths[i] = tmpDir + "/" + string(rune('a'+i)) + ".go"
		generatedPaths[i] = tmpDir + "/" + string(rune('a'+i)) + "_validate.gen.go"

		if err := os.WriteFile(sourcePaths[i], []byte("package main\n"), 0600); err != nil {
			t.Fatalf("failed to write fake source %d: %v", i, err)
		}
		if err := os.WriteFile(generatedPaths[i], []byte("package main\n"), 0600); err != nil {
			t.Fatalf("failed to write fake generated file %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = UpdateAndSave(tmpDir, sourcePaths[idx], []string{generatedPaths[idx]})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("UpdateAndSave for source %d failed: %v", i, err)
		}
	}

	final, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("failed to load final manifest: %v", err)
	}

	if len(final.Entries) != numConcurrent {
		t.Errorf("expected %d entries in final manifest, got %d — some concurrent writes were lost", numConcurrent, len(final.Entries))
	}
}