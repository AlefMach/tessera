package event

import "time"

// Event is the internal stream contract between core and consumers.
type Event struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

func New(eventType, title, message string, data map[string]any) Event {
	return Event{
		Type:      eventType,
		Title:     title,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}
