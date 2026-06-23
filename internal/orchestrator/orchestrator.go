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
	"github.com/alef-mach/tessera/internal/port"
	"github.com/alef-mach/tessera/internal/session"
)

type Orchestrator struct {
	llm      port.LLM
	memory   port.MemoryStore
	ui       port.UIRenderer
	executor port.ToolExecutor
	config   config.Config
	session  session.Session
}

func New(llm port.LLM, memory port.MemoryStore, ui port.UIRenderer, executor port.ToolExecutor, cfg config.Config) *Orchestrator {
	return &Orchestrator{llm: llm, memory: memory, ui: ui, executor: executor, config: cfg}
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.memory.Ensure(ctx); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	o.session = session.New(makeSessionID(), cwd, o.config.Provider, o.config.Model)
	if err := o.memory.SaveSession(ctx, o.session); err != nil {
		return err
	}

	o.emit(ctx, event.New("session.started", "Session started", "Type your task or /help.", map[string]any{
		"session_id":     o.session.ID,
		"cwd":            cwd,
		"provider":       o.config.Provider,
		"model":          o.config.Model,
		"context_tokens": o.config.ContextTokens,
		"max_tokens":     o.config.MaxTokens,
		"calls":          0,
	}))

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
		default:
			o.emit(ctx, event.New("task.received", "Task received", "LLM execution is not implemented in Milestone 0.", map[string]any{
				"input": input,
			}))
		}
	}
}

func (o *Orchestrator) renderHelp(ctx context.Context) {
	o.emit(ctx, event.New("help", "Commands", "/help  Show available commands\n/exit  End the session", nil))
}

func (o *Orchestrator) emit(ctx context.Context, evt event.Event) {
	o.ui.RenderEvent(evt)
	if o.session.ID != "" {
		_ = o.memory.SaveEvent(ctx, o.session.ID, evt)
	}
}

func makeSessionID() string {
	return "sess-" + time.Now().UTC().Format("20060102-150405")
}

func appendHistory(path, input string) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(input + "\n")
}
