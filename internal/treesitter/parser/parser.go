package parser

import (
	"errors"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/alef-mach/tessera/internal/treesitter/language"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func Parse(data []byte, spec language.Spec) (model.FileIndex, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(spec.Parser()); err != nil {
		return model.FileIndex{}, err
	}
	tree := parser.Parse(data, nil)
	if tree == nil {
		return model.FileIndex{}, errors.New("tree-sitter parse returned nil")
	}
	defer tree.Close()

	out := model.FileIndex{Language: spec.Name}
	walk(tree.RootNode(), data, spec.Tree, &out)
	return out, nil
}

func walk(node *sitter.Node, data []byte, rules language.TreeRules, out *model.FileIndex) {
	if node == nil {
		return
	}
	if rules.Symbol != nil {
		if symbol, ok := rules.Symbol(node, data); ok {
			out.Symbols = append(out.Symbols, symbol)
		}
	}
	if rules.Import != nil {
		if imp, ok := rules.Import(node, data); ok {
			out.Imports = append(out.Imports, imp)
		}
	}
	if rules.Export != nil {
		if exports := rules.Export(node, data); len(exports) > 0 {
			out.Exports = append(out.Exports, exports...)
		}
	}
	for child := uint(0); child < node.NamedChildCount(); child++ {
		walk(node.NamedChild(child), data, rules, out)
	}
}
