package trust

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustPersistsFolder(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStoreAt(filepath.Join(tempDir, "trusted.json"))

	trusted, err := store.IsTrusted(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if trusted {
		t.Fatal("expected folder to start untrusted")
	}

	if err := store.Trust(projectDir); err != nil {
		t.Fatal(err)
	}

	trusted, err = store.IsTrusted(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if !trusted {
		t.Fatal("expected folder to be trusted")
	}
}
