package memory

import (
	"context"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/project"
	"github.com/alef-mach/tessera/internal/session"
)

type Run struct {
	ID        string
	SessionID string
	Input     string
	Status    string
	Steps     int
	Calls     int
	StartedAt time.Time
	UpdatedAt time.Time
	EndedAt   *time.Time
}

type LLMCall struct {
	ID           string
	SessionID    string
	RunID        string
	Provider     string
	Model        string
	Prompt       string
	System       string
	Response     string
	InputTokens  int
	OutputTokens int
	DurationMS   int64
	Error        string
	CreatedAt    time.Time
}

type Observation struct {
	ID        string
	SessionID string
	RunID     string
	Kind      string
	Content   string
	Data      map[string]any
	CreatedAt time.Time
}

type FileSummary struct {
	ID             string
	SessionID      string
	Path           string
	Language       string
	Summary        string
	Hash           string
	Imports        []string
	Exports        []string
	HasTestsNearby bool
	UpdatedAt      time.Time
}

type Symbol struct {
	ID        string
	SessionID string
	Name      string
	Kind      string
	Path      string
	Line      int
	StartLine int
	EndLine   int
	Summary   string
	UpdatedAt time.Time
}

type Stats struct {
	SessionID     string
	Model         string
	Provider      string
	Calls         int
	Steps         int
	Runs          int
	Observations  int
	FileSummaries int
	Symbols       int
}

type Store interface {
	Ensure(ctx context.Context) error
	SaveSession(ctx context.Context, sess session.Session) error
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
	ListSessions(ctx context.Context) ([]session.Session, error)
	SaveRun(ctx context.Context, run Run) error
	GetRun(ctx context.Context, runID string) (Run, error)
	ListRuns(ctx context.Context, sessionID string) ([]Run, error)
	SaveCall(ctx context.Context, call LLMCall) error
	GetCall(ctx context.Context, callID string) (LLMCall, error)
	ListCalls(ctx context.Context, sessionID string) ([]LLMCall, error)
	SaveObservation(ctx context.Context, observation Observation) error
	GetObservation(ctx context.Context, observationID string) (Observation, error)
	ListObservations(ctx context.Context, sessionID string) ([]Observation, error)
	SaveFileSummary(ctx context.Context, summary FileSummary) error
	GetFileSummary(ctx context.Context, sessionID, path string) (FileSummary, error)
	ListFileSummaries(ctx context.Context, sessionID string) ([]FileSummary, error)
	SaveSymbol(ctx context.Context, symbol Symbol) error
	GetSymbol(ctx context.Context, symbolID string) (Symbol, error)
	ListSymbols(ctx context.Context, sessionID string) ([]Symbol, error)
	ClearIndex(ctx context.Context, sessionID string) error
	SaveProjectProfile(ctx context.Context, profile project.ProjectProfile) error
	GetProjectProfile(ctx context.Context, sessionID string) (project.ProjectProfile, error)
	SaveEvent(ctx context.Context, sessionID string, evt event.Event) error
	ListEvents(ctx context.Context, sessionID string) ([]event.Event, error)
	Stats(ctx context.Context, sessionID string) (Stats, error)
}
