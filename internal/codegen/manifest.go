// internal/codegen/manifest.go
package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type manifestEntry struct {
	Hash      string   `json:"hash"`
	Generated []string `json:"generated"`
}

// Manifest tracks, per source file, the hash it was last generated
// from and the files that generation produced. Version increments
// on every successful Save and is used by UpdateAndSave to detect
// concurrent writes from another process — see ADR 0018.
type Manifest struct {
	path            string
	Version         int                       `json:"version"`
	Entries         map[string]manifestEntry `json:"entries"`
	expectedVersion int
}

// errVersionConflict signals that the manifest on disk has a
// different version than expected, meaning another process wrote
// to it since this one loaded it. See ADR 0018.
var errVersionConflict = errors.New("manifest: version conflict, another process wrote concurrently")

// saveMu serializes the full load-update-save cycle in
// UpdateAndSave, so concurrent goroutines within the same process
// cannot interleave their read and write steps. The version check
// in Save still exists and is what detects writes from OTHER
// processes, where this in-process mutex offers no protection.
// See ADR 0018.
var saveMu sync.Mutex

func LoadManifest(startDir string) (*Manifest, error) {
	root, err := FindProjectRoot(startDir)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(root, ".gofast", "manifest.json")

	m := &Manifest{
		path:    path,
		Entries: map[string]manifestEntry{},
	}

	// #nosec G304 -- path is derived from the project root found
	// via go.mod, not from untrusted external input. This is a
	// CLI tool operating on the local developer's own filesystem.
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		m.expectedVersion = m.Version
		return m, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}

	m.expectedVersion = m.Version
	return m, nil
}

// Save writes the manifest to disk, incrementing Version. If the
// file on disk has a different version than this Manifest was
// loaded with, Save returns errVersionConflict without writing —
// another process wrote concurrently, and the caller (typically
// UpdateAndSave) should reload and retry. See ADR 0018.
func (m *Manifest) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0750); err != nil {
		return err
	}

	// #nosec G304 -- path is derived from the project root found
	// via go.mod, not from untrusted external input.
	existing, err := os.ReadFile(m.path)
	if err == nil {
		var onDisk Manifest
		if err := json.Unmarshal(existing, &onDisk); err == nil {
			if onDisk.Version != m.expectedVersion {
				return errVersionConflict
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	m.Version = m.expectedVersion + 1

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(m.path, data, 0600); err != nil {
		return err
	}

	m.expectedVersion = m.Version
	return nil
}

// UpdateAndSave applies Update(sourcePath, generatedFiles) and
// saves the manifest, retrying automatically (up to 5 attempts) if
// another process wrote to the manifest concurrently. The entire
// load-update-save cycle runs under saveMu so concurrent goroutines
// within this same process are fully serialized, not just each
// individual Save call — otherwise, many competing goroutines could
// each load a now-stale version between their own retries and
// exhaust their retry budget before ever succeeding. Each retry
// reloads the manifest fresh from disk and re-applies this
// process's own Update on top of the latest state, so no other
// process's entry is lost. See ADR 0018.
func UpdateAndSave(startDir, sourcePath string, generatedFiles []string) error {
	saveMu.Lock()
	defer saveMu.Unlock()

	const maxRetries = 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		m, err := LoadManifest(startDir)
		if err != nil {
			return err
		}

		if err := m.Update(sourcePath, generatedFiles); err != nil {
			return err
		}

		err = m.Save()
		if err == nil {
			return nil
		}
		if !errors.Is(err, errVersionConflict) {
			return err
		}
		// Conflict detected — loop reloads fresh and retries.
	}

	return errors.New("manifest: failed to save after concurrent write retries exhausted")
}

func (m *Manifest) HashMatches(sourcePath string) (bool, error) {
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return false, err
	}

	currentHash, err := hashFile(sourcePath)
	if err != nil {
		return false, err
	}

	entry, ok := m.Entries[absPath]
	if !ok {
		return false, nil
	}

	if entry.Hash != currentHash {
		return false, nil
	}

	for _, generatedPath := range entry.Generated {
		if _, err := os.Stat(generatedPath); err != nil {
			return false, nil
		}
	}

	return true, nil
}

func (m *Manifest) Update(sourcePath string, generatedFiles []string) error {
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}

	hash, err := hashFile(sourcePath)
	if err != nil {
		return err
	}

	m.Entries[absPath] = manifestEntry{
		Hash:      hash,
		Generated: generatedFiles,
	}

	return nil
}

func hashFile(path string) (string, error) {
	// #nosec G304 -- path is the source file the CLI user
	// explicitly passed as an argument (e.g. `gofast generate
	// ./main.go`), not untrusted external input.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}