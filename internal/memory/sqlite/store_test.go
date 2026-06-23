package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/session"
)

func TestMemoryStorePersistsSessionRunCallAndObservations(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	sess := session.Session{
		ID:        "sess-test",
		CWD:       "/tmp/project",
		Provider:  "ollama",
		Model:     "llama",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	run := memory.Run{
		ID:        "run-test",
		SessionID: sess.ID,
		Input:     "do work",
		Status:    "finished",
		Steps:     2,
		Calls:     1,
		StartedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatal(err)
	}

	call := memory.LLMCall{
		ID:           "call-test",
		SessionID:    sess.ID,
		RunID:        run.ID,
		Provider:     "ollama",
		Model:        "llama",
		Prompt:       "prompt",
		System:       "system",
		Response:     "response",
		InputTokens:  10,
		OutputTokens: 5,
		DurationMS:   42,
		CreatedAt:    now,
	}
	if err := store.SaveCall(ctx, call); err != nil {
		t.Fatal(err)
	}

	observation := memory.Observation{
		ID:        "obs-test",
		SessionID: sess.ID,
		RunID:     run.ID,
		Kind:      "note",
		Content:   "observed state",
		Data:      map[string]any{"file": "main.go"},
		CreatedAt: now,
	}
	if err := store.SaveObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveEvent(ctx, sess.ID, event.New("task.received", "Task received", "do work", map[string]any{"input": "do work"})); err != nil {
		t.Fatal(err)
	}

	gotSession, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotSession.ID != sess.ID || gotSession.Model != "llama" {
		t.Fatalf("unexpected session: %#v", gotSession)
	}

	gotRun, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotRun.Steps != 2 || gotRun.Status != "finished" {
		t.Fatalf("unexpected run: %#v", gotRun)
	}

	gotCall, err := store.GetCall(ctx, call.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotCall.OutputTokens != 5 || gotCall.RunID != run.ID {
		t.Fatalf("unexpected call: %#v", gotCall)
	}

	observations, err := store.ListObservations(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(observations))
	}

	events, err := store.ListEvents(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "task.received" {
		t.Fatalf("unexpected events: %#v", events)
	}

	stats, err := store.Stats(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Calls != 1 || stats.Steps != 2 || stats.Runs != 1 || stats.Observations != 2 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}
