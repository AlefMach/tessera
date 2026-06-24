package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/port"
	"github.com/alef-mach/tessera/internal/session"
)

func TestInteractiveInputCallsLLM(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{
		resp: llm.GenerateResponse{
			Text:         `{"type":"finish","summary":"resposta do modelo"}`,
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
		resps: []llm.GenerateResponse{
			{Text: `{"type":"run","reason":"Run the focused Go tests.","command":"go test ./..."}`},
			{Text: `{"type":"finish","summary":"Tests passed. No code changes are needed."}`},
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
	if len(model.requests) != 2 {
		t.Fatalf("expected Tessera to continue after command output, got %d LLM calls", len(model.requests))
	}
	if !strings.Contains(model.requests[1].Prompt, "command.result") || !strings.Contains(model.requests[1].Prompt, "ok") {
		t.Fatalf("expected second prompt to include command result, got:\n%s", model.requests[1].Prompt)
	}
}

func TestProposePatchRequiresApprovalAppliesAndContinues(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/patch\n")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	patch := "diff --git a/hello.txt b/hello.txt\nnew file mode 100644\nindex 0000000..ce01362\n--- /dev/null\n+++ b/hello.txt\n@@ -0,0 +1 @@\n+hello\n"
	model := &fakeLLM{
		resps: []llm.GenerateResponse{
			{Text: `{"type":"patch","reason":"Create hello.txt.","files":["hello.txt"],"patch":` + strconv.Quote(patch) + `}`},
			{Text: `{"type":"finish","summary":"Patch applied."}`},
		},
	}
	exec := &fakeExecutor{out: port.Output{Stdout: ""}}
	ui := &scriptedUI{lines: []string{"crie hello\n", "/exit\n"}}
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

	if len(exec.commands) != 2 {
		t.Fatalf("expected git status and one patch apply command, got %#v", exec.commands)
	}
	if got := exec.commands[0]; got.Name != "git" || strings.Join(got.Args, " ") != "status --short --branch" || got.Dir != root {
		t.Fatalf("unexpected pre-patch git status command: %#v", got)
	}
	got := exec.commands[1]
	if got.Name != "git" || len(got.Args) != 3 || got.Args[0] != "apply" || got.Dir != root {
		t.Fatalf("unexpected patch command: %#v", got)
	}
	if !ui.sawEvent("approval.requested") || !ui.sawEvent("patch.applied") {
		t.Fatalf("expected patch approval and applied events, got %#v", ui.events)
	}
	if len(model.requests) != 2 {
		t.Fatalf("expected Tessera to continue after applying patch, got %d LLM calls", len(model.requests))
	}
}

func TestBlockerActionFailsRun(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{resp: llm.GenerateResponse{Text: `{"type":"blocker","reason":"missing required decision"}`}}
	ui := &scriptedUI{lines: []string{"continue\n", "/exit\n"}}
	cfg := config.Config{Provider: "ollama", Model: "llama3.2", MaxTokens: 128, TesseraDir: t.TempDir()}

	orch := New(model, store, ui, nil, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	runs, err := store.ListRuns(ctx, model.requests[0].SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("expected failed run, got %#v", runs)
	}
	if !ui.sawEvent("run.failed") {
		t.Fatalf("expected run.failed event, events=%#v", ui.events)
	}
}

func TestAgentLoopStopsAtMaxSteps(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/limit\n")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{resp: llm.GenerateResponse{Text: `{"type":"inspect","reason":"Need more context.","files":["go.mod"]}`}}
	ui := &scriptedUI{lines: []string{"loop\n", "/exit\n"}}
	cfg := config.Config{Provider: "ollama", Model: "llama3.2", MaxTokens: 128, TesseraDir: t.TempDir()}

	orch := New(model, store, ui, nil, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if len(model.requests) != maxAgentSteps {
		t.Fatalf("expected %d LLM calls, got %d", maxAgentSteps, len(model.requests))
	}
	runs, err := store.ListRuns(ctx, model.requests[0].SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != "failed" || runs[0].Steps != maxAgentSteps {
		t.Fatalf("expected failed run at max steps, got %#v", runs)
	}
}

func TestInvalidModelActionFailsRun(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{resp: llm.GenerateResponse{Text: `{"type":"run"}`}}
	ui := &scriptedUI{lines: []string{"invalid\n", "/exit\n"}}
	cfg := config.Config{Provider: "ollama", Model: "llama3.2", MaxTokens: 128, TesseraDir: t.TempDir()}

	orch := New(model, store, ui, nil, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	runs, err := store.ListRuns(ctx, model.requests[0].SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("expected invalid action to fail run, got %#v", runs)
	}
	if !ui.sawEvent("run.failed") {
		t.Fatalf("expected run.failed event, events=%#v", ui.events)
	}
}

func TestRunStepsUpdatedAcrossLoop(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	model := &fakeLLM{resps: []llm.GenerateResponse{
		{Text: `{"type":"run","reason":"Verify.","command":"go test ./..."}`},
		{Text: `{"type":"finish","summary":"done"}`},
	}}
	exec := &fakeExecutor{out: port.Output{Stdout: "ok\n"}}
	ui := &scriptedUI{lines: []string{"steps\n", "/exit\n"}}
	cfg := config.Config{Provider: "ollama", Model: "llama3.2", MaxTokens: 128, TesseraDir: t.TempDir()}

	orch := New(model, store, ui, exec, cfg)
	if err := orch.Start(ctx); err != nil {
		t.Fatal(err)
	}

	runs, err := store.ListRuns(ctx, model.requests[0].SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Steps != 2 || runs[0].Status != "finished" {
		t.Fatalf("expected finished run with 2 steps, got %#v", runs)
	}
}

func TestFailRunPersistsFailedStatusAndErrorObservation(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	sess := session.Session{ID: "sess-test", CWD: t.TempDir(), Provider: "ollama", Model: "llama3.2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	ui := &scriptedUI{}
	orch := New(nil, store, ui, nil, config.Config{TesseraDir: t.TempDir()})
	orch.session = sess
	run := &memory.Run{ID: "run-test", SessionID: sess.ID, Input: "x", Status: "running", StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := store.SaveRun(ctx, *run); err != nil {
		t.Fatal(err)
	}

	orch.failRun(ctx, run, errors.New("boom"))

	got, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.EndedAt == nil {
		t.Fatalf("expected failed run with ended_at, got %#v", got)
	}
	observations, err := store.ListObservations(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundError := false
	for _, observation := range observations {
		if observation.Kind == "error" && strings.Contains(observation.Content, "boom") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Fatalf("expected error observation, got %#v", observations)
	}
}

func TestModelActionValidationRejectsIncompleteActions(t *testing.T) {
	tests := []ModelAction{
		{Type: ActionInspect},
		{Type: ActionPatch},
		{Type: ActionRun},
		{Type: ActionFinish},
		{Type: ActionBlocker},
		{Type: ActionType("unknown")},
	}
	for _, tt := range tests {
		if err := tt.Validate(); err == nil {
			t.Fatalf("expected validation error for %#v", tt)
		}
	}
}

type fakeLLM struct {
	requests []llm.GenerateRequest
	resps    []llm.GenerateResponse
	resp     llm.GenerateResponse
	err      error
}

func (f *fakeLLM) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.resps) > 0 {
		resp := f.resps[0]
		f.resps = f.resps[1:]
		return resp, f.err
	}
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
