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

const maxInspectFileBytes = 24_000

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
	clean, err := cleanWorkspaceRelPath(rel)
	if err != nil {
		return "", err
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
