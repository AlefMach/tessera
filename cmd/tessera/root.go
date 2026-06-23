package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/alef-mach/tessera/internal/config"
)

type configuredRunner func(context.Context, config.Config) error

func newRootCommand() *cobra.Command {
	var flags config.Flags

	root := &cobra.Command{
		Use:   "tessera",
		Short: "Local-first bounded-context coding agent",
		RunE:  runWithConfig(&flags, runInteractive),
	}

	bindConfigFlags(root, &flags)

	root.AddCommand(
		configuredCommand("run", "Start an interactive Tessera session", &flags, runInteractive),
		configuredCommand("doctor", "Show local Tessera environment diagnostics", &flags, runDoctor),
		configuredCommand("index", "Index project files", &flags, runIndex),

		stubCommand("config", "Inspect or update Tessera config"),
		stubCommand("memory", "Inspect Tessera memory"),
		stubCommand("replay", "Replay a previous Tessera session"),
	)

	return root
}

func configuredCommand(
	use string,
	short string,
	flags *config.Flags,
	run configuredRunner,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE:  runWithConfig(flags, run),
	}
}

func runWithConfig(
	flags *config.Flags,
	run configuredRunner,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigFromCommand(cmd, *flags)
		if err != nil {
			return err
		}

		return run(cmd.Context(), cfg)
	}
}