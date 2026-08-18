package codegen

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrProjectRootNotFound = errors.New("could not find go.mod in any parent directory")

func FindProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrProjectRootNotFound
		}
		dir = parent
	}
}
