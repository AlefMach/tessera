package orchestrator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/alef-mach/tessera/internal/llm"
	"github.com/alef-mach/tessera/internal/memory"
)

// TaskIntent is the structured result of classifying the user's task intent.
type TaskIntent struct {
	RequiresCodeChange bool     `json:"requires_code_change"`
	TaskType           string   `json:"task_type"` // refactor, create, fix, explain, question, run_tests, other
	TargetFiles        []string `json:"target_files,omitempty"`
}

var intentJSONSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "requires_code_change": {"type": "boolean"},
    "task_type": {"type": "string", "enum": ["refactor","create","fix","explain","question","run_tests","other"]},
    "target_files": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["requires_code_change", "task_type"]
}`)

const intentSystemPrompt = `You are a task classifier for a coding agent. Given a user task in any language, return JSON with:
- requires_code_change: true if the task asks to edit, refactor, fix, create, write, rename, or delete code; false for questions or explanations
- task_type: one of refactor, create, fix, explain, question, run_tests, other
- target_files: list of file paths explicitly mentioned in the task (optional)
Respond with ONLY valid JSON matching the required schema.`

// classifyTaskIntent asks the LLM to classify the user's task deterministically using a JSON schema.
// On any failure, defaults to RequiresCodeChange: true to err on the safe side.
func (o *Orchestrator) classifyTaskIntent(ctx context.Context, run *memory.Run, input string) TaskIntent {
	if o.llm == nil {
		return TaskIntent{RequiresCodeChange: true}
	}

	if run != nil {
		run.Calls++
	}

	resp, err := o.llm.Generate(ctx, llm.GenerateRequest{
		System:     intentSystemPrompt,
		Prompt:     "Classify this task: " + input,
		MaxTokens:  60,
		JSONSchema: intentJSONSchema,
		SessionID:  o.session.ID,
		RunID:      runID(run),
	})
	if err != nil {
		return TaskIntent{RequiresCodeChange: true}
	}

	raw := strings.TrimSpace(resp.Text)
	raw = extractJSON(raw)
	var intent TaskIntent
	if err := json.Unmarshal([]byte(raw), &intent); err != nil {
		return TaskIntent{RequiresCodeChange: true}
	}
	return intent
}

func blockerOnlyMentionsMissingTests(reason string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(reason), " "))
	hasTestWord := strings.Contains(lower, "test") ||
		strings.Contains(lower, "unit") ||
		strings.Contains(lower, "teste") ||
		strings.Contains(lower, "_test") ||
		strings.Contains(lower, "spec")
	hasMissingWord := strings.Contains(lower, "missing") ||
		strings.Contains(lower, "no ") ||
		strings.Contains(lower, "without") ||
		strings.Contains(lower, "provide") ||
		strings.Contains(lower, "requires") ||
		strings.Contains(lower, "required") ||
		strings.Contains(lower, "existing test structure") ||
		strings.Contains(lower, "before proceeding")
	return hasTestWord && hasMissingWord
}
