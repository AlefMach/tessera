package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/port"
	"github.com/alef-mach/tessera/internal/project"
	"github.com/alef-mach/tessera/internal/treesitter"
)

func (o *Orchestrator) handleSlashCommand(ctx context.Context, input string) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return
	}

	switch fields[0] {
	case "/help":
		o.renderHelp(ctx)
	case "/status":
		o.renderStatus(ctx)
	case "/memory":
		o.renderMemory(ctx)
	case "/index":
		o.renderIndex(ctx)
	case "/git":
		o.renderGit(ctx)
	case "/diff":
		o.renderDiff(ctx)
	case "/rollback":
		o.renderRollback(ctx, fields[1:])
	case "/commit-message":
		o.renderCommitMessage(ctx)
	case "/context":
		o.renderContext(ctx)
	case "/calls":
		o.renderCalls(ctx)
	case "/clear":
		o.renderClear(ctx)
	case "/approve", "/deny":
		o.emit(ctx, event.New("approval.inactive", "No active approval", "Use y/n/d when Tessera is asking for approval. Slash approval shortcuts are reserved for a future non-blocking approval flow.", nil))
	default:
		o.emit(ctx, event.New("command.unknown", "Unknown command", fmt.Sprintf("%s is not a Tessera command. Type /help to see available commands.", fields[0]), map[string]any{"command": fields[0]}))
	}
}

func (o *Orchestrator) renderHelp(ctx context.Context) {
	o.emit(ctx, event.New("help", "Commands", strings.TrimSpace(`
/help              Show available commands
/status            Show active session status
/diff              Show current working-tree diff
/git               Show Git status
/rollback [run-id] Reverse the latest Tessera-applied patch when metadata is available
/commit-message    Suggest a commit message for the current diff
/context           Show project context Tessera is using
/calls             Show recent LLM calls
/memory            Show saved observations
/index             Re-index project files
/clear             Start a fresh visual section without clearing terminal scrollback
/exit              End the session

During approval prompts, use: y yes, n no, d diff.
`), nil))
}

func (o *Orchestrator) renderStatus(ctx context.Context) {
	stats, err := o.memory.Stats(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Status unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	profile, err := o.memory.GetProjectProfile(ctx, o.session.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			profile = o.profileProject(ctx)
		} else {
			o.emit(ctx, event.New("error.occurred", "Profile unavailable", err.Error(), map[string]any{"error": err.Error()}))
			return
		}
	}
	o.emit(ctx, event.New("status", "Status", "", map[string]any{
		"session":      stats.SessionID,
		"provider":     stats.Provider,
		"model":        stats.Model,
		"mode":         profile.Mode,
		"stack":        profile.Stack,
		"git":          profile.HasGit,
		"tests":        profile.HasTests,
		"test_runner":  profile.TestRunner,
		"calls":        stats.Calls,
		"steps":        stats.Steps,
		"runs":         stats.Runs,
		"observations": stats.Observations,
	}))
}

func (o *Orchestrator) profileProject(ctx context.Context) project.ProjectProfile {
	profile, err := project.Profile(o.session.CWD)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Project profile unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return project.ProjectProfile{}
	}
	profile.SessionID = o.session.ID
	if err := o.memory.SaveProjectProfile(ctx, profile); err != nil {
		o.emit(ctx, event.New("error.occurred", "Project profile not saved", err.Error(), map[string]any{"error": err.Error()}))
		return profile
	}
	o.emit(ctx, event.New("project.profiled", "Project profiled", "", map[string]any{
		"root":        profile.Root,
		"mode":        profile.Mode,
		"stack":       profile.Stack,
		"manifests":   profile.Manifests,
		"git":         profile.HasGit,
		"tests":       profile.HasTests,
		"test_runner": profile.TestRunner,
	}))
	return profile
}

func (o *Orchestrator) renderMemory(ctx context.Context) {
	observations, err := o.memory.ListObservations(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Memory unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	var builder strings.Builder
	limit := min(len(observations), 12)
	for i := 0; i < limit; i++ {
		observation := observations[i]
		builder.WriteString("- ")
		builder.WriteString(observation.CreatedAt.Local().Format("15:04:05"))
		builder.WriteString(" ")
		builder.WriteString(observation.Kind)
		builder.WriteString(" ")
		builder.WriteString(oneLine(observation.Content))
		builder.WriteString("\n")
	}
	if limit == 0 {
		builder.WriteString("No saved observations yet.")
	}
	o.emit(ctx, event.New("memory", "Memory", strings.TrimRight(builder.String(), "\n"), map[string]any{
		"session":      o.session.ID,
		"observations": len(observations),
	}))
}

func (o *Orchestrator) indexProjectQuietly(ctx context.Context) {
	_, err := treesitter.New(o.session.CWD, o.memory).Index(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("index.skipped", "Index skipped", err.Error(), map[string]any{
			"root":  o.session.CWD,
			"error": err.Error(),
		}))
	}
}

func (o *Orchestrator) renderIndex(ctx context.Context) {
	o.emit(ctx, event.New("index.started", "Indexing with Tree-sitter", "", map[string]any{
		"root": o.session.CWD,
	}))
	result, err := treesitter.New(o.session.CWD, o.memory).Index(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Index failed", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	message := result.RepoMap
	if message == "" {
		message = "No indexable source files found."
	}
	o.emit(ctx, event.New("index.finished", "Index finished", message, map[string]any{
		"files":       result.Files,
		"symbols":     result.Symbols,
		"started_at":  result.StartedAt,
		"finished_at": result.FinishedAt,
	}))
}

func (o *Orchestrator) renderGit(ctx context.Context) {
	status := o.gitStatus(ctx)
	o.emit(ctx, event.New("git.status", "Git status", status, map[string]any{
		"root": o.session.CWD,
	}))
}

func (o *Orchestrator) renderDiff(ctx context.Context) {
	out, err := o.runGit(ctx, 10*time.Second, "diff", "--")
	message := commandOutput(out, err)
	if strings.TrimSpace(message) == "" {
		message = "No working-tree diff."
	}
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Diff unavailable", message, map[string]any{"error": err.Error()}))
		return
	}
	o.emit(ctx, event.New("diff", "Working-tree diff", message, map[string]any{
		"root": o.session.CWD,
	}))
}

func (o *Orchestrator) renderCommitMessage(ctx context.Context) {
	out, err := o.runGit(ctx, 10*time.Second, "diff", "--name-status", "--")
	message := commandOutput(out, err)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Commit message unavailable", message, map[string]any{"error": err.Error()}))
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		o.emit(ctx, event.New("commit.message", "Suggested commit", "No working-tree changes found.", nil))
		return
	}

	suggestion := suggestCommitMessage(message)
	o.emit(ctx, event.New("commit.message", "Suggested commit", suggestion, map[string]any{
		"commit_message": suggestion,
		"changed_files":  message,
	}))
}

func (o *Orchestrator) renderContext(ctx context.Context) {
	profile, err := o.memory.GetProjectProfile(ctx, o.session.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			o.emit(ctx, event.New("error.occurred", "Context unavailable", err.Error(), map[string]any{"error": err.Error()}))
			return
		}
		profile = o.profileProject(ctx)
	}
	stats, _ := o.memory.Stats(ctx, o.session.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "root: %s\n", profile.Root)
	fmt.Fprintf(&b, "mode: %s\n", profile.Mode)
	fmt.Fprintf(&b, "stack: %s\n", profile.Stack)
	fmt.Fprintf(&b, "manifests: %s\n", strings.Join(profile.Manifests, ", "))
	fmt.Fprintf(&b, "tests: %t\n", profile.HasTests)
	fmt.Fprintf(&b, "test runner: %s\n", profile.TestRunner)
	fmt.Fprintf(&b, "git: %t\n", profile.HasGit)
	fmt.Fprintf(&b, "context budget: %d tokens approx.\n", o.config.ContextTokens)
	fmt.Fprintf(&b, "prompt char budget: %d\n", o.promptCharBudget())
	if stats.SessionID != "" {
		fmt.Fprintf(&b, "calls: %d\n", stats.Calls)
		fmt.Fprintf(&b, "symbols: %d\n", stats.Symbols)
	}
	if repoMap := o.repoMap(ctx, 20); repoMap != "" {
		fmt.Fprintf(&b, "\nrepo map:\n%s\n", repoMap)
	}
	o.emit(ctx, event.New("context", "Context", strings.TrimRight(b.String(), "\n"), nil))
}

func (o *Orchestrator) renderCalls(ctx context.Context) {
	calls, err := o.memory.ListCalls(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Calls unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	if len(calls) == 0 {
		o.emit(ctx, event.New("calls", "LLM calls", "No LLM calls recorded yet.", nil))
		return
	}
	var b strings.Builder
	limit := min(len(calls), 8)
	for i := 0; i < limit; i++ {
		call := calls[i]
		fmt.Fprintf(&b, "- %s %s/%s in=%d out=%d duration=%dms",
			call.CreatedAt.Local().Format("15:04:05"), call.Provider, call.Model, call.InputTokens, call.OutputTokens, call.DurationMS)
		if call.Error != "" {
			fmt.Fprintf(&b, " error=%s", oneLine(call.Error))
		}
		if text := oneLine(call.Response); text != "" {
			fmt.Fprintf(&b, " response=%s", truncateMiddle(text, 180))
		}
		b.WriteString("\n")
	}
	o.emit(ctx, event.New("calls", "LLM calls", strings.TrimRight(b.String(), "\n"), map[string]any{"calls": len(calls)}))
}

func (o *Orchestrator) renderClear(ctx context.Context) {
	o.emit(ctx, event.New("clear", "Fresh section", "────────────────────────────────────────\nTerminal scrollback was preserved.", nil))
}

func (o *Orchestrator) renderRollback(ctx context.Context, args []string) {
	if o.executor == nil {
		o.emit(ctx, event.New("error.occurred", "Rollback unavailable", "No command executor is configured.", nil))
		return
	}
	runFilter := ""
	if len(args) > 0 {
		runFilter = strings.TrimSpace(args[0])
	}

	observations, err := o.memory.ListObservations(ctx, o.session.ID)
	if err != nil {
		o.emit(ctx, event.New("error.occurred", "Rollback unavailable", err.Error(), map[string]any{"error": err.Error()}))
		return
	}
	observation, patchPath, ok := latestPatchObservation(observations, runFilter)
	if !ok {
		message := "No Tessera-applied patch was found in this session."
		if runFilter != "" {
			message = "No Tessera-applied patch was found for run " + runFilter + "."
		}
		o.emit(ctx, event.New("rollback.unavailable", "Rollback unavailable", message, nil))
		return
	}

	patch := ""
	if data, err := os.ReadFile(patchPath); err == nil {
		patch = string(data)
	}
	if !o.ui.AskApproval(event.New("approval.requested", "Rollback patch?", "Apply the saved patch in reverse.", map[string]any{
		"run_id":     observation.RunID,
		"patch_file": patchPath,
		"diff":       patch,
		"risk":       "workspace file changes",
	})) {
		o.emit(ctx, event.New("rollback.denied", "Rollback denied", "No files were changed.", map[string]any{"run_id": observation.RunID}))
		return
	}

	out, err := o.executor.Run(ctx, port.Command{
		Name:    "git",
		Args:    []string{"apply", "-R", "--whitespace=nowarn", patchPath},
		Dir:     o.session.CWD,
		Timeout: 30 * time.Second,
	})
	output := commandOutput(out, err)
	if err != nil || out.ExitCode != 0 {
		o.emit(ctx, event.New("rollback.failed", "Rollback failed", output, map[string]any{
			"run_id":     observation.RunID,
			"patch_file": patchPath,
			"exit_code":  out.ExitCode,
		}))
		return
	}

	message := firstNonEmpty(output, "Rollback applied.")
	o.saveObservation(ctx, o.activeRun, "rollback.applied", message, map[string]any{
		"source_run_id": observation.RunID,
		"patch_file":    patchPath,
	})
	o.emit(ctx, event.New("rollback.applied", "Rollback applied", message, map[string]any{
		"run_id":     observation.RunID,
		"patch_file": patchPath,
	}))
}

func (o *Orchestrator) runGit(ctx context.Context, timeout time.Duration, args ...string) (port.Output, error) {
	if o.executor == nil {
		return port.Output{}, errors.New("no command executor is configured")
	}
	return o.executor.Run(ctx, port.Command{
		Name:    "git",
		Args:    args,
		Dir:     o.session.CWD,
		Timeout: timeout,
	})
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

func suggestCommitMessage(nameStatus string) string {
	entries := strings.Split(strings.TrimSpace(nameStatus), "\n")
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts := strings.Fields(entry)
		if len(parts) == 0 {
			continue
		}
		paths = append(paths, parts[len(parts)-1])
	}
	sort.Strings(paths)

	scope := commitScope(paths)
	kind := commitKind(paths)
	if scope == "" {
		return kind + ": update project files"
	}
	return fmt.Sprintf("%s(%s): update %s", kind, scope, scope)
}

func commitKind(paths []string) string {
	if len(paths) == 0 {
		return "chore"
	}
	allDocs := true
	anyTest := false
	for _, path := range paths {
		lower := strings.ToLower(path)
		if !strings.HasSuffix(lower, ".md") && !strings.HasPrefix(lower, "docs/") {
			allDocs = false
		}
		if strings.Contains(lower, "_test.") || strings.Contains(lower, "/test") || strings.Contains(lower, "/spec") {
			anyTest = true
		}
	}
	if allDocs {
		return "docs"
	}
	if anyTest {
		return "test"
	}
	return "chore"
}

func commitScope(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	first := filepath.ToSlash(paths[0])
	if strings.HasPrefix(first, "cmd/") {
		parts := strings.Split(first, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
		return "cli"
	}
	if strings.HasPrefix(first, "internal/") {
		parts := strings.Split(first, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
		return "internal"
	}
	if strings.HasSuffix(strings.ToLower(first), ".md") {
		return "docs"
	}
	base := strings.TrimSuffix(filepath.Base(first), filepath.Ext(first))
	if base == "" {
		return "project"
	}
	return base
}
