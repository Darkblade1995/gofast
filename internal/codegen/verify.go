package codegen

import "fmt"

type StaleFile struct {
	SourcePath string
}

func VerifyUpToDate(startDir string, sourcePaths []string) ([]StaleFile, error) {
	manifest, err := LoadManifest(startDir)
	if err != nil {
		return nil, err
	}

	var stale []StaleFile

	for _, sourcePath := range sourcePaths {
		matches, err := manifest.HashMatches(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("checking %s: %w", sourcePath, err)
		}
		if !matches {
			stale = append(stale, StaleFile{SourcePath: sourcePath})
		}
	}

	return stale, nil
}
