// internal/codegen/manifest.go
package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type manifestEntry struct {
	Hash      string   `json:"hash"`
	Generated []string `json:"generated"`
}

type Manifest struct {
	path    string
	Entries map[string]manifestEntry `json:"entries"`
}

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
		return m, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manifest) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0600)
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

	return entry.Hash == currentHash, nil
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
