package repomap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func Build(files []model.FileIndex) string {
	sort.SliceStable(files, func(a, b int) bool {
		return files[a].Path < files[b].Path
	})
	var b strings.Builder
	for _, file := range files {
		fmt.Fprintf(&b, "%s [%s]", file.Path, file.Language)
		if file.HasTestsNearby {
			b.WriteString(" tests_nearby")
		}
		if file.Fallback != "" {
			fmt.Fprintf(&b, " fallback=%s", file.Fallback)
		}
		b.WriteByte('\n')
		if len(file.Imports) > 0 {
			fmt.Fprintf(&b, "  imports: %s\n", strings.Join(file.Imports, ", "))
		}
		if len(file.Exports) > 0 {
			fmt.Fprintf(&b, "  exports: %s\n", strings.Join(file.Exports, ", "))
		}
		for _, symbol := range file.Symbols {
			fmt.Fprintf(&b, "  - %s %s:%d-%d\n", symbol.Kind, symbol.Name, symbol.StartLine, symbol.EndLine)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func Normalize(index model.FileIndex) model.FileIndex {
	index.Imports = uniqueStrings(index.Imports)
	index.Exports = uniqueStrings(index.Exports)
	sort.SliceStable(index.Symbols, func(a, b int) bool {
		if index.Symbols[a].StartLine == index.Symbols[b].StartLine {
			return index.Symbols[a].Name < index.Symbols[b].Name
		}
		return index.Symbols[a].StartLine < index.Symbols[b].StartLine
	})
	return index
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
