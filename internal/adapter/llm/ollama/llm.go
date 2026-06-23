package ollama

import newollama "github.com/alef-mach/tessera/internal/llm/ollama"

type LLM = newollama.OllamaLLM

func NewLLM(baseURL, model string) *LLM {
	return newollama.NewOllamaLLM(baseURL, model)
}
