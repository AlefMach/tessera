package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/port"
)

func TestInteractiveInputCallsLLM(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{
		resp: llm.GenerateResponse{
			Text:         "resposta do modelo",
			Model:        "llama3.2",
			InputTokens:  3,
			OutputTokens: 4,
			Duration:     12 * time.Millisecond,
		},
	}
	ui := &scriptedUI{lines: []string{"explique o projeto\n", "/exit\n"}}
	cfg := config.Config{
		Provider:   "ollama",
		Model:      "llama3.2",
		MaxTokens:  128,
		TesseraDir: t.TempDir(),
	}

	orch := New(model, store, ui, nil, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if len(model.requests) != 1 {
		t.Fatalf("expected 1 LLM request, got %d", len(model.requests))
	}
	req := model.requests[0]
	for _, want := range []string{"# User task\nexplique o projeto", "# Project profile", "# Constraints"} {
		if !strings.Contains(req.Prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, req.Prompt)
		}
	}
	if req.System == "" || !strings.Contains(req.System, "local-first interactive coding agent") {
		t.Fatalf("expected Tessera system prompt, got %q", req.System)
	}
	if req.MaxTokens != 128 || req.SessionID == "" || req.RunID == "" {
		t.Fatalf("unexpected LLM request: %#v", req)
	}
	if !ui.sawEvent("llm.call.started") {
		t.Fatal("expected llm.call.started event")
	}
	if !ui.sawMessage("resposta do modelo") {
		t.Fatalf("expected model response to be rendered, events=%#v", ui.events)
	}

	stats, err := store.Stats(ctx, req.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Runs != 1 {
		t.Fatalf("expected 1 saved run, got %#v", stats)
	}
}

func TestStatusIncludesProjectProfile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/status\n")
	writeTestFile(t, filepath.Join(root, "service_test.go"), "package status\n")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	ui := &scriptedUI{lines: []string{"/status\n", "/exit\n"}}
	cfg := config.Config{
		Provider:   "ollama",
		Model:      "llama3.2",
		MaxTokens:  128,
		TesseraDir: t.TempDir(),
	}

	orch := New(&fakeLLM{}, store, ui, nil, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	status := ui.eventByType("status")
	if status.Type == "" {
		t.Fatalf("expected status event, events=%#v", ui.events)
	}
	for key, want := range map[string]any{
		"mode":        "existing_project",
		"stack":       "Go",
		"git":         false,
		"tests":       true,
		"test_runner": "go test ./...",
	} {
		if got := status.Data[key]; got != want {
			t.Fatalf("expected status %s=%#v, got %#v in %#v", key, want, got, status.Data)
		}
	}
}

func TestIndexSlashCommandPersistsSymbols(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/index\n")
	writeTestFile(t, filepath.Join(root, "service.go"), "package index\n\nfunc Build() {}\n")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	ui := &scriptedUI{lines: []string{"/index\n", "/exit\n"}}
	cfg := config.Config{
		Provider:   "ollama",
		Model:      "llama3.2",
		MaxTokens:  128,
		TesseraDir: t.TempDir(),
	}

	orch := New(&fakeLLM{}, store, ui, nil, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if !ui.sawEvent("index.finished") {
		t.Fatalf("expected index.finished event, events=%#v", ui.events)
	}
	sess, err := store.GetSession(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := store.ListSymbols(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) == 0 || symbols[0].Name != "Build" || symbols[0].StartLine != 3 {
		t.Fatalf("unexpected symbols: %#v", symbols)
	}
}

func TestRunCommandActionRequiresApprovalAndExecutes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/command\n")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{
		resp: llm.GenerateResponse{
			Text: `{"action":"run_command","message":"Run the focused Go tests.","command":{"name":"go","args":["test","./..."],"reason":"Verify the current project."}}`,
		},
	}
	exec := &fakeExecutor{out: port.Output{Stdout: "ok\n"}}
	ui := &scriptedUI{lines: []string{"rode os testes\n", "/exit\n"}}
	cfg := config.Config{
		Provider:   "ollama",
		Model:      "llama3.2",
		MaxTokens:  128,
		TesseraDir: t.TempDir(),
	}

	orch := New(model, store, ui, exec, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if len(exec.commands) != 1 {
		t.Fatalf("expected one executed command, got %#v", exec.commands)
	}
	got := exec.commands[0]
	if got.Name != "go" || strings.Join(got.Args, " ") != "test ./..." || got.Dir != root {
		t.Fatalf("unexpected command: %#v", got)
	}
	if !ui.sawEvent("approval.requested") {
		t.Fatal("expected approval request before command execution")
	}
	if !ui.sawEvent("test.finished") {
		t.Fatalf("expected command result event, events=%#v", ui.events)
	}
}

type fakeLLM struct {
	requests []llm.GenerateRequest
	resp     llm.GenerateResponse
	err      error
}

func (f *fakeLLM) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	f.requests = append(f.requests, req)
	return f.resp, f.err
}

type fakeExecutor struct {
	commands []port.Command
	out      port.Output
	err      error
}

func (f *fakeExecutor) Run(ctx context.Context, cmd port.Command) (port.Output, error) {
	f.commands = append(f.commands, cmd)
	return f.out, f.err
}

type scriptedUI struct {
	lines  []string
	events []event.Event
}

func (s *scriptedUI) RenderEvent(evt event.Event) {
	s.events = append(s.events, evt)
}

func (s *scriptedUI) AskApproval(evt event.Event) bool {
	s.events = append(s.events, evt)
	return true
}

func (s *scriptedUI) ReadLine(prompt string) (string, error) {
	if len(s.lines) == 0 {
		return "", nil
	}
	line := s.lines[0]
	s.lines = s.lines[1:]
	return line, nil
}

func (s *scriptedUI) sawEvent(eventType string) bool {
	for _, evt := range s.events {
		if evt.Type == eventType {
			return true
		}
	}
	return false
}

func (s *scriptedUI) sawMessage(message string) bool {
	for _, evt := range s.events {
		if strings.Contains(evt.Message, message) {
			return true
		}
	}
	return false
}

func (s *scriptedUI) eventByType(eventType string) event.Event {
	for _, evt := range s.events {
		if evt.Type == eventType {
			return evt
		}
	}
	return event.Event{}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
