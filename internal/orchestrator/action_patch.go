package orchestrator

import (
	"context"
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
		failureResult := strings.TrimSpace(fmt.Sprintf(
			"patch_status: failed\nhint: unified diff headers must match actual file paths; use --- /dev/null for new files\n\n%s",
			output,
		))
		o.saveObservation(ctx, run, "patch.failed", failureResult, map[string]any{
			"patch_file": patchPath,
		})
		return failureResult, false, nil
	}

	filesChanged := strings.Join(action.Files, ", ")
	if filesChanged == "" {
		filesChanged = "unknown"
	}
	result := strings.TrimSpace(fmt.Sprintf(
		"%s\nfiles_changed: %s\n%s",
		patchStatusApplied,
		filesChanged,
		output,
	))
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

// applyPatch tries git apply first, then falls back to GNU patch for non-git repos.
func (o *Orchestrator) applyPatch(ctx context.Context, patchPath string) (string, error) {
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

	return fmt.Sprintf("git apply failed: %s\npatch fallback failed: %s", gitErr, output2),
		fmt.Errorf("patch application failed")
}

func writePreviewDiff(path, oldContent, newContent string, fileExists bool) string {
	oldPath := "a/" + filepath.ToSlash(path)
	if !fileExists {
		oldPath = "/dev/null"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", oldPath)
	fmt.Fprintf(&b, "+++ b/%s\n", filepath.ToSlash(path))
	b.WriteString("@@\n")
	if fileExists {
		for _, line := range strings.Split(strings.TrimSuffix(oldContent, "\n"), "\n") {
			if line != "" {
				fmt.Fprintf(&b, "-%s\n", line)
			}
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(newContent, "\n"), "\n") {
		if line != "" {
			fmt.Fprintf(&b, "+%s\n", line)
		}
	}
	return b.String()
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
