package kotlin

import (
	"regexp"

	"github.com/alef-mach/tessera/internal/treesitter/language"
)

func Spec() language.Spec {
	return language.Spec{
		Name:       "kotlin",
		Extensions: []string{".kt", ".kts"},
		Regex: language.RegexRules{
			Symbols: []language.RegexSymbol{
				{Pattern: regexp.MustCompile(`^\s*(?:public\s+|private\s+|internal\s+|protected\s+)?(?:suspend\s+)?fun\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "function", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*(?:public\s+|private\s+|internal\s+|protected\s+)?(?:data\s+|sealed\s+|abstract\s+|open\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "class", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*(?:public\s+|private\s+|internal\s+|protected\s+)?interface\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "type", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*(?:const\s+)?val\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "constant", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*object\s+([A-Za-z_][A-Za-z0-9_]*)`), Kind: "module", NameGroup: 1},
			},
			Imports: []*regexp.Regexp{regexp.MustCompile(`^\s*import\s+(.+)$`)},
		},
	}
}
