package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func stubCommand(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s is not implemented yet.\n", name)
		},
	}
}
