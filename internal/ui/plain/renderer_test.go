package plain

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/alef-mach/tessera/internal/event"
)

func TestRenderSessionHeaderDoesNotClearScreen(t *testing.T) {
	var out bytes.Buffer
	renderer := NewRendererWithIO(strings.NewReader(""), &out)
	renderer.markdown = nil

	renderer.RenderEvent(event.Event{
		Type:      "session.started",
		Title:     "Session started",
		Message:   "Type your task.",
		Timestamp: time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC),
		Data: map[string]any{
			"session_id":     "sess-1",
			"provider":       "ollama",
			"model":          "qwen",
			"context_tokens": 4096,
			"max_tokens":     1024,
			"calls":          0,
			"cwd":            "/repo",
		},
	})

	got := out.String()
	for _, want := range []string{"Tessera", "Model:", "ollama/qwen", "Context:", "4096 tokens", "Project:", "/repo"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
	assertNoClearScreen(t, got)
}

func TestRenderPatchProposedIncludesCopyFriendlyDiff(t *testing.T) {
	var out bytes.Buffer
	renderer := NewRendererWithIO(strings.NewReader(""), &out)
	renderer.markdown = nil

	diff := "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n"
	renderer.RenderEvent(event.Event{
		Type:    "patch.proposed",
		Title:   "Patch proposed",
		Message: "Review changes.",
		Data:    map[string]any{"diff": diff},
	})

	got := out.String()
	for _, want := range []string{"Patch proposed", "diff --git a/a.go b/a.go", "@@ -1 +1 @@", "-old", "+new"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
	assertNoClearScreen(t, got)
}

func TestAskApprovalCanShowDiffBeforeYes(t *testing.T) {
	input := strings.NewReader("d\ny\n")
	var out bytes.Buffer
	renderer := NewRendererWithIO(input, &out)
	renderer.markdown = nil

	ok := renderer.AskApproval(event.Event{
		Type:  "approval.requested",
		Title: "Approval requested",
		Data:  map[string]any{"diff": "@@ -1 +1 @@\n-no\n+yes\n"},
	})

	if !ok {
		t.Fatal("expected approval to return true")
	}
	got := out.String()
	for _, want := range []string{"[y] yes [n] no [d] diff", "-no", "+yes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
	assertNoClearScreen(t, got)
}

func assertNoClearScreen(t *testing.T, got string) {
	t.Helper()
	for _, forbidden := range []string{"\x1b[2J", "\x1b[3J", "\x1b[H\x1b[2J"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("output contains clear-screen sequence %q", forbidden)
		}
	}
}
