package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/port"
)

const maxInspectFileBytes = 24_000

type ActionType string

const (
	ActionInspect ActionType = "inspect"
	ActionPatch   ActionType = "patch"
	ActionRun     ActionType = "run"
	ActionFinish  ActionType = "finish"
	ActionBlocker ActionType = "blocker"
)

type ModelAction struct {
	Type    ActionType `json:"type"`
	Reason  string     `json:"reason,omitempty"`
	Files   []string   `json:"files,omitempty"`
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

func (o *Orchestrator) executeInspect(ctx context.Context, run *memory.Run, action ModelAction) (string, bool, error) {
	var b strings.Builder
	for _, file := range action.Files {
		rel := filepath.ToSlash(strings.TrimSpace(file))
		if rel == "" {
			continue
		}
		content, err := o.readWorkspaceFile(rel)
		if err != nil {
			fmt.Fprintf(&b, "## %s\nerror: %s\n\n", rel, err)
			continue
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", rel, content)
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return "", false, errors.New("inspect action did not read any files")
	}
	o.emit(ctx, event.New("inspect.finished", "Inspection finished", action.Reason, map[string]any{
		"files":  action.Files,
		"run_id": runID(run),
	}))
	o.saveObservation(ctx, run, "inspect.result", result, map[string]any{
		"files": action.Files,
	})
	return result, false, nil
}

func (o *Orchestrator) readWorkspaceFile(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("path outside workspace is not allowed: %s", rel)
	}
	if isSensitiveWorkspacePath(clean) {
		return "", fmt.Errorf("sensitive file is not inspectable by default: %s", rel)
	}
	path := filepath.Join(o.session.CWD, clean)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	if len(content) > maxInspectFileBytes {
		content = content[:maxInspectFileBytes] + "\n... file truncated ..."
	}
	return content, nil
}

func isSensitiveWorkspacePath(path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return false
	}

	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))

	if lower == ".git" || strings.HasPrefix(lower, ".git/") {
		return true
	}
	if lower == ".tessera" || strings.HasPrefix(lower, ".tessera/") {
		return true
	}
	if strings.HasPrefix(base, ".env") {
		return true
	}

	sensitiveNames := map[string]bool{
		"id_rsa":               true,
		"id_dsa":               true,
		"id_ecdsa":             true,
		"id_ed25519":           true,
		"credentials":          true,
		"credentials.json":     true,
		"service-account.json": true,
	}
	if sensitiveNames[base] {
		return true
	}

	return strings.HasSuffix(base, ".pem") ||
		strings.HasSuffix(base, ".key") ||
		strings.HasSuffix(base, ".p12") ||
		strings.HasSuffix(base, ".pfx")
}

func (o *Orchestrator) executePatch(ctx context.Context, run *memory.Run, action ModelAction) (string, bool, error) {
	gitStatus := ""
	if o.executor != nil {
		gitStatus = o.gitStatus(ctx)
	}
	message := action.Reason
	if isDirtyGitStatus(gitStatus) {
		message = firstNonEmpty(message, "Review this patch before applying it.") + "\n\nWarning: the working tree already has changes. Review the diff carefully so user changes are not overwritten."
	}

	o.emit(ctx, event.New("patch.proposed", "Patch proposed", message, map[string]any{
		"files":      action.Files,
		"patch":      action.Patch,
		"git_status": gitStatus,
		"run_id":     runID(run),
	}))
	o.saveObservation(ctx, run, "patch.proposed", action.Reason, map[string]any{
		"files":      action.Files,
		"patch":      action.Patch,
		"git_status": gitStatus,
	})
	if o.executor == nil {
		return "", false, errors.New("patch unavailable: no command executor is configured")
	}

	approved := o.ui.AskApproval(event.New("approval.requested", "Apply patch?", message, map[string]any{
		"files":      action.Files,
		"patch":      action.Patch,
		"git_status": gitStatus,
		"risk":       "workspace file changes",
		"run_id":     runID(run),
	}))
	if !approved {
		return "patch denied by user", false, errors.New("patch denied by user")
	}

	patchPath, err := o.writePatchFile(run, action.Patch)
	if err != nil {
		return "", false, fmt.Errorf("save patch: %w", err)
	}

	output, applyErr := o.applyPatch(ctx, patchPath)
	if applyErr != nil {
		o.emit(ctx, event.New("run.aborted", "Patch failed", output, map[string]any{
			"patch_file": patchPath,
			"run_id":     runID(run),
		}))
		o.saveObservation(ctx, run, "patch.failed", output, map[string]any{
			"patch_file": patchPath,
			"hint":       "Check that the unified diff header paths match the actual file paths in the project.",
		})
		return output, false, nil
	}

	result := firstNonEmpty(output, "patch applied")
	o.emit(ctx, event.New("patch.applied", "Patch applied", strings.TrimSpace(action.Reason), map[string]any{
		"files":      action.Files,
		"patch_file": patchPath,
		"run_id":     runID(run),
	}))
	o.saveObservation(ctx, run, "patch.applied", result, map[string]any{
		"files":      action.Files,
		"patch_file": patchPath,
	})
	return result, false, nil
}

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

func (o *Orchestrator) writePatchFile(run *memory.Run, patch string) (string, error) {
	dir := filepath.Join(o.config.TesseraDir, "runs", firstNonEmpty(runID(run), "ad-hoc"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := "latest.patch"
	if runID(run) != "" {
		filename = time.Now().UTC().Format("20060102-150405.000000000") + ".patch"
	}
	path := filepath.Join(dir, filename)
	if !strings.HasSuffix(patch, "\n") {
		patch += "\n"
	}
	if err := os.WriteFile(path, []byte(patch), 0o644); err != nil {
		return "", err
	}
	return path, nil
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

// applyPatch tries to apply a patch file using git apply first, then falls back to GNU patch.
// This allows Tessera to work in projects that are not git repositories.
func (o *Orchestrator) applyPatch(ctx context.Context, patchPath string) (string, error) {
	// Try git apply first (git repos and also works without --git on many versions)
	out, err := o.executor.Run(ctx, port.Command{
		Name:    "git",
		Args:    []string{"apply", "--whitespace=nowarn", patchPath},
		Dir:     o.session.CWD,
		Timeout: 30 * time.Second,
	})
	output := commandOutput(out, err)
	if err == nil && out.ExitCode == 0 {
		return output, nil
	}
	gitErr := output

	// Fall back to GNU patch for non-git repos or when git apply fails
	out2, err2 := o.executor.Run(ctx, port.Command{
		Name:    "patch",
		Args:    []string{"-p1", "--forward", "--batch", "-i", patchPath},
		Dir:     o.session.CWD,
		Timeout: 30 * time.Second,
	})
	output2 := commandOutput(out2, err2)
	if err2 == nil && out2.ExitCode == 0 {
		return output2, nil
	}

	// Both failed — return combined diagnostics so the model can fix the patch
	return fmt.Sprintf("git apply failed: %s\npatch fallback failed: %s", gitErr, output2),
		fmt.Errorf("patch application failed")
}

func isDirtyGitStatus(status string) bool {
	status = strings.TrimSpace(status)
	if status == "" || status == "clean" || strings.HasPrefix(status, "unavailable:") {
		return false
	}
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "##") {
			continue
		}
		return true
	}
	return false
}
