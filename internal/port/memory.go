package port

import (
	"context"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/session"
)

type MemoryStore interface {
	Ensure(ctx context.Context) error
	SaveSession(ctx context.Context, sess session.Session) error
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
	SaveEvent(ctx context.Context, sessionID string, evt event.Event) error
	ListEvents(ctx context.Context, sessionID string) ([]event.Event, error)
}
