package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileDetectsGoProjectWithGitAndTests(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "main_test.go"), "package main\n")
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	profile, err := Profile(root)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Mode != ModeExistingProject {
		t.Fatalf("expected existing project, got %q", profile.Mode)
	}
	if profile.Stack != "Go" {
		t.Fatalf("expected Go stack, got %q", profile.Stack)
	}
	if !profile.HasGit {
		t.Fatal("expected git to be detected")
	}
	if !profile.HasTests {
		t.Fatal("expected tests to be detected")
	}
	if profile.TestRunner != "go test ./..." {
		t.Fatalf("unexpected test runner: %q", profile.TestRunner)
	}
}

func TestProfileTreatsOnlyLocalMetadataAsEmptyProject(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".tessera"), 0o755); err != nil {
		t.Fatal(err)
	}

	profile, err := Profile(root)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Mode != ModeEmptyProject {
		t.Fatalf("expected empty project, got %q", profile.Mode)
	}
	if profile.Stack != "unknown" {
		t.Fatalf("expected unknown stack, got %q", profile.Stack)
	}
	if !profile.HasGit {
		t.Fatal("expected git to be detected")
	}
}

func TestProfileDetectsNodeLockfileRunner(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"vitest"}}`)
	writeFile(t, filepath.Join(root, "pnpm-lock.yaml"), "")
	if err := os.Mkdir(filepath.Join(root, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}

	profile, err := Profile(root)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Stack != "Node" {
		t.Fatalf("expected Node stack, got %q", profile.Stack)
	}
	if profile.TestRunner != "pnpm test" {
		t.Fatalf("unexpected test runner: %q", profile.TestRunner)
	}
	if !profile.HasTests {
		t.Fatal("expected spec directory to count as tests")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
