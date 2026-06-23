package plain

import (
	"fmt"
	"strings"

	"github.com/alef-mach/tessera/internal/event"
)

func (r *Renderer) AskApproval(evt event.Event) bool {
	r.RenderEvent(evt)

	for {
		fmt.Fprint(r.out, r.styles.prompt.Render("[y] yes [n] no [d] diff")+" ")
		answer, err := r.in.ReadString('\n')
		if err != nil {
			return false
		}

		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		case "d", "diff":
			diff := dataString(evt.Data, "diff", "patch")
			if diff == "" {
				fmt.Fprintln(r.out, r.styles.muted.Render("No diff available."))
				continue
			}
			r.writeBlock(renderDiff(diff, r.diffStyles))
		default:
			fmt.Fprintln(r.out, r.styles.muted.Render("Choose y, n, or d."))
		}
	}
}
