package port

import "github.com/alef-mach/tessera/internal/event"

type UIRenderer interface {
	RenderEvent(evt event.Event)
	AskApproval(evt event.Event) bool
	ReadLine(prompt string) (string, error)
}
