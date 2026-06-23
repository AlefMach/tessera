package javascript

import (
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"

	"github.com/alef-mach/tessera/internal/treesitter/language"
	"github.com/alef-mach/tessera/internal/treesitter/language/common"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func JavaScriptSpec() language.Spec {
	return language.Spec{
		Name:       "javascript",
		Extensions: []string{".js", ".jsx", ".mjs", ".cjs"},
		Parser:     func() *sitter.Language { return sitter.NewLanguage(tree_sitter_javascript.Language()) },
		Tree:       treeRules(),
		Regex:      regexRules(),
	}
}

func TypeScriptSpec() language.Spec {
	return language.Spec{
		Name:       "typescript",
		Extensions: []string{".ts"},
		Parser:     func() *sitter.Language { return sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()) },
		Tree:       treeRules(),
		Regex:      regexRules(),
	}
}

func TSXSpec() language.Spec {
	return language.Spec{
		Name:       "typescript",
		Extensions: []string{".tsx"},
		Parser:     func() *sitter.Language { return sitter.NewLanguage(tree_sitter_typescript.LanguageTSX()) },
		Tree:       treeRules(),
		Regex:      regexRules(),
	}
}

func RegexRules() language.RegexRules {
	return regexRules()
}

func treeRules() language.TreeRules {
	return language.TreeRules{
		Symbol: symbol,
		Import: importPath,
		Export: export,
	}
}

func regexRules() language.RegexRules {
	return language.RegexRules{
		Symbols: []language.RegexSymbol{
			{Pattern: regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`), Kind: "function", NameGroup: 1},
			{Pattern: regexp.MustCompile(`^\s*(?:export\s+)?class\s+([A-Za-z_$][\w$]*)`), Kind: "class", NameGroup: 1},
			{Pattern: regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=`), Kind: "constant", NameGroup: 1},
			{Pattern: regexp.MustCompile(`^\s*(?:export\s+)?(?:type|interface)\s+([A-Za-z_$][\w$]*)`), Kind: "type", NameGroup: 1},
		},
		Imports: []*regexp.Regexp{regexp.MustCompile(`^\s*import.*?["']([^"']+)["']`)},
		Exports: []*regexp.Regexp{regexp.MustCompile(`^\s*export\s+\{([^}]+)\}`)},
	}
}

func symbol(node *sitter.Node, data []byte) (model.SymbolIndex, bool) {
	switch node.Kind() {
	case "function_declaration", "method_definition", "generator_function_declaration":
		return common.NamedSymbol(node, data, "function")
	case "class_declaration":
		return common.NamedSymbol(node, data, "class")
	case "interface_declaration", "type_alias_declaration":
		return common.NamedSymbol(node, data, "type")
	case "lexical_declaration", "variable_declaration":
		if exportedOrConst(node, data) {
			name := common.FirstIdentifier(node, data)
			if name != "" {
				return common.SymbolFromNode(node, name, "constant"), true
			}
		}
	}
	return model.SymbolIndex{}, false
}

func importPath(node *sitter.Node, data []byte) (string, bool) {
	if node.Kind() == "import_statement" || node.Kind() == "export_statement" {
		return quotedModule(node.Utf8Text(data))
	}
	return "", false
}

func export(node *sitter.Node, data []byte) []string {
	if node.Kind() != "export_statement" {
		return nil
	}
	return exportNames(strings.TrimSpace(node.Utf8Text(data)))
}

func exportedOrConst(node *sitter.Node, data []byte) bool {
	text := strings.TrimSpace(node.Utf8Text(data))
	return strings.HasPrefix(text, "const ") || strings.Contains(text, " const ")
}

func quotedModule(text string) (string, bool) {
	matches := regexp.MustCompile(`(?m)(?:from|import)\s+["']([^"']+)["']`).FindStringSubmatch(text)
	if len(matches) == 2 {
		return matches[1], true
	}
	return "", false
}

func exportNames(text string) []string {
	var names []string
	if strings.HasPrefix(text, "export default") {
		names = append(names, "default")
	}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`export\s+(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`export\s+class\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`export\s+(?:const|let|var)\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`export\s+(?:type|interface)\s+([A-Za-z_$][\w$]*)`),
	} {
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			names = append(names, match[1])
		}
	}
	if block := regexp.MustCompile(`export\s*\{([^}]+)\}`).FindStringSubmatch(text); len(block) == 2 {
		for _, part := range strings.Split(block[1], ",") {
			name := strings.TrimSpace(part)
			if before, _, ok := strings.Cut(name, " as "); ok {
				name = strings.TrimSpace(before)
			}
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
