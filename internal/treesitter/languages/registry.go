package languages

import (
	"path/filepath"
	"strings"

	"github.com/alef-mach/tessera/internal/treesitter/language"
	"github.com/alef-mach/tessera/internal/treesitter/language/elixir"
	"github.com/alef-mach/tessera/internal/treesitter/language/golang"
	"github.com/alef-mach/tessera/internal/treesitter/language/javascript"
	"github.com/alef-mach/tessera/internal/treesitter/language/kotlin"
	"github.com/alef-mach/tessera/internal/treesitter/language/python"
)

var specs = []language.Spec{
	golang.Spec(),
	javascript.JavaScriptSpec(),
	javascript.TypeScriptSpec(),
	javascript.TSXSpec(),
	python.Spec(),
	kotlin.Spec(),
	elixir.Spec(),
}

func SpecForPath(path string) language.Spec {
	ext := strings.ToLower(filepath.Ext(path))
	for _, spec := range specs {
		for _, candidate := range spec.Extensions {
			if ext == candidate {
				return spec
			}
		}
	}
	return language.Spec{
		Name:       strings.TrimPrefix(ext, "."),
		Extensions: []string{ext},
		Regex:      javascript.RegexRules(),
	}
}

func IndexableExtensions() map[string]bool {
	extensions := map[string]bool{}
	for _, spec := range specs {
		for _, ext := range spec.Extensions {
			extensions[ext] = true
		}
	}
	return extensions
}
