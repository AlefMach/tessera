package elixir

import (
	"regexp"

	"github.com/alef-mach/tessera/internal/treesitter/language"
)

func Spec() language.Spec {
	return language.Spec{
		Name:       "elixir",
		Extensions: []string{".ex", ".exs"},
		Regex: language.RegexRules{
			Symbols: []language.RegexSymbol{
				{Pattern: regexp.MustCompile(`^\s*defmodule\s+([A-Za-z0-9_.!?\-]+)`), Kind: "module", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*defp?\s+([A-Za-z0-9_!?]+)`), Kind: "function", NameGroup: 1},
				{Pattern: regexp.MustCompile(`^\s*@([A-Z][A-Za-z0-9_]*)`), Kind: "constant", NameGroup: 1},
			},
			Imports: []*regexp.Regexp{regexp.MustCompile(`^\s*(?:alias|import|require|use)\s+(.+)$`)},
		},
	}
}
