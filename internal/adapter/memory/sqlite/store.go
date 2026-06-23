package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/session"
)

type MemoryStore struct {
	path       string
	tesseraDir string
	mu         sync.Mutex
}

func NewMemoryStore(path, tesseraDir string) *MemoryStore {
	return &MemoryStore{path: path, tesseraDir: tesseraDir}
}

func (s *MemoryStore) Ensure(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Join(s.tesseraDir, "sessions"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.tesseraDir, "events"), 0o755); err != nil {
		return err
	}
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func (s *MemoryStore) SaveSession(ctx context.Context, sess session.Session) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Join(s.tesseraDir, "sessions"), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.tesseraDir, "sessions", "current.json"), payload, 0o644)
}

func (s *MemoryStore) GetSession(ctx context.Context, sessionID string) (session.Session, error) {
	select {
	case <-ctx.Done():
		return session.Session{}, ctx.Err()
	default:
	}

	payload, err := os.ReadFile(filepath.Join(s.tesseraDir, "sessions", "current.json"))
	if err != nil {
		return session.Session{}, err
	}

	var sess session.Session
	if err := json.Unmarshal(payload, &sess); err != nil {
		return session.Session{}, err
	}
	if sessionID != "" && sess.ID != sessionID {
		return session.Session{}, errors.New("session not found")
	}
	return sess, nil
}

func (s *MemoryStore) SaveEvent(ctx context.Context, sessionID string, evt event.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Join(s.tesseraDir, "events"), 0o755); err != nil {
		return err
	}

	path := filepath.Join(s.tesseraDir, "events", sessionID+".jsonl")
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(payload)
	return err
}

func (s *MemoryStore) ListEvents(ctx context.Context, sessionID string) ([]event.Event, error) {
	return []event.Event{}, nil
}
