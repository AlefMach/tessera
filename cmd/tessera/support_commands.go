package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alef-mach/tessera/internal/adapter/executor/localexec"
	"github.com/alef-mach/tessera/internal/config"
	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/memory/sqlite"
	"github.com/alef-mach/tessera/internal/port"
	"github.com/alef-mach/tessera/internal/ui/plain"
)

func runConfig(ctx context.Context, cfg config.Config) error {
	fmt.Println("Tessera config")
	fmt.Printf("Provider:       %s\n", cfg.Provider)
	fmt.Printf("Model:          %s\n", cfg.Model)
	fmt.Printf("Ollama URL:     %s\n", cfg.OllamaURL)
	fmt.Printf("SQLite path:    %s\n", cfg.SQLitePath)
	fmt.Printf("Context tokens: %d\n", cfg.ContextTokens)
	fmt.Printf("Max tokens:     %d\n", cfg.MaxTokens)
	fmt.Printf("Config path:    %s\n", cfg.ConfigPath)
	fmt.Printf("Tessera dir:    %s\n", cfg.TesseraDir)
	return nil
}

func runMemory(ctx context.Context, cfg config.Config) error {
	store := sqlite.NewMemoryStore(cfg.SQLitePath)
	if err := store.Ensure(ctx); err != nil {
		return err
	}
	sess, err := store.GetSession(ctx, "")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Println("No Tessera sessions recorded yet.")
			return nil
		}
		return err
	}
	stats, err := store.Stats(ctx, sess.ID)
	if err != nil {
		return err
	}
	fmt.Println("Tessera memory")
	fmt.Printf("Session:       %s\n", sess.ID)
	fmt.Printf("Project:       %s\n", sess.CWD)
	fmt.Printf("Provider/model:%s/%s\n", stats.Provider, stats.Model)
	fmt.Printf("Runs:          %d\n", stats.Runs)
	fmt.Printf("Steps:         %d\n", stats.Steps)
	fmt.Printf("LLM calls:     %d\n", stats.Calls)
	fmt.Printf("Observations:  %d\n", stats.Observations)
	fmt.Printf("Files indexed: %d\n", stats.FileSummaries)
	fmt.Printf("Symbols:       %d\n", stats.Symbols)

	observations, err := store.ListObservations(ctx, sess.ID)
	if err != nil {
		return err
	}
	limit := min(len(observations), 10)
	if limit == 0 {
		return nil
	}
	fmt.Println("\nRecent observations:")
	for i := 0; i < limit; i++ {
		observation := observations[i]
		fmt.Printf("- %s %s %s\n", observation.CreatedAt.Local().Format("15:04:05"), observation.Kind, oneLine(observation.Content))
	}
	return nil
}

func replayCommand(flags *config.Flags) *cobra.Command {
	cmd := configuredArgsCommand("replay <run-id>", "Replay a previous Tessera run from local memory", flags, runReplay)
	cmd.Args = cobra.ExactArgs(1)
	return cmd
}

func runReplay(ctx context.Context, cfg config.Config, args []string) error {
	runID := args[0]
	store := sqlite.NewMemoryStore(cfg.SQLitePath)
	if err := store.Ensure(ctx); err != nil {
		return err
	}
	run, err := store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	sess, err := store.GetSession(ctx, run.SessionID)
	if err != nil {
		return err
	}
	observations, err := store.ListObservations(ctx, run.SessionID)
	if err != nil {
		return err
	}
	fmt.Printf("Tessera replay %s\n", run.ID)
	fmt.Printf("Project: %s\n", sess.CWD)
	fmt.Printf("Input:   %s\n", run.Input)
	fmt.Printf("Status:  %s\n", run.Status)
	fmt.Printf("Steps:   %d\n", run.Steps)
	fmt.Printf("Calls:   %d\n", run.Calls)
	fmt.Println()
	for i := len(observations) - 1; i >= 0; i-- {
		observation := observations[i]
		if observation.RunID != run.ID {
			continue
		}
		fmt.Printf("[%s] %s\n", observation.CreatedAt.Local().Format("15:04:05"), observation.Kind)
		content := strings.TrimSpace(observation.Content)
		if content != "" {
			fmt.Println(indent(content, "  "))
		}
	}
	return nil
}

func gitCommand() *cobra.Command {
	git := &cobra.Command{
		Use:   "git",
		Short: "Inspect Git state",
	}
	git.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show git status for the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGitStatus(cmd.Context())
		},
	})
	return git
}

func runGitStatus(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	exec := localexec.NewExecutor()
	out, err := exec.Run(ctx, port.Command{
		Name:    "git",
		Args:    []string{"status", "--short", "--branch"},
		Dir:     cwd,
		Timeout: 5 * time.Second,
	})
	text := commandOutput(out, err)
	if text != "" {
		fmt.Println(text)
	}
	return err
}

func rollbackCommand(flags *config.Flags) *cobra.Command {
	cmd := configuredArgsCommand("rollback [run-id]", "Reverse the latest Tessera-applied patch", flags, runRollback)
	cmd.Args = cobra.MaximumNArgs(1)
	return cmd
}

func runRollback(ctx context.Context, cfg config.Config, args []string) error {
	ui := plain.NewRenderer()
	ok, err := ensureTrustedFolder(ui)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	store := sqlite.NewMemoryStore(cfg.SQLitePath)
	if err := store.Ensure(ctx); err != nil {
		return err
	}

	runFilter := ""
	var sessID string
	var originalCWD string
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		runFilter = strings.TrimSpace(args[0])
		run, err := store.GetRun(ctx, runFilter)
		if err != nil {
			return err
		}
		sess, err := store.GetSession(ctx, run.SessionID)
		if err != nil {
			return err
		}
		sessID = sess.ID
		originalCWD = sess.CWD
	} else {
		sess, err := store.GetSession(ctx, "")
		if err != nil {
			return err
		}
		sessID = sess.ID
		originalCWD = sess.CWD
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if filepath.Clean(cwd) != filepath.Clean(originalCWD) {
		return fmt.Errorf("rollback must be run from the original workspace: %s", originalCWD)
	}

	observations, err := store.ListObservations(ctx, sessID)
	if err != nil {
		return err
	}
	observation, patchPath, ok := latestPatchObservation(observations, runFilter)
	if !ok {
		if runFilter != "" {
			return fmt.Errorf("no Tessera-applied patch found for run %s", runFilter)
		}
		return errors.New("no Tessera-applied patch found in latest session")
	}
	patch := ""
	if data, err := os.ReadFile(patchPath); err == nil {
		patch = string(data)
	}
	if !ui.AskApproval(event.New("approval.requested", "Rollback patch?", "Apply the saved patch in reverse.", map[string]any{
		"run_id":     observation.RunID,
		"patch_file": patchPath,
		"diff":       patch,
		"risk":       "workspace file changes",
	})) {
		fmt.Println("Rollback denied. No files were changed.")
		return nil
	}

	exec := localexec.NewExecutor()
	out, err := exec.Run(ctx, port.Command{
		Name:    "git",
		Args:    []string{"apply", "-R", "--whitespace=nowarn", patchPath},
		Dir:     cwd,
		Timeout: 30 * time.Second,
	})
	output := commandOutput(out, err)
	if output != "" {
		fmt.Println(output)
	}
	if err != nil || out.ExitCode != 0 {
		return fmt.Errorf("rollback failed")
	}
	fmt.Println("Rollback applied.")
	return nil
}

func latestPatchObservation(observations []memory.Observation, runFilter string) (memory.Observation, string, bool) {
	for _, observation := range observations {
		if observation.Kind != "patch.applied" {
			continue
		}
		if runFilter != "" && observation.RunID != runFilter {
			continue
		}
		patchPath := dataStringValue(observation.Data, "patch_file")
		if patchPath == "" {
			continue
		}
		return observation, patchPath, true
	}
	return memory.Observation{}, "", false
}

func dataStringValue(data map[string]any, key string) string {
	if len(data) == 0 {
		return ""
	}
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func commandOutput(out port.Output, err error) string {
	text := strings.TrimSpace(strings.Join([]string{out.Stdout, out.Stderr}, "\n"))
	if text == "" && err != nil {
		text = err.Error()
	}
	return text
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func indent(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
