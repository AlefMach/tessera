package main

import (
	"github.com/spf13/cobra"

	"github.com/alef-mach/tessera/internal/config"
)

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
	flags = clearUnchangedFlags(cmd, flags)
	return config.Load(flags)
}

func clearUnchangedFlags(cmd *cobra.Command, flags config.Flags) config.Flags {
	if !flagChanged(cmd, "provider") {
		flags.Provider = ""
	}
	if !flagChanged(cmd, "model") {
		flags.Model = ""
	}
	if !flagChanged(cmd, "ollama-url") {
		flags.OllamaURL = ""
	}
	if !flagChanged(cmd, "sqlite-path") {
		flags.SQLitePath = ""
	}
	if !flagChanged(cmd, "context-tokens") {
		flags.ContextTokens = 0
	}
	if !flagChanged(cmd, "max-tokens") {
		flags.MaxTokens = 0
	}
	if !flagChanged(cmd, "config") {
		flags.ConfigPath = ""
	}

	return flags
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	return flag != nil && flag.Changed
}