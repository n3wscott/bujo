package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadmeGeneratedCommandsSectionIsCurrent(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	readmePath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "README.md"))
	contentBytes, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	content := string(contentBytes)

	updated, err := ReplaceGeneratedCommandsSection(content)
	if err != nil {
		t.Fatalf("generate README command section: %v", err)
	}
	if content != updated {
		t.Fatalf("README generated command section is stale; run: go run ./cmd/gendocs --write")
	}

	start := strings.Index(content, ReadmeCommandsStartMarker)
	end := strings.Index(content, ReadmeCommandsEndMarker)
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("README markers missing or invalid")
	}
}
