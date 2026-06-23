package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/session"
)

func TestGenerateSendsStructuredOutputSchemaAndPersistsCall(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemoryStore(filepath.Join(t.TempDir(), "memory.db"))
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	sess := session.Session{
		ID:        "sess-test",
		CWD:       t.TempDir(),
		Provider:  "ollama",
		Model:     "llama3.2",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	var gotPayload map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(200, `{"model":"llama3.2","response":"{\"ok\":true}","prompt_eval_count":7,"eval_count":3}`), nil
	})}

	client := NewOllamaLLM("http://ollama.local", "llama3.2", WithMemoryStore(store), WithHTTPClient(httpClient))
	resp, err := client.Generate(ctx, llm.GenerateRequest{
		Prompt:      "return json",
		System:      "be precise",
		MaxTokens:   64,
		Temperature: 0.2,
		JSONSchema:  json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"]}`),
		SessionID:   sess.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != `{"ok":true}` || resp.TokenCount != 10 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if gotPayload["model"] != "llama3.2" || gotPayload["stream"] != false {
		t.Fatalf("unexpected payload: %#v", gotPayload)
	}
	if gotPayload["format"] == nil {
		t.Fatalf("expected format schema in payload: %#v", gotPayload)
	}
	options, ok := gotPayload["options"].(map[string]any)
	if !ok {
		t.Fatalf("expected options in payload: %#v", gotPayload)
	}
	if options["num_predict"] != float64(64) || options["temperature"] != 0.2 {
		t.Fatalf("unexpected options: %#v", options)
	}

	calls, err := store.ListCalls(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 saved call, got %d", len(calls))
	}
	if calls[0].Prompt != "return json" || calls[0].Response != `{"ok":true}` || calls[0].OutputTokens != 3 {
		t.Fatalf("unexpected saved call: %#v", calls[0])
	}
}

func TestGenerateRetriesNetworkFailure(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, timeoutError{}
		}
		return jsonResponse(200, `{"model":"llama3.2","response":"ok","prompt_eval_count":1,"eval_count":1}`), nil
	})}

	client := NewOllamaLLM("http://ollama.local", "llama3.2", WithRetries(1), WithHTTPClient(httpClient))
	resp, err := client.Generate(ctx, llm.GenerateRequest{Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" || attempts != 2 {
		t.Fatalf("expected retry success, response=%#v attempts=%d", resp, attempts)
	}
}

func TestCheckReportsConnectedEndpoint(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(200, `{"models":[]}`), nil
	})}

	if err := Check(context.Background(), "http://ollama.local", httpClient); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "temporary network failure" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ interface {
	error
	Timeout() bool
	Temporary() bool
} = timeoutError{}
