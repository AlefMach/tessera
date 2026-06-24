package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
)

type ActionType string

const (
	ActionInspect ActionType = "inspect"
	ActionPatch   ActionType = "patch"
	ActionWrite   ActionType = "write"
	ActionRun     ActionType = "run"
	ActionFinish  ActionType = "finish"
	ActionBlocker ActionType = "blocker"
)

type ModelAction struct {
	Type    ActionType `json:"type"`
	Reason  string     `json:"reason,omitempty"`
	Files   []string   `json:"files,omitempty"`
	Path    string     `json:"path,omitempty"`
	Content string     `json:"content,omitempty"`
	Patch   string     `json:"patch,omitempty"`
	Command string     `json:"command,omitempty"`
	Summary string     `json:"summary,omitempty"`
}

func (a ModelAction) Validate() error {
	switch a.Type {
	case ActionInspect:
		if len(a.Files) == 0 {
			return errors.New("inspect action requires at least one file")
		}
	case ActionPatch:
		if strings.TrimSpace(a.Patch) == "" {
			return errors.New("patch action requires a non-empty patch")
		}
	case ActionWrite:
		if strings.TrimSpace(a.Path) == "" && len(a.Files) != 1 {
			return errors.New("write action requires path or exactly one file")
		}
		if a.Content == "" {
			return errors.New("write action requires non-empty content")
		}
	case ActionRun:
		if strings.TrimSpace(a.Command) == "" {
			return errors.New("run action requires a non-empty command")
		}
	case ActionFinish:
		if strings.TrimSpace(a.Summary) == "" {
			return errors.New("finish action requires a non-empty summary")
		}
	case ActionBlocker:
		if strings.TrimSpace(a.Reason) == "" {
			return errors.New("blocker action requires a non-empty reason")
		}
	default:
		return fmt.Errorf("unknown action type %q", a.Type)
	}
	return nil
}

func (o *Orchestrator) executeModelAction(ctx context.Context, run *memory.Run, action ModelAction) (string, bool, error) {
	if err := action.Validate(); err != nil {
		return "", false, err
	}

	switch action.Type {
	case ActionInspect:
		return o.executeInspect(ctx, run, action)
	case ActionPatch:
		return o.executePatch(ctx, run, action)
	case ActionWrite:
		return o.executeWrite(ctx, run, action)
	case ActionRun:
		return o.executeRun(ctx, run, action)
	case ActionFinish:
		o.emit(ctx, event.New("run.completed", "Run completed", action.Summary, map[string]any{
			"run_id": runID(run),
		}))
		o.saveObservation(ctx, run, "finish", action.Summary, nil)
		return action.Summary, true, nil
	case ActionBlocker:
		return "", false, fmt.Errorf("model reported blocker: %s", action.Reason)
	default:
		return "", false, fmt.Errorf("unknown action type %q", action.Type)
	}
}

func parseModelAction(text string) (ModelAction, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return ModelAction{}, errors.New("empty model response")
	}
	raw = extractJSON(raw)
	var action ModelAction
	if err := json.Unmarshal([]byte(raw), &action); err != nil {
		return ModelAction{}, err
	}
	action.Type = ActionType(strings.TrimSpace(strings.ToLower(string(action.Type))))
	action.Reason = strings.TrimSpace(action.Reason)
	action.Path = strings.TrimSpace(action.Path)
	action.Patch = strings.TrimSpace(action.Patch)
	action.Command = strings.TrimSpace(action.Command)
	action.Summary = strings.TrimSpace(action.Summary)
	action.Files = cleanArgs(action.Files)
	if err := action.Validate(); err != nil {
		return ModelAction{}, err
	}
	return action, nil
}

func extractJSON(text string) string {
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			lines = lines[1:]
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			text = strings.Join(lines, "\n")
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func cleanArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg != "" {
			out = append(out, arg)
		}
	}
	return out
}
