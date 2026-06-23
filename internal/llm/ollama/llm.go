package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory"
)

const (
	defaultBaseURL = "http://localhost:11434"
	defaultRetries = 1
)

type OllamaLLM struct {
	baseURL        string
	model          string
	httpClient     *http.Client
	store          memory.Store
	defaultTimeout time.Duration
	retries        int
}

type Option func(*OllamaLLM)

func NewOllamaLLM(baseURL, model string, opts ...Option) *OllamaLLM {
	client := &OllamaLLM{
		baseURL:    normalizeBaseURL(baseURL),
		model:      model,
		httpClient: http.DefaultClient,
		retries:    defaultRetries,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(l *OllamaLLM) {
		if httpClient != nil {
			l.httpClient = httpClient
		}
	}
}

func WithMemoryStore(store memory.Store) Option {
	return func(l *OllamaLLM) {
		l.store = store
	}
}

func WithDefaultTimeout(timeout time.Duration) Option {
	return func(l *OllamaLLM) {
		l.defaultTimeout = timeout
	}
}

func WithRetries(retries int) Option {
	return func(l *OllamaLLM) {
		if retries >= 0 {
			l.retries = retries
		}
	}
}

func (l *OllamaLLM) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	start := time.Now().UTC()
	callCtx := ctx
	cancel := func() {}
	if timeout := requestTimeout(req.Timeout, l.defaultTimeout); timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	response, err := l.generate(callCtx, req)
	duration := time.Since(start)
	response.Duration = duration

	saveErr := l.saveCall(ctx, req, response, duration, err, start)
	if err != nil {
		return response, err
	}
	if saveErr != nil {
		return response, saveErr
	}
	return response, nil
}

func (l *OllamaLLM) generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	payload, err := l.requestPayload(req)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.GenerateResponse{}, err
	}

	var lastErr error
	attempts := l.retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, 100*time.Millisecond); err != nil {
				return llm.GenerateResponse{}, err
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, l.endpoint("/api/generate"), bytes.NewReader(body))
		if err != nil {
			return llm.GenerateResponse{}, err
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := l.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if isNetworkError(err) && attempt+1 < attempts {
				continue
			}
			return llm.GenerateResponse{}, err
		}

		raw, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt+1 < attempts {
				continue
			}
			return llm.GenerateResponse{}, readErr
		}
		if closeErr != nil {
			lastErr = closeErr
			if attempt+1 < attempts {
				continue
			}
			return llm.GenerateResponse{}, closeErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return llm.GenerateResponse{}, fmt.Errorf("ollama generate failed: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
		}

		var decoded generateResponse
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return llm.GenerateResponse{}, err
		}
		return llm.GenerateResponse{
			Text:         decoded.Response,
			Model:        pickModel(decoded.Model, l.model),
			TokenCount:   decoded.PromptEvalCount + decoded.EvalCount,
			InputTokens:  decoded.PromptEvalCount,
			OutputTokens: decoded.EvalCount,
			Raw:          raw,
		}, nil
	}
	return llm.GenerateResponse{}, lastErr
}

func (l *OllamaLLM) requestPayload(req llm.GenerateRequest) (map[string]any, error) {
	if strings.TrimSpace(l.model) == "" {
		return nil, errors.New("ollama model is required")
	}
	payload := map[string]any{
		"model":  l.model,
		"prompt": req.Prompt,
		"stream": false,
	}
	if req.System != "" {
		payload["system"] = req.System
	}
	options := map[string]any{}
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		options["temperature"] = req.Temperature
	}
	if len(options) > 0 {
		payload["options"] = options
	}
	if len(req.JSONSchema) > 0 {
		var schema any
		if err := json.Unmarshal(req.JSONSchema, &schema); err != nil {
			return nil, fmt.Errorf("invalid JSON schema: %w", err)
		}
		payload["format"] = schema
	}
	return payload, nil
}

func (l *OllamaLLM) saveCall(ctx context.Context, req llm.GenerateRequest, resp llm.GenerateResponse, duration time.Duration, callErr error, createdAt time.Time) error {
	if l.store == nil || req.SessionID == "" {
		return nil
	}
	errorText := ""
	if callErr != nil {
		errorText = callErr.Error()
	}
	model := resp.Model
	if model == "" {
		model = l.model
	}
	return l.store.SaveCall(ctx, memory.LLMCall{
		ID:           "llm-" + createdAt.Format("20060102-150405.000000000"),
		SessionID:    req.SessionID,
		RunID:        req.RunID,
		Provider:     "ollama",
		Model:        model,
		Prompt:       req.Prompt,
		System:       req.System,
		Response:     resp.Text,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		DurationMS:   duration.Milliseconds(),
		Error:        errorText,
		CreatedAt:    createdAt,
	})
}

func Check(ctx context.Context, baseURL string, httpClient *http.Client) error {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizeBaseURL(baseURL)+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func (l *OllamaLLM) endpoint(path string) string {
	return l.baseURL + path
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(baseURL, "/")
	}
	return strings.TrimRight(parsed.String(), "/")
}

func requestTimeout(request, fallback time.Duration) time.Duration {
	if request > 0 {
		return request
	}
	return fallback
}

func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func pickModel(actual, fallback string) string {
	if actual != "" {
		return actual
	}
	return fallback
}

type generateResponse struct {
	Model           string `json:"model"`
	Response        string `json:"response"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}
