package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/alef-mach/tessera/internal/adapter/executor/localexec"
	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/llm/ollama"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/orchestrator"
	"github.com/alef-mach/tessera/internal/trust"
	"github.com/alef-mach/tessera/internal/ui/plain"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var flags config.Flags

	root := &cobra.Command{
		Use:   "tessera",
		Short: "Local-first bounded-context coding agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFromCommand(cmd, flags)
			if err != nil {
				return err
			}
			return runInteractive(cmd.Context(), cfg)
		},
	}

	bindConfigFlags(root, &flags)

	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Start an interactive Tessera session",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFromCommand(cmd, flags)
			if err != nil {
				return err
			}
			return runInteractive(cmd.Context(), cfg)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Show local Tessera environment diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFromCommand(cmd, flags)
			if err != nil {
				return err
			}
			return runDoctor(cfg)
		},
	})

	root.AddCommand(stubCommand("index", "Index project files"))
	root.AddCommand(stubCommand("config", "Inspect or update Tessera config"))
	root.AddCommand(stubCommand("memory", "Inspect Tessera memory"))
	root.AddCommand(stubCommand("replay", "Replay a previous Tessera session"))

	return root
}

func bindConfigFlags(cmd *cobra.Command, flags *config.Flags) {
	cmd.PersistentFlags().StringVar(&flags.Provider, "provider", "", "LLM provider")
	cmd.PersistentFlags().StringVar(&flags.Model, "model", "", "LLM model")
	cmd.PersistentFlags().StringVar(&flags.OllamaURL, "ollama-url", "", "Ollama base URL")
	cmd.PersistentFlags().StringVar(&flags.SQLitePath, "sqlite-path", "", "SQLite memory path")
	cmd.PersistentFlags().IntVar(&flags.ContextTokens, "context-tokens", 0, "Context token budget")
	cmd.PersistentFlags().IntVar(&flags.MaxTokens, "max-tokens", 0, "Maximum generated tokens")
	cmd.PersistentFlags().StringVar(&flags.ConfigPath, "config", "", "Path to config.toml")
}

func loadConfigFromCommand(cmd *cobra.Command, flags config.Flags) (config.Config, error) {
	if !cmd.Flags().Changed("provider") {
		flags.Provider = ""
	}
	if !cmd.Flags().Changed("model") {
		flags.Model = ""
	}
	if !cmd.Flags().Changed("ollama-url") {
		flags.OllamaURL = ""
	}
	if !cmd.Flags().Changed("sqlite-path") {
		flags.SQLitePath = ""
	}
	if !cmd.Flags().Changed("context-tokens") {
		flags.ContextTokens = 0
	}
	if !cmd.Flags().Changed("max-tokens") {
		flags.MaxTokens = 0
	}
	if !cmd.Flags().Changed("config") {
		flags.ConfigPath = ""
	}
	return config.Load(flags)
}

func runInteractive(ctx context.Context, cfg config.Config) error {
	ui := plain.NewRenderer()
	if ok, err := ensureTrustedFolder(ui); err != nil || !ok {
		return err
	}

	memory := sqlite.NewMemoryStore(cfg.SQLitePath)
	llm := ollama.NewOllamaLLM(cfg.OllamaURL, cfg.Model, ollama.WithMemoryStore(memory))
	executor := localexec.NewExecutor()

	orch := orchestrator.New(llm, memory, ui, executor, cfg)
	return orch.Start(ctx)
}

func ensureTrustedFolder(ui *plain.Renderer) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	store, err := trust.NewStore()
	if err != nil {
		return false, err
	}

	trusted, err := store.IsTrusted(cwd)
	if err != nil {
		return false, err
	}
	if trusted {
		return true, nil
	}

	approved := ui.AskApproval(event.New(
		"workspace.trust.requested",
		"Trust this folder?",
		fmt.Sprintf("Tessera will be allowed to create local session data and operate in:\n%s", cwd),
		map[string]any{"cwd": cwd, "trust_store": store.Path()},
	))
	if !approved {
		ui.RenderEvent(event.New("workspace.trust.denied", "Folder not trusted", "Session cancelled.", map[string]any{"cwd": cwd}))
		return false, nil
	}

	if err := store.Trust(cwd); err != nil {
		return false, err
	}
	ui.RenderEvent(event.New("workspace.trust.saved", "Folder trusted", cwd, map[string]any{"cwd": cwd, "trust_store": store.Path()}))
	return true, nil
}

func runDoctor(cfg config.Config) error {
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
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := ollama.Check(ctx, cfg.OllamaURL, nil); err != nil {
			fmt.Printf("Ollama: error (%s)\n", err)
		} else {
			fmt.Println("Ollama: connected")
		}
	}
	fmt.Printf("Memory:         local (%s)\n", cfg.SQLitePath)
	fmt.Printf("Context:        bounded (%d tokens)\n", cfg.ContextTokens)
	fmt.Printf("Max tokens:     %d\n", cfg.MaxTokens)
	fmt.Printf("Tessera dir:    %s\n", cfg.TesseraDir)
	fmt.Printf("Go heap alloc:  %d bytes\n", stats.Alloc)
	return nil
}

func stubCommand(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s is not implemented yet.\n", name)
		},
	}
}
