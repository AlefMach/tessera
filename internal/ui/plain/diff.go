package plain

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type diffStyles struct {
	header  lipgloss.Style
	hunk    lipgloss.Style
	added   lipgloss.Style
	removed lipgloss.Style
	meta    lipgloss.Style
}

func newDiffStyles() diffStyles {
	return diffStyles{
		header:  lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true),
		hunk:    lipgloss.NewStyle().Foreground(lipgloss.Color("177")),
		added:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		removed: lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		meta:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

func renderDiff(diff string, styles diffStyles) string {
	diff = strings.TrimRight(diff, "\n")
	if diff == "" {
		return ""
	}

	lines := strings.Split(diff, "\n")
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"), strings.HasPrefix(line, "index "):
			rendered = append(rendered, styles.header.Render(line))
		case strings.HasPrefix(line, "@@"):
			rendered = append(rendered, styles.hunk.Render(line))
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			rendered = append(rendered, styles.added.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			rendered = append(rendered, styles.removed.Render(line))
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			rendered = append(rendered, styles.meta.Render(line))
		default:
			rendered = append(rendered, line)
		}
	}
	return strings.Join(rendered, "\n")
}
