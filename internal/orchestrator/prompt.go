package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/port"
)

const tesseraSystemPrompt = `You are Tessera, a local-first interactive coding agent.

Work in small, reviewable steps. Use the provided project context, git state, memory, and repo map.
Do not claim you changed files unless an approved tool action actually changed them.
Prefer narrow test commands before broad suites.
Never ask to commit, push, discard changes, rewrite git history, or install global tools unless the user explicitly asks.

Respond with exactly one JSON object, like this:
{
  "action": "answer" | "run_command" | "propose_patch" | "suggest_commit_message",
  "message": "short explanation for the user",
  "command": {"name": "which language", "args": "which command", "reason": "why this command is useful"},
  "patch": "unified diff when action is propose_patch",
  "files": "which files are affected by the patch",
}

Use "answer" when more inspection or a human decision is needed. Use "run_command" only for local project commands that help inspect or verify. Use "propose_patch" when you can produce a small unified diff.`

func (o *Orchestrator) buildPrompt(ctx context.Context, input string) string {
	profile, err := o.memory.GetProjectProfile(ctx, o.session.ID)
	if err != nil {
		profile = o.profileProject(ctx)
	}

	var b strings.Builder
	b.WriteString("# User task\n")
	b.WriteString(input)
	b.WriteString("\n\n# Project profile\n")
	fmt.Fprintf(&b, "- root: %s\n", profile.Root)
	fmt.Fprintf(&b, "- mode: %s\n", profile.Mode)
	fmt.Fprintf(&b, "- stack: %s\n", profile.Stack)
	fmt.Fprintf(&b, "- manifests: %s\n", strings.Join(profile.Manifests, ", "))
	fmt.Fprintf(&b, "- has_git: %t\n", profile.HasGit)
	fmt.Fprintf(&b, "- has_tests: %t\n", profile.HasTests)
	fmt.Fprintf(&b, "- test_paths: %s\n", strings.Join(limitStrings(profile.TestPaths, 12), ", "))
	fmt.Fprintf(&b, "- suggested_test_command: %s\n", profile.TestRunner)

	if profile.HasGit {
		b.WriteString("\n# Git status\n")
		b.WriteString(o.gitStatus(ctx))
		b.WriteString("\n")
	}

	if memoryText := o.recentMemory(ctx, 8); memoryText != "" {
		b.WriteString("\n# Recent local memory\n")
		b.WriteString(memoryText)
		b.WriteString("\n")
	}

	if repoMap := o.repoMap(ctx, 60); repoMap != "" {
		b.WriteString("\n# Repo map\n")
		b.WriteString(repoMap)
		b.WriteString("\n")
	}

	b.WriteString("\n# Constraints\n")
	b.WriteString("- Keep the next step small.\n")
	b.WriteString("- If proposing a file change, return a unified diff and do not assume it was applied.\n")
	b.WriteString("- If running a command, return a JSON command object; Tessera will ask the user for approval.\n")
	b.WriteString("- If existing user changes are present, mention that before proposing edits.\n")

	return truncateMiddle(b.String(), o.promptCharBudget())
}

func (o *Orchestrator) gitStatus(ctx context.Context) string {
	if o.executor == nil {
		return "unavailable: no executor configured"
	}
	out, err := o.executor.Run(ctx, port.Command{
		Name:    "git",
		Args:    []string{"status", "--short", "--branch"},
		Dir:     o.session.CWD,
		Timeout: 5 * time.Second,
	})
	text := strings.TrimSpace(strings.Join([]string{out.Stdout, out.Stderr}, "\n"))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "unavailable: " + oneLine(text)
	}
	if text == "" {
		return "clean"
	}
	return text
}

func (o *Orchestrator) recentMemory(ctx context.Context, limit int) string {
	observations, err := o.memory.ListObservations(ctx, o.session.ID)
	if err != nil {
		return ""
	}
	var b strings.Builder
	count := 0
	for _, observation := range observations {
		if observation.Kind == "event" || strings.HasPrefix(observation.Kind, "llm.") {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n", observation.Kind, oneLine(observation.Content))
		count++
		if count >= limit {
			break
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (o *Orchestrator) repoMap(ctx context.Context, limit int) string {
	symbols, err := o.memory.ListSymbols(ctx, o.session.ID)
	if err != nil || len(symbols) == 0 {
		return ""
	}
	var b strings.Builder
	for i, symbol := range symbols {
		if i >= limit {
			fmt.Fprintf(&b, "... %d more symbols omitted\n", len(symbols)-limit)
			break
		}
		fmt.Fprintf(&b, "- %s %s %s:%d-%d\n", symbol.Kind, symbol.Name, symbol.Path, symbol.StartLine, symbol.EndLine)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (o *Orchestrator) promptCharBudget() int {
	if o.config.ContextTokens <= 0 {
		return 12000
	}
	// A conservative approximation keeps local-model prompts bounded without
	// depending on provider-specific tokenizers.
	return max(4000, o.config.ContextTokens*3)
}

func truncateMiddle(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	half := (limit - 64) / 2
	if half <= 0 {
		return value[:limit]
	}
	return value[:half] + "\n... context truncated ...\n" + value[len(value)-half:]
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	out := append([]string{}, values[:limit]...)
	out = append(out, fmt.Sprintf("... %d more", len(values)-limit))
	return out
}
