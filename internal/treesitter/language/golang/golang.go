package golang

import (
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"

	"github.com/alef-mach/tessera/internal/treesitter/language"
	"github.com/alef-mach/tessera/internal/treesitter/language/common"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func Spec() language.Spec {
	return language.Spec{
		Name:       "go",
		Extensions: []string{".go"},
		Parser:     func() *sitter.Language { return sitter.NewLanguage(tree_sitter_go.Language()) },
		Tree: language.TreeRules{
			Symbol: symbol,
			Import: importPath,
			Export: export,
		},
		Regex: language.RegexRules{
			Symbols: []language.RegexSymbol{
				{Pattern: regexp.MustCompile(`^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`), Kind: "function", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "type", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*const\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "constant", NameGroup: 1},
			},
			Imports: []*regexp.Regexp{regexp.MustCompile(`^\s*import\s+"([^"]+)"`)},
		},
	}
}

func symbol(node *sitter.Node, data []byte) (model.SymbolIndex, bool) {
	switch node.Kind() {
	case "function_declaration", "method_declaration":
		return common.NamedSymbol(node, data, "function")
	case "type_spec":
		return common.NamedSymbol(node, data, "type")
	case "const_spec":
		name := common.FirstNamedChildText(node, data)
		if name != "" {
			return common.SymbolFromNode(node, name, "constant"), true
		}
	}
	return model.SymbolIndex{}, false
}

func importPath(node *sitter.Node, data []byte) (string, bool) {
	if node.Kind() != "import_spec" {
		return "", false
	}
	text := strings.Trim(strings.TrimSpace(node.Utf8Text(data)), `"`)
	if idx := strings.LastIndex(text, " "); idx >= 0 {
		text = strings.Trim(strings.TrimSpace(text[idx+1:]), `"`)
	}
	return text, text != ""
}

func export(node *sitter.Node, data []byte) []string {
	symbol, ok := symbol(node, data)
	if ok && isExported(symbol.Name) {
		return []string{symbol.Name}
	}
	return nil
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)[0]
	return r >= 'A' && r <= 'Z'
}
