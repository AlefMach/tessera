package main

import (
	"context"

	"github.com/alef-mach/tessera/internal/adapter/executor/localexec"
	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/llm/ollama"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/orchestrator"
	"github.com/alef-mach/tessera/internal/ui/plain"
)

func runInteractive(ctx context.Context, cfg config.Config) error {
	ui := plain.NewRenderer()

	ok, err := ensureTrustedFolder(ui)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	memory := sqlite.NewMemoryStore(cfg.SQLitePath)

	llm := ollama.NewOllamaLLM(
		cfg.OllamaURL,
		cfg.Model,
		ollama.WithMemoryStore(memory),
	)

	executor := localexec.NewExecutor()

	orch := orchestrator.New(llm, memory, ui, executor, cfg)
	return orch.Start(ctx)
}
func runTask(ctx context.Context, cfg config.Config, task string) error {
	ui := plain.NewRenderer()

	ok, err := ensureTrustedFolder(ui)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	memory := sqlite.NewMemoryStore(cfg.SQLitePath)

	llm := ollama.NewOllamaLLM(
		cfg.OllamaURL,
		cfg.Model,
		ollama.WithMemoryStore(memory),
	)

	executor := localexec.NewExecutor()

	orch := orchestrator.New(llm, memory, ui, executor, cfg)
	return orch.RunTask(ctx, task)
}
