package language

import (
	"regexp"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/alef-mach/tessera/internal/treesitter/model"
)

type Spec struct {
	Name       string
	Extensions []string
	Parser     func() *sitter.Language
	Tree       TreeRules
	Regex      RegexRules
}

type TreeRules struct {
	Symbol func(*sitter.Node, []byte) (model.SymbolIndex, bool)
	Import func(*sitter.Node, []byte) (string, bool)
	Export func(*sitter.Node, []byte) []string
}

type RegexRules struct {
	Symbols []RegexSymbol
	Imports []*regexp.Regexp
	Exports []*regexp.Regexp
}

type RegexSymbol struct {
	Pattern   *regexp.Regexp
	Kind      string
	NameGroup int
}
