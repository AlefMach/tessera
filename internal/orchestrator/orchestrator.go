package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/port"
	"github.com/alef-mach/tessera/internal/project"
	"github.com/alef-mach/tessera/internal/session"
	"github.com/alef-mach/tessera/internal/treesitter"
)

type Orchestrator struct {
	llm       port.LLM
	memory    port.MemoryStore
	ui        port.UIRenderer
	executor  port.ToolExecutor
	config    config.Config
	session   session.Session
	manager   *session.Manager
	activeRun *memory.Run
}

func New(llm port.LLM, memory port.MemoryStore, ui port.UIRenderer, executor port.ToolExecutor, cfg config.Config) *Orchestrator {
	return &Orchestrator{llm: llm, memory: memory, ui: ui, executor: executor, config: cfg}
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.memory.Ensure(ctx); err != nil {
		return err
	}
	o.manager = session.NewManager(o.memory)
	sess, err := o.manager.Start(ctx, o.config.Provider, o.config.Model)
	if err != nil {
		return err
	}
	o.session = sess

	o.emit(ctx, event.New("session.started", "Session started", "Type your task or /help.", map[string]any{
		"session_id":     o.session.ID,
		"cwd":            o.session.CWD,
		"provider":       o.config.Provider,
		"model":          o.config.Model,
		"context_tokens": o.config.ContextTokens,
		"max_tokens":     o.config.MaxTokens,
		"calls":          0,
	}))
	o.profileProject(ctx)

	return o.interactive(ctx)
}

func (o *Orchestrator) interactive(ctx context.Context) error {
	historyPath := filepath.Join(o.config.TesseraDir, "history")

	for {
		line, err := o.ui.ReadLine("› ")
		if err != nil {
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
				return nil
			}
			if !errors.Is(err, io.EOF) {
				return err
			}
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		appendHistory(historyPath, input)

		switch input {
		case "/exit", "/quit":
			o.emit(ctx, event.New("session.ended", "Session ended", "", nil))
			return nil
		case "/help":
			o.renderHelp(ctx)
		case "/status":
			o.renderStatus(ctx)
		case "/memory":
			o.renderMemory(ctx)
		case "/index":
			o.renderIndex(ctx)
		default:
			run := o.startRun(ctx, input)
			o.executeLLM(ctx, run, input)
			o.finishRun(ctx, run)
		}
	}
}

func (o *Orchestrator) executeLLM(ctx context.Context, run *memory.Run, input string) {
	if o.llm == nil {
		o.emit(ctx, event.New("error.occurred", "LLM unavailable", "No LLM provider is configured.", map[string]any{
			"error": "llm provider is nil",
		}))
		return
	}

	runID := ""
	if run != nil {
		runID = run.ID
		run.Calls++
	}

	o.emit(ctx, event.New("llm.call.started", "LLM call started", "", map[string]any{
		"provider": o.config.Provider,
		"model":    o.config.Model,
		"run_id":   runID,
	}))

	resp, err := o.llm.Generate(ctx, llm.GenerateRequest{
		Prompt:    input,
		MaxTokens: o.config.MaxTokens,
		SessionID: o.session.ID,
		RunID:     runID,
	})
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "LLM call failed", err.Error(), map[string]any{
			"error":  err.Error(),
			"run_id": runID,
		}))
		return
	}

	o.emit(ctx, event.New("llm.call.finished", "LLM response", strings.TrimSpace(resp.Text), map[string]any{
		"provider":      o.config.Provider,
		"model":         firstNonEmpty(resp.Model, o.config.Model),
		"input_tokens":  resp.InputTokens,
		"output_tokens": resp.OutputTokens,
		"duration":      resp.Duration.Truncate(time.Millisecond).String(),
		"run_id":        runID,
	}))
}

func (o *Orchestrator) renderHelp(ctx context.Context) {
	o.emit(ctx, event.New("help", "Commands", "/help    Show available commands\n/status  Show active session status\n/memory  Show saved memory\n/index   Index project files\n/exit    End the session", nil))
}

func (o *Orchestrator) renderStatus(ctx context.Context) {
	stats, err := o.memory.Stats(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Status unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	profile, err := o.memory.GetProjectProfile(ctx, o.session.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			profile = o.profileProject(ctx)
		} else {
			o.emit(ctx, event.New("error.occurred", "Profile unavailable", err.Error(), map[string]any{"error": err.Error()}))
			return
		}
	}
	o.emit(ctx, event.New("status", "Status", "", map[string]any{
		"session":      stats.SessionID,
		"provider":     stats.Provider,
		"model":        stats.Model,
		"mode":         profile.Mode,
		"stack":        profile.Stack,
		"git":          profile.HasGit,
		"tests":        profile.HasTests,
		"test_runner":  profile.TestRunner,
		"calls":        stats.Calls,
		"steps":        stats.Steps,
		"runs":         stats.Runs,
		"observations": stats.Observations,
	}))
}

func (o *Orchestrator) profileProject(ctx context.Context) project.ProjectProfile {
	profile, err := project.Profile(o.session.CWD)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Project profile unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return project.ProjectProfile{}
	}
	profile.SessionID = o.session.ID
	if err := o.memory.SaveProjectProfile(ctx, profile); err != nil {
		o.emit(ctx, event.New("error.occurred", "Project profile not saved", err.Error(), map[string]any{"error": err.Error()}))
		return profile
	}
	o.emit(ctx, event.New("project.profiled", "Project profiled", "", map[string]any{
		"root":        profile.Root,
		"mode":        profile.Mode,
		"stack":       profile.Stack,
		"manifests":   profile.Manifests,
		"git":         profile.HasGit,
		"tests":       profile.HasTests,
		"test_runner": profile.TestRunner,
	}))
	return profile
}

func (o *Orchestrator) renderMemory(ctx context.Context) {
	observations, err := o.memory.ListObservations(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Memory unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	var builder strings.Builder
	limit := min(len(observations), 12)
	for i := 0; i < limit; i++ {
		observation := observations[i]
		builder.WriteString("- ")
		builder.WriteString(observation.CreatedAt.Local().Format("15:04:05"))
		builder.WriteString(" ")
		builder.WriteString(observation.Kind)
		builder.WriteString(" ")
		builder.WriteString(oneLine(observation.Content))
		builder.WriteString("\n")
	}
	if limit == 0 {
		builder.WriteString("No saved observations yet.")
	}
	o.emit(ctx, event.New("memory", "Memory", strings.TrimRight(builder.String(), "\n"), map[string]any{
		"session":      o.session.ID,
		"observations": len(observations),
	}))
}

func (o *Orchestrator) renderIndex(ctx context.Context) {
	o.emit(ctx, event.New("index.started", "Indexing with Tree-sitter", "", map[string]any{
		"root": o.session.CWD,
	}))
	result, err := treesitter.New(o.session.CWD, o.memory).Index(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Index failed", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	message := result.RepoMap
	if message == "" {
		message = "No indexable source files found."
	}
	o.emit(ctx, event.New("index.finished", "Index finished", message, map[string]any{
		"files":       result.Files,
		"symbols":     result.Symbols,
		"started_at":  result.StartedAt,
		"finished_at": result.FinishedAt,
	}))
}

func (o *Orchestrator) startRun(ctx context.Context, input string) *memory.Run {
	now := time.Now().UTC()
	run := &memory.Run{
		ID:        "run-" + now.Format("20060102-150405.000000000"),
		SessionID: o.session.ID,
		Input:     input,
		Status:    "running",
		Steps:     1,
		StartedAt: now,
		UpdatedAt: now,
	}
	if err := o.memory.SaveRun(ctx, *run); err != nil {
		o.emit(ctx, event.New("error.occurred", "Run not saved", err.Error(), map[string]any{"error": err.Error()}))
		return nil
	}
	o.activeRun = run
	return run
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (o *Orchestrator) finishRun(ctx context.Context, run *memory.Run) {
	if run == nil {
		return
	}
	now := time.Now().UTC()
	run.Status = "finished"
	run.UpdatedAt = now
	run.EndedAt = &now
	if err := o.memory.SaveRun(ctx, *run); err != nil {
		o.emit(ctx, event.New("error.occurred", "Run not saved", err.Error(), map[string]any{"error": err.Error()}))
	}
	o.activeRun = nil
}

func (o *Orchestrator) emit(ctx context.Context, evt event.Event) {
	o.ui.RenderEvent(evt)
	if o.session.ID != "" {
		_ = o.memory.SaveEvent(ctx, o.session.ID, evt)
	}
}

func appendHistory(path, input string) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(input + "\n")
}
