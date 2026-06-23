package python

import (
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"

	"github.com/alef-mach/tessera/internal/treesitter/language"
	"github.com/alef-mach/tessera/internal/treesitter/language/common"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func Spec() language.Spec {
	return language.Spec{
		Name:       "python",
		Extensions: []string{".py"},
		Parser:     func() *sitter.Language { return sitter.NewLanguage(tree_sitter_python.Language()) },
		Tree: language.TreeRules{
			Symbol: symbol,
			Import: importPath,
			Export: export,
		},
		Regex: language.RegexRules{
			Symbols: []language.RegexSymbol{
				{Pattern: regexp.MustCompile(`^\s*def\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "function", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "class", NameGroup: 1},
			},
			Imports: []*regexp.Regexp{regexp.MustCompile(`^\s*(import\s+.+|from\s+.+\s+import\s+.+)$`)},
			Exports: []*regexp.Regexp{regexp.MustCompile(`^\s*__all__\s*=\s*(.+)$`)},
		},
	}
}

func symbol(node *sitter.Node, data []byte) (model.SymbolIndex, bool) {
	switch node.Kind() {
	case "function_definition":
		return common.NamedSymbol(node, data, "function")
	case "class_definition":
		return common.NamedSymbol(node, data, "class")
	default:
		return model.SymbolIndex{}, false
	}
}

func importPath(node *sitter.Node, data []byte) (string, bool) {
	if node.Kind() == "import_statement" || node.Kind() == "import_from_statement" {
		return strings.TrimSpace(node.Utf8Text(data)), true
	}
	return "", false
}

func export(node *sitter.Node, data []byte) []string {
	text := strings.TrimSpace(node.Utf8Text(data))
	if strings.HasPrefix(text, "__all__") {
		return common.QuotedNames(text)
	}
	return nil
}
