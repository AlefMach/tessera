package treesitter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/session"
)

func TestIndexerPersistsGoSymbolsAndRepoMap(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "service.go", `package demo

import "fmt"

const DefaultName = "tessera"

type Service struct{}

func NewService() Service {
	return Service{}
}

func (s Service) Greet(name string) string {
	return fmt.Sprintf("hello %s", name)
}
`)
	writeFile(t, root, "service_test.go", `package demo

func TestGreet(t *testing.T) {}
`)

	store := sqlite.NewMemoryStore(filepath.Join(root, ".tessera", "memory.db"))
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	sess := session.Session{ID: "sess-index", CWD: root, Provider: "ollama", Model: "local", CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	result, err := New(root, store, WithClock(func() time.Time { return now })).Index(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 2 {
		t.Fatalf("expected 2 files, got %d", result.Files)
	}
	if !strings.Contains(result.RepoMap, "function NewService:9-11") || !strings.Contains(result.RepoMap, "function Greet:13-15") {
		t.Fatalf("repo map missing Go functions:\n%s", result.RepoMap)
	}

	summary, err := store.GetFileSummary(ctx, sess.ID, "service.go")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Language != "go" || !summary.HasTestsNearby {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if len(summary.Imports) != 1 || summary.Imports[0] != "fmt" {
		t.Fatalf("unexpected imports: %#v", summary.Imports)
	}
	if !contains(summary.Exports, "Service") || !contains(summary.Exports, "NewService") {
		t.Fatalf("unexpected exports: %#v", summary.Exports)
	}

	symbols, err := store.ListSymbols(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSymbol(symbols, "NewService", "function", 9, 11) || !hasSymbol(symbols, "Service", "type", 7, 7) {
		t.Fatalf("unexpected symbols: %#v", symbols)
	}
}

func TestIndexerUsesRegexFallbackForElixir(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "lib/demo.ex", `defmodule Demo do
  alias Demo.User

  def hello(name) do
    "hello #{name}"
  end
end
`)
	store := sqlite.NewMemoryStore(filepath.Join(root, ".tessera", "memory.db"))
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	sess := session.Session{ID: "sess-elixir", CWD: root, Provider: "ollama", Model: "local", CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	result, err := New(root, store, WithClock(func() time.Time { return now })).Index(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.RepoMap, "fallback=regex") || !strings.Contains(result.RepoMap, "module Demo:1-1") || !strings.Contains(result.RepoMap, "function hello:4-4") {
		t.Fatalf("unexpected repo map:\n%s", result.RepoMap)
	}
}

func TestIndexerSupportsKotlinWithRegexFallback(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "src/UserService.kt", `package demo

import demo.User

data class UserService(val repo: UserRepository) {
  fun createUser(name: String): User {
    return User(name)
  }
}

const val DefaultName = "tessera"
`)
	store := sqlite.NewMemoryStore(filepath.Join(root, ".tessera", "memory.db"))
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	sess := session.Session{ID: "sess-kotlin", CWD: root, Provider: "ollama", Model: "local", CreatedAt: now, UpdatedAt: now}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	result, err := New(root, store, WithClock(func() time.Time { return now })).Index(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.RepoMap, "src/UserService.kt [kotlin] fallback=regex") ||
		!strings.Contains(result.RepoMap, "class UserService:5-5") ||
		!strings.Contains(result.RepoMap, "function createUser:6-6") ||
		!strings.Contains(result.RepoMap, "constant DefaultName:11-11") {
		t.Fatalf("unexpected repo map:\n%s", result.RepoMap)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasSymbol(symbols []memory.Symbol, name, kind string, start, end int) bool {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind && symbol.StartLine == start && symbol.EndLine == end {
			return true
		}
	}
	return false
}
