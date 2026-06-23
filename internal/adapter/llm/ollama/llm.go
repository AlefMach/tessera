package ollama

import (
	"context"
	"errors"

	"github.com/alef-mach/tessera/internal/port"
)

type LLM struct {
	baseURL string
	model   string
}

func NewLLM(baseURL, model string) *LLM {
	return &LLM{baseURL: baseURL, model: model}
}

func (l *LLM) Generate(ctx context.Context, req port.GenerateRequest) (port.GenerateResponse, error) {
	return port.GenerateResponse{}, errors.New("ollama adapter is not implemented yet")
}
