package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory"
)

const maxAgentSteps = 400
const maxActionParseAttempts = 2

func (o *Orchestrator) runAgentLoop(ctx context.Context, run *memory.Run, input string) error {
	if o.llm == nil {
		return fmt.Errorf("LLM unavailable: no LLM provider is configured")
	}

	previousResult := ""
	successfulRunActions := 0
	patchesAppliedSinceLastRun := false

	for step := 1; step <= maxAgentSteps; step++ {
		run.Steps = step
		run.UpdatedAt = time.Now().UTC()
		if err := o.memory.SaveRun(ctx, *run); err != nil {
			return fmt.Errorf("save run step %d: %w", step, err)
		}

		o.emit(ctx, event.New("agent.step.started", "Agent step started", "", map[string]any{
			"run_id": run.ID,
			"step":   step,
		}))

		action, err := o.requestModelAction(ctx, run, input, previousResult)
		if err != nil {
			return err
		}

		result, done, err := o.executeModelAction(ctx, run, action)
		if err != nil {
			return err
		}

		o.emit(ctx, event.New("agent.step.finished", "Agent step finished", oneLine(result), map[string]any{
			"run_id": run.ID,
			"step":   step,
			"action": string(action.Type),
		}))

		if done {
			return nil
		}

		switch action.Type {
		case ActionPatch:
			if isSuccessfulPatchResult(result) {
				patchesAppliedSinceLastRun = true
				successfulRunActions = 0
			}

		case ActionRun:
			if isSuccessfulRunResult(result) {
				successfulRunActions++

				if successfulRunActions >= 2 && !patchesAppliedSinceLastRun {
					summary := "Verification commands passed. Stopping to avoid repeating tests without new changes."
					o.saveObservation(ctx, run, "finish.auto", summary, map[string]any{
						"successful_run_actions": successfulRunActions,
					})
					o.emit(ctx, event.New("run.completed", "Run completed", summary, map[string]any{
						"run_id": run.ID,
					}))
					return nil
				}

				patchesAppliedSinceLastRun = false
			} else {
				successfulRunActions = 0
			}
		}

		previousResult = result
	}
	return fmt.Errorf("agent stopped after reaching the maximum of %d steps", maxAgentSteps)
}

func (o *Orchestrator) requestModelAction(ctx context.Context, run *memory.Run, task string, previousResult string) (ModelAction, error) {
	runID := runID(run)

	var lastInvalid string
	var lastErr error

	for attempt := 1; attempt <= maxActionParseAttempts; attempt++ {
		if run != nil {
			run.Calls++
			run.UpdatedAt = time.Now().UTC()
			if err := o.memory.SaveRun(ctx, *run); err != nil {
				o.emit(ctx, event.New("error.occurred", "Run not saved", err.Error(), map[string]any{
					"error":  err.Error(),
					"run_id": runID,
				}))
			}
		}

		o.emit(ctx, event.New("llm.call.started", "LLM call started", "", map[string]any{
			"provider": o.config.Provider,
			"model":    o.config.Model,
			"run_id":   runID,
			"attempt":  attempt,
		}))

		prompt := o.buildPrompt(ctx, task, previousResult)
		if attempt > 1 {
			prompt = appendInvalidActionRepairPrompt(prompt, lastInvalid, lastErr)
		}

		resp, err := o.llm.Generate(ctx, llm.GenerateRequest{
			Prompt:    prompt,
			System:    tesseraSystemPrompt,
			MaxTokens: o.config.MaxTokens,
			SessionID: o.session.ID,
			RunID:     runID,
		})
		if err != nil {
			o.emit(ctx, event.New("error.occurred", "LLM call failed", err.Error(), map[string]any{
				"error":   err.Error(),
				"run_id":  runID,
				"attempt": attempt,
			}))
			return ModelAction{}, err
		}

		o.emit(ctx, event.New("llm.call.finished", "LLM response", strings.TrimSpace(resp.Text), map[string]any{
			"provider":      o.config.Provider,
			"model":         firstNonEmpty(resp.Model, o.config.Model),
			"input_tokens":  resp.InputTokens,
			"output_tokens": resp.OutputTokens,
			"duration":      resp.Duration.Truncate(time.Millisecond).String(),
			"run_id":        runID,
			"attempt":       attempt,
		}))

		action, err := parseModelAction(resp.Text)
		if err == nil {
			o.saveObservation(ctx, run, "model."+string(action.Type), firstNonEmpty(action.Reason, action.Summary), map[string]any{
				"action":  string(action.Type),
				"files":   action.Files,
				"attempt": attempt,
			})
			return action, nil
		}

		lastInvalid = strings.TrimSpace(resp.Text)
		lastErr = err
		o.saveObservation(ctx, run, "model.invalid_action", lastInvalid, map[string]any{
			"error":   err.Error(),
			"attempt": attempt,
		})
		o.emit(ctx, event.New("model.invalid_action", "Invalid model action", err.Error(), map[string]any{
			"run_id":  runID,
			"attempt": attempt,
		}))
	}

	return ModelAction{}, fmt.Errorf("invalid model action after %d attempts: %w", maxActionParseAttempts, lastErr)
}
