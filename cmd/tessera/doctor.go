package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/llm/ollama"
)

func runDoctor(ctx context.Context, cfg config.Config) error {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	fmt.Println("Tessera doctor")
	fmt.Printf("OS:             %s\n", runtime.GOOS)
	fmt.Printf("Arch:           %s\n", runtime.GOARCH)
	fmt.Printf("CWD:            %s\n", cwd)
	fmt.Printf("Provider:       %s\n", cfg.Provider)
	fmt.Printf("Model:          %s\n", cfg.Model)
	fmt.Printf("Ollama URL:     %s\n", cfg.OllamaURL)

	if cfg.Provider == "ollama" {
		printOllamaStatus(ctx, cfg.OllamaURL)
	}

	fmt.Printf("Memory:         local (%s)\n", cfg.SQLitePath)
	fmt.Printf("Context:        bounded (%d tokens)\n", cfg.ContextTokens)
	fmt.Printf("Max tokens:     %d\n", cfg.MaxTokens)
	fmt.Printf("Tessera dir:    %s\n", cfg.TesseraDir)
	fmt.Printf("Go heap alloc:  %d bytes\n", stats.Alloc)

	return nil
}

func printOllamaStatus(ctx context.Context, ollamaURL string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := ollama.Check(ctx, ollamaURL, nil); err != nil {
		fmt.Printf("Ollama: error (%s)\n", err)
		return
	}

	fmt.Println("Ollama: connected")
}
