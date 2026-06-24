package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/port"
)

const tesseraSystemPrompt = `You are Tessera, a local-first interactive coding agent.

Your job is to help the user with coding tasks by taking ONE small action at a time.
You MUST respond with ONLY valid JSON — no markdown, no explanation outside the JSON.

Available action types:
- "inspect": read specific files to understand the codebase before making changes
- "patch": apply a unified diff to create or modify files
- "write": write the complete final content of one workspace file
- "run": execute a local verification command (tests, build, etc.)
- "finish": mark the task as complete with a summary
- "blocker": report when something prevents safe progress

Rules:
- Always inspect relevant files BEFORE writing patches — do not guess file content
- When creating a new file, use patch with a unified diff (--- /dev/null header)
- If generating a correct unified diff is difficult, use "write" with the complete final file content instead
- After a run succeeds (exit_code 0), prefer "finish" unless there is still failing work
- Never commit, push, rewrite git history, or install global tools unless explicitly asked
- Do not claim changes were made unless a patch or write action was actually approved and applied
- Keep patches small and focused — one logical change at a time
- If you do not have enough context, use "inspect" first

JSON shape (use exactly one):
{"type":"inspect","reason":"...","files":["path/to/file"]}
{"type":"patch","reason":"...","patch":"--- a/file\n+++ b/file\n@@ ... @@\n..."}
{"type":"write","reason":"...","path":"path/to/file","content":"complete final file content\n"}
{"type":"run","reason":"...","command":"go test ./..."}
{"type":"finish","summary":"..."}
{"type":"blocker","reason":"..."}`

func (o *Orchestrator) buildPrompt(ctx context.Context, task string, previousResult string) string {
	profile, err := o.memory.GetProjectProfile(ctx, o.session.ID)
	if err != nil {
		profile = o.profileProject(ctx)
	}

	var b strings.Builder

	b.WriteString("# User task\n")
	b.WriteString(task)
	b.WriteString("\n")

	if strings.TrimSpace(previousResult) != "" {
		b.WriteString("\n# Result of previous action\n")
		b.WriteString(previousResult)
		b.WriteString("\n")
	}

	b.WriteString("\n# Project profile\n")
	fmt.Fprintf(&b, "- root: %s\n", profile.Root)
	fmt.Fprintf(&b, "- stack: %s\n", profile.Stack)
	fmt.Fprintf(&b, "- mode: %s\n", profile.Mode)
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
		b.WriteString("\n# Recent actions\n")
		b.WriteString(memoryText)
		b.WriteString("\n")
	}

	// Inline relevant file contents so a small local model can read and write code
	if fileContext := o.relevantFileContext(ctx, task, previousResult); fileContext != "" {
		b.WriteString("\n# Relevant file contents\n")
		b.WriteString(fileContext)
		b.WriteString("\n")
	} else if repoMap := o.repoMap(ctx, 60); repoMap != "" {
		// Fall back to repo map when no files can be inlined
		b.WriteString("\n# Repo map\n")
		b.WriteString(repoMap)
		b.WriteString("\n")
	}

	b.WriteString("\n# Constraints\n")
	b.WriteString("- Respond with ONLY valid JSON, no other text.\n")
	b.WriteString("- Choose exactly ONE action type: inspect, patch, write, run, finish, or blocker.\n")
	b.WriteString("- Use inspect when you need to read a file before editing it.\n")
	b.WriteString("- Use patch to create or modify files (unified diff format).\n")
	b.WriteString("- Use write when you know the complete final content for one file and a unified diff is likely to fail.\n")
	b.WriteString("- Use run for tests or build commands — Tessera will ask for approval first.\n")
	b.WriteString("- Use finish when the task is complete.\n")
	b.WriteString("- Use blocker when you cannot proceed safely.\n")
	b.WriteString("\n# Example responses\n")
	b.WriteString(`{"type":"inspect","reason":"Need to see the existing test structure before writing a new test.","files":["internal/sum/sum_test.go","internal/sum/sum.go"]}`)
	b.WriteString("\n")
	b.WriteString(`{"type":"patch","reason":"Create the first unit test for sum.go.","patch":"--- /dev/null\n+++ b/internal/sum/sum_test.go\n@@ -0,0 +1,12 @@\n+package sum_test\n+\n+import (\n+\t\"testing\"\n+\t\"github.com/example/project/internal/sum\"\n+)\n+\n+func TestAdd(t *testing.T) {\n+\tif got := sum.Add(1, 2); got != 3 {\n+\t\tt.Errorf(\"Add(1,2) = %d, want 3\", got)\n+\t}\n+}"}`)
	b.WriteString("\n")
	b.WriteString(`{"type":"write","reason":"Replace the file with the corrected implementation.","path":"internal/sum/sum.go","content":"package sum\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"}`)
	b.WriteString("\n")
	b.WriteString(`{"type":"run","reason":"Run the tests to verify the new test file passes.","command":"go test ./..."}`)
	b.WriteString("\n")

	return truncateMiddle(b.String(), o.promptCharBudget())
}

// relevantFileContext reads and inlines the content of files most relevant to the current task.
// It scores files based on keyword overlap with the task and previous result, then inlines the
// top candidates so a small local model has actual code to read and edit.
func (o *Orchestrator) relevantFileContext(ctx context.Context, task, previousResult string) string {
	summaries, err := o.memory.ListFileSummaries(ctx, o.session.ID)
	if err != nil || len(summaries) == 0 {
		return ""
	}

	combined := strings.ToLower(task + " " + previousResult)
	taskWords := tokenize(combined)

	type scored struct {
		path  string
		score int
	}
	var candidates []scored
	for _, s := range summaries {
		score := scoreFile(s.Path, taskWords)
		if score > 0 {
			candidates = append(candidates, scored{path: s.Path, score: score})
		}
	}

	// If no matches by keywords, fall back to test files and entry points for new-project tasks
	if len(candidates) == 0 {
		for _, s := range summaries {
			base := strings.ToLower(filepath.Base(s.Path))
			if strings.Contains(base, "_test") || strings.Contains(s.Path, "/test") {
				candidates = append(candidates, scored{path: s.Path, score: 1})
			}
		}
		// Also add small source files that look like entry points
		for _, s := range summaries {
			base := strings.ToLower(filepath.Base(s.Path))
			if base == "main.go" || base == "main.ts" || base == "main.py" ||
				base == "index.ts" || base == "index.js" || base == "app.py" {
				candidates = append(candidates, scored{path: s.Path, score: 2})
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	budget := o.promptCharBudget() / 3 // use at most 1/3 of the char budget for file contents
	var b strings.Builder
	seen := map[string]bool{}

	for _, c := range candidates {
		if seen[c.path] {
			continue
		}
		content, err := o.readWorkspaceFile(c.path)
		if err != nil {
			continue
		}
		block := fmt.Sprintf("## %s\n%s\n\n", c.path, content)
		if b.Len()+len(block) > budget {
			break
		}
		b.WriteString(block)
		seen[c.path] = true
	}

	return strings.TrimSpace(b.String())
}

// scoreFile returns a relevance score for a file path given a set of task keywords.
func scoreFile(path string, taskWords map[string]bool) int {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	parts := strings.FieldsFunc(lowerPath, func(r rune) bool {
		return r == '/' || r == '_' || r == '-' || r == '.' || r == ' '
	})
	score := 0
	for _, part := range parts {
		if taskWords[part] {
			score++
		}
	}
	// boost test files when task mentions test/testing
	if (taskWords["test"] || taskWords["tests"] || taskWords["testing"]) &&
		(strings.Contains(lowerPath, "_test") || strings.Contains(lowerPath, "/test")) {
		score += 2
	}
	return score
}

// tokenize splits text into a set of unique lowercase words (>2 chars).
func tokenize(text string) map[string]bool {
	words := map[string]bool{}
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	}) {
		if len(word) > 2 {
			words[word] = true
		}
	}
	return words
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
	return max(4000, o.config.ContextTokens*3)
}

func appendInvalidActionRepairPrompt(prompt string, previousResponse string, previousErr error) string {
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n# Previous response was invalid\n")
	b.WriteString("Your previous response was not valid Tessera action JSON. Return ONLY valid JSON now.\n")
	if previousErr != nil {
		b.WriteString("Error: ")
		b.WriteString(previousErr.Error())
		b.WriteString("\n")
	}
	if strings.TrimSpace(previousResponse) != "" {
		b.WriteString("Previous response:\n")
		b.WriteString(previousResponse)
		b.WriteString("\n")
	}
	b.WriteString(`Expected shape: {"type":"inspect|patch|write|run|finish|blocker","reason":"...","files":[...],"path":"...","content":"...","patch":"...","command":"...","summary":"..."}`)
	b.WriteString("\n")
	return b.String()
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
