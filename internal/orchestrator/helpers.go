package orchestrator

import (
	"regexp"
	"strings"

	"github.com/alef-mach/tessera/internal/memory"
)

var filePathPattern = regexp.MustCompile(`(?i)(^|[\s"'(:])[\w./-]+\.(go|js|jsx|ts|tsx|py|java|kt|kts|rs|c|cc|cpp|h|hpp|cs|rb|php|swift|scala|ex|exs|erl|hrl|lua|sh|bash|zsh|fish|ps1|sql|html|css|scss|sass|less|vue|svelte|json|yaml|yml|toml|xml|md|proto|graphql|gql|tf)\b`)

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
	normalized := normalizeTaskText(task)
	if normalized == "" {
		return false
	}

	for _, phrase := range codeChangePhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}

	words := tokenize(normalized)
	for _, prefix := range codeChangeWordPrefixes {
		for word := range words {
			if strings.HasPrefix(word, prefix) {
				return true
			}
		}
	}

	if filePathPattern.MatchString(task) && hasFileEditIntent(normalized, words) {
		return true
	}

	return false
}

func rejectedPrematureFinishResult(reason string) string {
	return "finish_status: rejected\nreason: " + reason
}

func blockerOnlyMentionsMissingTests(reason string) bool {
	lower := normalizeTaskText(reason)
	hasTestWord := strings.Contains(lower, "test") ||
		strings.Contains(lower, "unit") ||
		strings.Contains(lower, "teste") ||
		strings.Contains(lower, "_test") ||
		strings.Contains(lower, "spec")
	hasMissingWord := strings.Contains(lower, "missing") ||
		strings.Contains(lower, "no ") ||
		strings.Contains(lower, "without") ||
		strings.Contains(lower, "aus") ||
		strings.Contains(lower, "sem ") ||
		strings.Contains(lower, "provide") ||
		strings.Contains(lower, "requires") ||
		strings.Contains(lower, "required") ||
		strings.Contains(lower, "existing test structure") ||
		strings.Contains(lower, "before proceeding") ||
		strings.Contains(lower, "antes de prosseguir")
	return hasTestWord && hasMissingWord
}

var codeChangePhrases = []string{
	"重构", "修改", "编辑", "更改", "修复", "实现", "添加", "删除", "更新", "改进", "创建", "写入",
	"リファクタ", "修正", "編集", "変更", "実装", "追加", "削除", "更新", "改善", "作成",
	"리팩터", "리팩토", "수정", "편집", "변경", "구현", "추가", "삭제", "업데이트", "개선", "작성",
	"исправ", "измен", "редакт", "рефактор", "реализ", "добав", "удал", "обнов", "созда",
	"عدّل", "عدل", "تعديل", "إصلاح", "اصلح", "نفذ", "أضف", "اضف", "احذف", "حدّث", "اكتب",
}

var codeChangeWordPrefixes = []string{
	"add", "adicion", "ajout", "aggiung", "agreg", "alter", "atualiz",
	"actualiz", "camb", "chang", "corr", "crea", "cri", "edit", "escrev",
	"fix", "implement", "implément", "mejor", "melhor", "modif", "mudar",
	"patch", "refactor", "refator", "refactoriz", "refactoris", "refakt",
	"reformat", "remov", "renam", "reescrev", "rewrite", "scriv", "supprim",
	"update", "write",
}

var fileEditIntentWords = map[string]bool{
	"file": true, "arquivo": true, "fichier": true, "archivo": true, "datei": true,
	"code": true, "codigo": true, "código": true, "classe": true, "class": true,
	"function": true, "func": true, "funcao": true, "função": true, "method": true,
	"metodo": true, "método": true, "implementation": true, "implementação": true,
}

func normalizeTaskText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func hasFileEditIntent(normalized string, words map[string]bool) bool {
	for word := range fileEditIntentWords {
		if words[word] {
			return true
		}
	}
	for _, phrase := range []string{"do what codex", "faça o que o codex", "faca o que o codex", "make it like codex"} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}
