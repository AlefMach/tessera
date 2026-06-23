package orchestrator

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/port"
	"github.com/alef-mach/tessera/internal/session"
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
			if run == nil {
				continue
			}
			if err := o.runAgentLoop(ctx, run, input); err != nil {
				o.failRun(ctx, run, err)
				continue
			}
			o.finishRun(ctx, run)
		}
	}
}

func (o *Orchestrator) startRun(ctx context.Context, input string) *memory.Run {
	now := time.Now().UTC()
	run := &memory.Run{
		ID:        "run-" + now.Format("20060102-150405.000000000"),
		SessionID: o.session.ID,
		Input:     input,
		Status:    "running",
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

func (o *Orchestrator) saveObservation(ctx context.Context, run *memory.Run, kind, content string, data map[string]any) {
	if strings.TrimSpace(content) == "" {
		return
	}
	now := time.Now().UTC()
	observation := memory.Observation{
		ID:        "obs-" + now.Format("20060102-150405.000000000"),
		SessionID: o.session.ID,
		RunID:     runID(run),
		Kind:      kind,
		Content:   content,
		Data:      data,
		CreatedAt: now,
	}
	if err := o.memory.SaveObservation(ctx, observation); err != nil {
		o.emit(ctx, event.New("error.occurred", "Observation not saved", err.Error(), map[string]any{"error": err.Error()}))
	}
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

func (o *Orchestrator) failRun(ctx context.Context, run *memory.Run, err error) {
	if run == nil {
		return
	}
	message := "run failed"
	if err != nil {
		message = err.Error()
	}
	now := time.Now().UTC()
	run.Status = "failed"
	run.UpdatedAt = now
	run.EndedAt = &now
	o.saveObservation(ctx, run, "error", message, map[string]any{
		"error": message,
	})
	if saveErr := o.memory.SaveRun(ctx, *run); saveErr != nil {
		o.emit(ctx, event.New("error.occurred", "Run not saved", saveErr.Error(), map[string]any{"error": saveErr.Error(), "run_id": run.ID}))
	}
	o.emit(ctx, event.New("run.failed", "Run failed", message, map[string]any{
		"run_id": run.ID,
		"error":  message,
	}))
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
