package orchestrator

import (
	"strings"

	"github.com/alef-mach/tessera/internal/memory"
)

const (
	patchStatusApplied = "patch_status: applied"
	writeStatusApplied = "write_status: applied"
	runStatusOK        = "status: ok"
	runExitCodeZero    = "exit_code: 0"
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
	return strings.Contains(lower, runStatusOK) &&
		strings.Contains(lower, runExitCodeZero)
}

func isSuccessfulPatchResult(result string) bool {
	return strings.Contains(strings.ToLower(result), patchStatusApplied)
}

func isSuccessfulWriteResult(result string) bool {
	return strings.Contains(strings.ToLower(result), writeStatusApplied)
}

func rejectedPrematureFinishResult(reason string) string {
	return "finish_status: rejected\nreason: " + reason
}
