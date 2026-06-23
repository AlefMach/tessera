package common

import (
	"regexp"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func NamedSymbol(node *sitter.Node, data []byte, kind string) (model.SymbolIndex, bool) {
	name := NodeName(node, data)
	if name == "" {
		return model.SymbolIndex{}, false
	}
	return SymbolFromNode(node, name, kind), true
}

func SymbolFromNode(node *sitter.Node, name, kind string) model.SymbolIndex {
	return model.SymbolIndex{
		Name:      name,
		Kind:      kind,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

func NodeName(node *sitter.Node, data []byte) string {
	if child := node.ChildByFieldName("name"); child != nil {
		return strings.TrimSpace(child.Utf8Text(data))
	}
	return FirstIdentifier(node, data)
}

func FirstIdentifier(node *sitter.Node, data []byte) string {
	for child := uint(0); child < node.NamedChildCount(); child++ {
		c := node.NamedChild(child)
		switch c.Kind() {
		case "identifier", "property_identifier", "type_identifier":
			return strings.TrimSpace(c.Utf8Text(data))
		}
		if name := FirstIdentifier(c, data); name != "" {
			return name
		}
	}
	return ""
}

func FirstNamedChildText(node *sitter.Node, data []byte) string {
	if node.NamedChildCount() == 0 {
		return ""
	}
	return strings.TrimSpace(node.NamedChild(0).Utf8Text(data))
}

func QuotedNames(text string) []string {
	re := regexp.MustCompile(`["']([^"']+)["']`)
	var names []string
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		names = append(names, match[1])
	}
	return names
}
