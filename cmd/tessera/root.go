package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alef-mach/tessera/internal/config"
)

type configuredRunner func(context.Context, config.Config) error
type configuredArgsRunner func(context.Context, config.Config, []string) error

func newRootCommand() *cobra.Command {
	var flags config.Flags

	root := &cobra.Command{
		Use:   "tessera",
		Short: "Local-first bounded-context coding agent",
		RunE:  runWithConfig(&flags, runInteractive),
	}

	bindConfigFlags(root, &flags)

	root.AddCommand(
		runCommand(&flags),
		configuredCommand("doctor", "Show local Tessera environment diagnostics", &flags, runDoctor),
		configuredCommand("index", "Index project files", &flags, runIndex),
		configuredCommand("config", "Show effective Tessera config", &flags, runConfig),
		configuredCommand("memory", "Inspect Tessera memory", &flags, runMemory),
		replayCommand(&flags),
		rollbackCommand(&flags),
		gitCommand(),
	)

	return root
}

func runCommand(flags *config.Flags) *cobra.Command {
	return &cobra.Command{
		Use:   "run [task]",
		Short: "Run one Tessera task, or start an interactive session when no task is given",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFromCommand(cmd, *flags)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return runInteractive(cmd.Context(), cfg)
			}
			return runTask(cmd.Context(), cfg, strings.Join(args, " "))
		},
	}
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
		Args:  cobra.NoArgs,
		RunE:  runWithConfig(flags, run),
	}
}

func configuredArgsCommand(
	use string,
	short string,
	flags *config.Flags,
	run configuredArgsRunner,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFromCommand(cmd, *flags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cfg, args)
		},
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
