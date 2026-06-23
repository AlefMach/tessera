package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory"
)

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

	prompt := o.buildPrompt(ctx, input)
	resp, err := o.llm.Generate(ctx, llm.GenerateRequest{
		Prompt:    prompt,
		System:    tesseraSystemPrompt,
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
	o.handleModelResponse(ctx, run, resp.Text)
}
