package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
)

func (o *Orchestrator) executeWrite(ctx context.Context, run *memory.Run, action ModelAction) (string, bool, error) {
	rel := strings.TrimSpace(action.Path)
	if rel == "" && len(action.Files) == 1 {
		rel = action.Files[0]
	}
	clean, err := cleanWorkspaceRelPath(rel)
	if err != nil {
		return "", false, err
	}
	if isSensitiveWorkspacePath(clean) {
		return "", false, fmt.Errorf("sensitive file is not writable by default: %s", rel)
	}

	existing, readErr := o.readWorkspaceFile(clean)
	fileExists := readErr == nil
	diff := writePreviewDiff(clean, existing, action.Content, fileExists)
	message := action.Reason
	gitStatus := ""
	if o.executor != nil {
		gitStatus = o.gitStatus(ctx)
	}
	if isDirtyGitStatus(gitStatus) {
		message = firstNonEmpty(message, "Review this write before applying it.") + "\n\nWarning: the working tree already has changes. Review carefully so user changes are not overwritten."
	}

	o.emit(ctx, event.New("write.proposed", "Write proposed", message, map[string]any{
		"files":      []string{filepath.ToSlash(clean)},
		"diff":       diff,
		"git_status": gitStatus,
		"run_id":     runID(run),
	}))
	o.saveObservation(ctx, run, "write.proposed", action.Reason, map[string]any{
		"files":      []string{filepath.ToSlash(clean)},
		"diff":       diff,
		"git_status": gitStatus,
	})

	approved := o.ui.AskApproval(event.New("approval.requested", "Write file?", message, map[string]any{
		"files":      []string{filepath.ToSlash(clean)},
		"diff":       diff,
		"git_status": gitStatus,
		"risk":       "workspace file changes",
		"run_id":     runID(run),
	}))
	if !approved {
		return "write denied by user", false, errors.New("write denied by user")
	}

	path := filepath.Join(o.session.CWD, clean)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, fmt.Errorf("create parent directory: %w", err)
	}
	content := action.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", false, fmt.Errorf("write file: %w", err)
	}

	result := fmt.Sprintf("%s\nfile_changed: %s", writeStatusApplied, filepath.ToSlash(clean))
	o.emit(ctx, event.New("write.applied", "Write applied", strings.TrimSpace(action.Reason), map[string]any{
		"files":  []string{filepath.ToSlash(clean)},
		"run_id": runID(run),
	}))
	o.saveObservation(ctx, run, "write.applied", result, map[string]any{
		"files": []string{filepath.ToSlash(clean)},
	})
	return result, false, nil
}
