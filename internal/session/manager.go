package session

import (
	"context"
	"os"
)

type Store interface {
	SaveSession(ctx context.Context, sess Session) error
	GetSession(ctx context.Context, sessionID string) (Session, error)
}

type Manager struct {
	store Store
}

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Start(ctx context.Context, provider, model string) (Session, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Session{}, err
	}
	sess := New(makeID(), cwd, provider, model)
	if err := m.store.SaveSession(ctx, sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (m *Manager) Current(ctx context.Context) (Session, error) {
	return m.store.GetSession(ctx, "")
}
