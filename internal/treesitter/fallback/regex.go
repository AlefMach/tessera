package fallback

import (
	"fmt"
	"strings"

	"github.com/alef-mach/tessera/internal/treesitter/language"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func RegexIndex(data []byte, rules language.RegexRules) ([]model.SymbolIndex, []string, []string) {
	lines := strings.Split(string(data), "\n")
	var symbols []model.SymbolIndex
	var imports []string
	var exports []string
	for idx, line := range lines {
		for _, pattern := range rules.Symbols {
			if match := pattern.Pattern.FindStringSubmatch(line); len(match) > pattern.NameGroup {
				symbols = append(symbols, model.SymbolIndex{Name: match[pattern.NameGroup], Kind: pattern.Kind, StartLine: idx + 1, EndLine: idx + 1})
			}
		}
		for _, pattern := range rules.Imports {
			if match := pattern.FindStringSubmatch(line); len(match) > 1 {
				imports = append(imports, strings.TrimSpace(match[1]))
			}
		}
		for _, pattern := range rules.Exports {
			if match := pattern.FindStringSubmatch(line); len(match) > 1 {
				exports = append(exports, strings.TrimSpace(match[1]))
			}
		}
	}
	return symbols, imports, exports
}

func LineChunks(data []byte) []model.SymbolIndex {
	lines := strings.Split(string(data), "\n")
	var chunks []model.SymbolIndex
	for start := 1; start <= len(lines); start += 120 {
		end := start + 119
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, model.SymbolIndex{Name: fmt.Sprintf("lines_%d_%d", start, end), Kind: "chunk", StartLine: start, EndLine: end})
	}
	return chunks
}
