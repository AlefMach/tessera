package redis

import (
	"context"
	"errors"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/project"
	"github.com/alef-mach/tessera/internal/session"
)

var ErrNotImplemented = errors.New("redis memory store is not implemented")

type MemoryStore struct{}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Ensure(ctx context.Context) error { return ErrNotImplemented }
func (s *MemoryStore) SaveSession(ctx context.Context, sess session.Session) error {
	return ErrNotImplemented
}
func (s *MemoryStore) GetSession(ctx context.Context, sessionID string) (session.Session, error) {
	return session.Session{}, ErrNotImplemented
}
func (s *MemoryStore) ListSessions(ctx context.Context) ([]session.Session, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) SaveRun(ctx context.Context, run memory.Run) error { return ErrNotImplemented }
func (s *MemoryStore) GetRun(ctx context.Context, runID string) (memory.Run, error) {
	return memory.Run{}, ErrNotImplemented
}
func (s *MemoryStore) ListRuns(ctx context.Context, sessionID string) ([]memory.Run, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) SaveCall(ctx context.Context, call memory.LLMCall) error {
	return ErrNotImplemented
}
func (s *MemoryStore) GetCall(ctx context.Context, callID string) (memory.LLMCall, error) {
	return memory.LLMCall{}, ErrNotImplemented
}
func (s *MemoryStore) ListCalls(ctx context.Context, sessionID string) ([]memory.LLMCall, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) SaveObservation(ctx context.Context, observation memory.Observation) error {
	return ErrNotImplemented
}
func (s *MemoryStore) GetObservation(ctx context.Context, observationID string) (memory.Observation, error) {
	return memory.Observation{}, ErrNotImplemented
}
func (s *MemoryStore) ListObservations(ctx context.Context, sessionID string) ([]memory.Observation, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) SaveFileSummary(ctx context.Context, summary memory.FileSummary) error {
	return ErrNotImplemented
}
func (s *MemoryStore) GetFileSummary(ctx context.Context, sessionID, path string) (memory.FileSummary, error) {
	return memory.FileSummary{}, ErrNotImplemented
}
func (s *MemoryStore) ListFileSummaries(ctx context.Context, sessionID string) ([]memory.FileSummary, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) SaveSymbol(ctx context.Context, symbol memory.Symbol) error {
	return ErrNotImplemented
}
func (s *MemoryStore) GetSymbol(ctx context.Context, symbolID string) (memory.Symbol, error) {
	return memory.Symbol{}, ErrNotImplemented
}
func (s *MemoryStore) ListSymbols(ctx context.Context, sessionID string) ([]memory.Symbol, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) SaveProjectProfile(ctx context.Context, profile project.ProjectProfile) error {
	return ErrNotImplemented
}
func (s *MemoryStore) GetProjectProfile(ctx context.Context, sessionID string) (project.ProjectProfile, error) {
	return project.ProjectProfile{}, ErrNotImplemented
}
func (s *MemoryStore) SaveEvent(ctx context.Context, sessionID string, evt event.Event) error {
	return ErrNotImplemented
}
func (s *MemoryStore) ListEvents(ctx context.Context, sessionID string) ([]event.Event, error) {
	return nil, ErrNotImplemented
}
func (s *MemoryStore) Stats(ctx context.Context, sessionID string) (memory.Stats, error) {
	return memory.Stats{}, ErrNotImplemented
}
