package plain

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alef-mach/tessera/internal/event"
)

type Renderer struct {
	in  *bufio.Reader
	out *os.File
}

func NewRenderer() *Renderer {
	return &Renderer{in: bufio.NewReader(os.Stdin), out: os.Stdout}
}

func (r *Renderer) RenderEvent(evt event.Event) {
	if evt.Message == "" {
		fmt.Fprintf(r.out, "● %s\n", evt.Title)
		return
	}
	fmt.Fprintf(r.out, "● %s\n", evt.Title)
	for _, line := range strings.Split(evt.Message, "\n") {
		fmt.Fprintf(r.out, "  %s\n", line)
	}
}

func (r *Renderer) AskApproval(evt event.Event) bool {
	r.RenderEvent(evt)
	fmt.Fprint(r.out, "Approve? [y/N] ")

	answer, err := r.in.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func (r *Renderer) ReadLine(prompt string) (string, error) {
	fmt.Fprint(r.out, prompt)
	return r.in.ReadString('\n')
}
