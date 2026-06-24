package orchestrator

import (
	"strings"

	"github.com/alef-mach/tessera/internal/memory"
)

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func runID(run *memory.Run) string {
	if run == nil {
		return ""
	}
	return run.ID
}

func isSuccessfulRunResult(result string) bool {
	lower := strings.ToLower(result)
	return strings.Contains(lower, "status: ok") &&
		strings.Contains(lower, "exit_code: 0")
}

func isSuccessfulPatchResult(result string) bool {
	return strings.Contains(strings.ToLower(result), "patch_status: applied")
}

func isSuccessfulWriteResult(result string) bool {
	return strings.Contains(strings.ToLower(result), "write_status: applied")
}
