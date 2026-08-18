package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseFile(f *testing.F) {
	seeds := []string{
		`package main

type LoginInput struct {
	Email    string ` + "`json:\"email\" validate:\"required,email\"`" + `
	Password string ` + "`json:\"password\" validate:\"required,min=8\"`" + `
}
`,
		`package main

type Empty struct{}
`,
		`package main

type Nested struct {
	ID   string ` + "`path:\"id\"`" + `
	Tags []string ` + "`validate:\"dive,min=3\"`" + `
	Meta map[string]string
	Next *Nested
}
`,
		`package main
`,
		``,
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "fuzz.go")

		if err := os.WriteFile(tmpFile, []byte(src), 0600); err != nil {
			t.Skip()
		}

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseFile panicked on input %q: %v", src, r)
			}
		}()

		_, _ = ParseFile(tmpFile)
	})
}