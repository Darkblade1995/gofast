package main

import (
	"testing"

		"github.com/Darkblade1995/gofast/internal/codegen"
)

func TestGeneratedCodeIsUpToDate(t *testing.T) {
	stale, err := codegen.VerifyUpToDate(".", []string{"./main.go"})
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	for _, s := range stale {
		t.Errorf("%s has changed since last generation — run `gofast generate %s`", s.SourcePath, s.SourcePath)
	}
}
