package session

import "time"

type Session struct {
	ID        string    `json:"id"`
	CWD       string    `json:"cwd"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func New(id, cwd, provider, model string) Session {
	now := time.Now().UTC()
	return Session{
		ID:        id,
		CWD:       cwd,
		Provider:  provider,
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func makeID() string {
	return "sess-" + time.Now().UTC().Format("20060102-150405")
}
