package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/port"
)

func (o *Orchestrator) executeRun(ctx context.Context, run *memory.Run, action ModelAction) (string, bool, error) {
	if o.executor == nil {
		return "", false, errors.New("command unavailable: no command executor is configured")
	}
	commandText := strings.TrimSpace(action.Command)
	name, args := splitCommand(commandText)
	if name == "" {
		return "", false, errors.New("run action provided an empty command")
	}
	if reason := commandRisk(commandText); reason != "" {
		return "", false, fmt.Errorf("command blocked: %s", reason)
	}

	approved := o.ui.AskApproval(event.New("approval.requested", "Approve command?", action.Reason, map[string]any{
		"command": commandText,
		"risk":    "local command execution",
		"run_id":  runID(run),
	}))
	if !approved {
		return "", false, errors.New("command denied by user")
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
	output := commandOutput(out, err)
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
	result := strings.TrimSpace(fmt.Sprintf(
		"command: %s\nstatus: %s\nexit_code: %d\n\n%s",
		commandText,
		status,
		out.ExitCode,
		output,
	))

	return result, false, nil
}

func splitCommand(command string) (string, []string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func commandOutput(out port.Output, err error) string {
	output := strings.TrimSpace(strings.Join([]string{out.Stdout, out.Stderr}, "\n"))
	if output == "" && err != nil {
		output = err.Error()
	}
	if len(output) > 12000 {
		output = output[:12000] + "\n... output truncated"
	}
	return output
}

func commandRisk(command string) string {
	lower := strings.ToLower(strings.Join(strings.Fields(command), " "))
	dangerous := []string{
		"rm -rf",
		"git reset --hard",
		"git checkout .",
		"git clean -fd",
		"git push",
		"curl",
		"wget",
	}
	for _, pattern := range dangerous {
		if strings.Contains(lower, pattern) {
			if (pattern == "curl" || pattern == "wget") && !strings.Contains(lower, "| sh") {
				continue
			}
			return "blocked dangerous command pattern: " + pattern
		}
	}
	if strings.ContainsAny(command, "&;<>`") {
		return "blocked shell metacharacter in command"
	}
	if strings.Contains(command, "|") {
		return "blocked piped command"
	}

	name, args := splitCommand(command)
	switch name {
	case "sudo", "su", "ssh", "scp", "rsync", "docker", "kubectl":
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
