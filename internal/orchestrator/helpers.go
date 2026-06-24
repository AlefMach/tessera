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

func taskRequestsCodeChange(task string) bool {
	words := tokenize(strings.ToLower(task))
	for _, word := range []string{
		"add", "alter", "alterar", "altere", "atualize", "change",
		"corrija", "corrigir", "create", "criar", "crie", "edit", "editar",
		"edite", "fix", "implement", "implementar", "implemente", "melhore",
		"melhorar", "modifique", "modify", "mudar", "mude", "refactor",
		"refactoring", "refatora", "refatorar", "refatore", "reformat",
		"remove", "remova", "remover", "reescreva", "rewrite", "update",
		"write",
	} {
		if words[word] {
			return true
		}
	}
	return false
}

func rejectedPrematureFinishResult(reason string) string {
	return "finish_status: rejected\nreason: " + reason
}

func blockerOnlyMentionsMissingTests(reason string) bool {
	lower := strings.ToLower(reason)
	hasTestWord := strings.Contains(lower, "test") ||
		strings.Contains(lower, "unit") ||
		strings.Contains(lower, "teste")
	hasMissingWord := strings.Contains(lower, "missing") ||
		strings.Contains(lower, "no ") ||
		strings.Contains(lower, "without") ||
		strings.Contains(lower, "aus") ||
		strings.Contains(lower, "sem ")
	return hasTestWord && hasMissingWord
}
