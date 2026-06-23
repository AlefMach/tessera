package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatal(err)
		}
	})

	configPath := filepath.Join(tempDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`
provider = "file-provider"
model = "file-model"
context_tokens = 1234
`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TESSERA_PROVIDER", "env-provider")
	t.Setenv("TESSERA_MODEL", "env-model")

	cfg, err := Load(Flags{
		Provider:      "flag-provider",
		ConfigPath:    configPath,
		ContextTokens: 2048,
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Provider != "flag-provider" {
		t.Fatalf("expected flag provider, got %q", cfg.Provider)
	}
	if cfg.Model != "env-model" {
		t.Fatalf("expected env model, got %q", cfg.Model)
	}
	if cfg.ContextTokens != 2048 {
		t.Fatalf("expected flag context tokens, got %d", cfg.ContextTokens)
	}
}

func TestLoadDefaults(t *testing.T) {
	tempDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatal(err)
		}
	})

	cfg, err := Load(Flags{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Provider != defaultProvider {
		t.Fatalf("expected default provider, got %q", cfg.Provider)
	}
	if cfg.Model != defaultModel {
		t.Fatalf("expected default model, got %q", cfg.Model)
	}
	if cfg.SQLitePath != filepath.Join(tempDir, ".tessera", "memory.db") {
		t.Fatalf("unexpected sqlite path: %q", cfg.SQLitePath)
	}
}
