package port

import "context"

type GenerateRequest struct {
	Prompt      string
	System      string
	MaxTokens   int
	Temperature float64
}

type GenerateResponse struct {
	Text       string
	Model      string
	TokenCount int
}

type LLM interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
