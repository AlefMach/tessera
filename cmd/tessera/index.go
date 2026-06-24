package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/session"
	"github.com/alef-mach/tessera/internal/treesitter"
	"github.com/alef-mach/tessera/internal/ui/plain"
)

func runIndex(ctx context.Context, cfg config.Config) error {
	ui := plain.NewRenderer()

	ok, err := ensureTrustedFolder(ui)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	memory := sqlite.NewMemoryStore(cfg.SQLitePath)
	if err := memory.Ensure(ctx); err != nil {
		return err
	}

	manager := session.NewManager(memory)

	sess, err := manager.Start(ctx, cfg.Provider, cfg.Model)
	if err != nil {
		return err
	}

	result, err := treesitter.New(sess.CWD, memory).Index(ctx, sess.ID)
	if err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stdout,
		"Indexed %d files and %d symbols.\n",
		result.Files,
		result.Symbols,
	)

	if result.RepoMap != "" {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, result.RepoMap)
	}

	return nil
}
