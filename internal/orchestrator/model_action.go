package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/port"
)

type modelAction struct {
	Action  string        `json:"action"`
	Message string        `json:"message"`
	Command commandAction `json:"command"`
	Patch   string        `json:"patch"`
	Files   []string      `json:"files"`
}

type commandAction struct {
	Name   string   `json:"name"`
	Args   []string `json:"args"`
	Reason string   `json:"reason"`
}

func (o *Orchestrator) handleModelResponse(ctx context.Context, run *memory.Run, text string) {
	action, ok := parseModelAction(text)
	if !ok {
		o.saveObservation(ctx, run, "model.answer", strings.TrimSpace(text), nil)
		return
	}
	if action.Message != "" {
		o.saveObservation(ctx, run, "model."+firstNonEmpty(action.Action, "answer"), action.Message, nil)
	}

	switch action.Action {
	case "", "answer":
		return
	case "run_command":
		o.handleRunCommand(ctx, run, action)
	case "propose_patch":
		o.handleProposedPatch(ctx, run, action)
	case "suggest_commit_message":
		o.emit(ctx, event.New("commit.message.suggested", "Suggested commit message", action.Message, map[string]any{
			"run_id": runID(run),
		}))
	default:
		o.emit(ctx, event.New("run.aborted", "Unsupported model action", action.Message, map[string]any{
			"action": action.Action,
			"run_id": runID(run),
		}))
	}
}

func (o *Orchestrator) handleRunCommand(ctx context.Context, run *memory.Run, action modelAction) {
	if o.executor == nil {
		o.emit(ctx, event.New("error.occurred", "Command unavailable", "No command executor is configured.", map[string]any{"run_id": runID(run)}))
		return
	}
	name := strings.TrimSpace(action.Command.Name)
	args := cleanArgs(action.Command.Args)
	if name == "" {
		o.emit(ctx, event.New("run.aborted", "Command rejected", "The model did not provide a command name.", map[string]any{"run_id": runID(run)}))
		return
	}
	if reason := commandRisk(name, args); reason != "" {
		o.emit(ctx, event.New("run.aborted", "Command blocked", reason, map[string]any{
			"command": commandString(name, args),
			"run_id":  runID(run),
		}))
		return
	}

	commandText := commandString(name, args)
	approved := o.ui.AskApproval(event.New("approval.requested", "Approve command?", action.Command.Reason, map[string]any{
		"command": commandText,
		"risk":    "local command execution",
		"run_id":  runID(run),
	}))
	if !approved {
		o.emit(ctx, event.New("run.aborted", "Command denied", commandText, map[string]any{"run_id": runID(run)}))
		return
	}

	started := time.Now()
	o.emit(ctx, event.New("command.started", "Command started", "", map[string]any{
		"command": commandText,
		"run_id":  runID(run),
	}))
	out, err := o.executor.Run(ctx, port.Command{
		Name:    name,
		Args:    args,
		Dir:     o.session.CWD,
		Timeout: 2 * time.Minute,
	})
	output := strings.TrimSpace(strings.Join([]string{out.Stdout, out.Stderr}, "\n"))
	if len(output) > 12000 {
		output = output[:12000] + "\n... output truncated"
	}
	status := "ok"
	if err != nil || out.ExitCode != 0 {
		status = "failed"
	}
	o.emit(ctx, event.New("test.finished", "Command finished", output, map[string]any{
		"command":   commandText,
		"status":    status,
		"exit_code": out.ExitCode,
		"duration":  time.Since(started).Truncate(time.Millisecond).String(),
		"run_id":    runID(run),
	}))
	o.saveObservation(ctx, run, "command.result", output, map[string]any{
		"command":   commandText,
		"status":    status,
		"exit_code": out.ExitCode,
	})
}

func (o *Orchestrator) handleProposedPatch(ctx context.Context, run *memory.Run, action modelAction) {
	o.emit(ctx, event.New("patch.proposed", "Patch proposed", action.Message, map[string]any{
		"files":  action.Files,
		"patch":  action.Patch,
		"run_id": runID(run),
	}))
	o.saveObservation(ctx, run, "patch.proposed", action.Message, map[string]any{
		"files": action.Files,
		"patch": action.Patch,
	})
}

func parseModelAction(text string) (modelAction, bool) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return modelAction{}, false
	}
	raw = extractJSON(raw)
	var action modelAction
	if err := json.Unmarshal([]byte(raw), &action); err != nil {
		return modelAction{}, false
	}
	action.Action = strings.TrimSpace(strings.ToLower(action.Action))
	action.Message = strings.TrimSpace(action.Message)
	action.Patch = strings.TrimSpace(action.Patch)
	action.Command.Name = strings.TrimSpace(action.Command.Name)
	action.Command.Reason = strings.TrimSpace(action.Command.Reason)
	action.Command.Args = cleanArgs(action.Command.Args)
	return action, true
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

func commandString(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func commandRisk(name string, args []string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "empty command"
	}
	if strings.ContainsAny(name, "&|;<>`$") {
		return "blocked shell metacharacter in command name"
	}
	for _, arg := range args {
		if strings.ContainsAny(arg, "&|;<>`") {
			return "blocked shell metacharacter in command arguments"
		}
	}

	switch name {
	case "rm", "rmdir", "mv", "cp", "chmod", "chown", "sudo", "su", "curl", "wget", "ssh", "scp", "rsync", "docker", "kubectl":
		return "blocked potentially destructive or networked command"
	case "git":
		if len(args) == 0 {
			return ""
		}
		switch args[0] {
		case "status", "diff", "log", "show", "branch":
			return ""
		default:
			return "blocked git command that may mutate repository state"
		}
	}
	return ""
}
