package llm

import (
	"context"
	"encoding/json"
	"time"
)

type GenerateRequest struct {
	Prompt      string
	System      string
	MaxTokens   int
	Temperature float64
	JSONSchema  json.RawMessage
	Timeout     time.Duration
	SessionID   string
	RunID       string
}

type GenerateResponse struct {
	Text         string
	Model        string
	TokenCount   int
	InputTokens  int
	OutputTokens int
	Duration     time.Duration
	Raw          json.RawMessage
}

type LLM interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
