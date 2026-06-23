package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/project"
	"github.com/alef-mach/tessera/internal/treesitter"
)

func (o *Orchestrator) renderHelp(ctx context.Context) {
	o.emit(ctx, event.New("help", "Commands", "/help    Show available commands\n/status  Show active session status\n/memory  Show saved memory\n/index   Index project files\n/exit    End the session", nil))
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
