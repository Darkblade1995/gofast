package codegen

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

func outputPathFor(sourcePath, suffix string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	name := strings.TrimSuffix(base, ".go")
	return filepath.Join(dir, name+suffix)
}

func OutputPath(sourcePath string) string {
	return outputPathFor(sourcePath, "_validate.gen.go")
}

func writeGenerated(sourcePath, suffix, code string) (string, error) {
	if code == "" {
		return "", nil
	}

	formatted, err := format.Source([]byte(code))
	if err != nil {
		return "", err
	}

	outPath := outputPathFor(sourcePath, suffix)
	if err := os.WriteFile(outPath, formatted, 0600); err != nil {
		return "", err
	}

	return outPath, nil
}

func GenerateAndWrite(sourcePath string) ([]string, error) {
	parsed, err := ParseFile(sourcePath)
	if err != nil {
		return nil, err
	}

	var written []string

	validateCode := GenerateValidate(parsed.PackageName, parsed.Structs)
	if out, err := writeGenerated(sourcePath, "_validate.gen.go", validateCode); err != nil {
		return nil, err
	} else if out != "" {
		written = append(written, out)
	}

	bindPathCode := GenerateBindPath(parsed.PackageName, parsed.Structs)
	if out, err := writeGenerated(sourcePath, "_bindpath.gen.go", bindPathCode); err != nil {
		return nil, err
	} else if out != "" {
		written = append(written, out)
	}

	bindQueryCode := GenerateBindQuery(parsed.PackageName, parsed.Structs)
	if out, err := writeGenerated(sourcePath, "_bindquery.gen.go", bindQueryCode); err != nil {
		return nil, err
	} else if out != "" {
		written = append(written, out)
	}

	return written, nil
}
