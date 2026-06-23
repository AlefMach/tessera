# Tessera

> Local-first interactive coding agent for small VRAM machines.

Tessera is an experimental CLI coding agent designed to run with **local LLMs** on limited hardware. It is built around an interactive terminal session, a strict context budget, external memory, and many small structured LLM calls.

The goal is simple:

> Get a Codex-like local workflow without depending on giant context windows.

Instead of trying to fit an entire repository into one prompt, Tessera breaks the project into small pieces, indexes them, stores structured summaries, rereads real files before editing, asks for approval, applies patches, and verifies changes through tests.

---

## Why Tessera?

A **tessera** is a small tile used to build a mosaic.

That is the core idea behind this project:

```text
large codebase
  ↓
split into small pieces
  ↓
analyze each piece with a local LLM
  ↓
store structured summaries externally
  ↓
reconstruct enough context to act safely
```

Tessera does not try to put the whole repository into a single prompt. It builds understanding incrementally.

---

## Project status

Tessera is currently in the planning / early implementation stage.

The first milestone is not a fully autonomous coding agent. The first milestone is a solid interactive CLI foundation:

```bash
tessera
```

That command should open a persistent session where the user can:

- type natural-language coding tasks;
- see what the agent is doing;
- review commands before execution;
- approve or deny risky actions;
- inspect diffs;
- copy code and terminal output easily;
- keep a local memory of runs, prompts, responses, patches, logs, and observations.

---

## Main experience

The main product experience is:

```bash
tessera
```

This opens an interactive session:

```text
╭────────────────────────────────────────────────────────────╮
│ Tessera  ·  local agent  ·  model: qwen2.5-coder:7b        │
│ cwd: ~/projects/my-app  ·  context: 4096  ·  calls: 0/100  │
╰────────────────────────────────────────────────────────────╯

Type your task or /help.

› create a unit test for the user service
```

Tessera then shows progress directly in the terminal:

```text
● Profiling project
  stack: Go
  files: 18
  tests: found

● Indexing with Tree-sitter
  indexed: 18 files
  symbols: 74

● Selecting context
  selected:
    - internal/user/service.go
    - internal/user/service_test.go

● Planning patch
  calls: 12/100
```

When an action needs approval:

```text
╭─ Approval required ─────────────────────────────╮
│ Tessera wants to modify 2 files:                │
│                                                │
│  M internal/user/service.go                    │
│  M internal/user/service_test.go               │
│                                                │
│ Reason: add failing test and implementation    │
╰────────────────────────────────────────────────╯

Approve? [y] yes  [n] no  [d] show diff
```

When the task finishes:

```text
✓ Task completed

Test command:
  go test ./...

Result:
  ok ./internal/user 0.004s

Changed files:
  M internal/user/service.go
  M internal/user/service_test.go

Run saved:
  .tessera/runs/run-001
```

---

## Design philosophy

### Interactive first

Tessera is primarily an interactive CLI session.

```bash
tessera
```

One-shot commands exist, but they are secondary:

```bash
tessera run "create the first unit test"
tessera doctor
tessera index
tessera replay <run-id>
```

Use cases:

```text
tessera          → main interactive experience
tessera run      → automation, scripts, CI, one-shot tasks
tessera doctor   → environment diagnostics
tessera index    → manual indexing
tessera replay   → inspect previous runs
```

---

### Copy-friendly terminal UI

The default UI should not be a full-screen TUI.

The default mode should be a beautiful but simple terminal transcript:

- no screen clearing;
- no broken scrollback;
- easy text selection;
- easy code copying;
- readable command output;
- readable diffs;
- works in most terminals.

A full-screen TUI may exist later:

```bash
tessera --tui
```

But the default should remain copy-friendly.

---

### Bounded context

Tessera should never require a huge context window.

Limits are explicit:

```bash
TESSERA_MAX_CONTEXT=4096
TESSERA_MAX_INPUT=3000
TESSERA_MAX_OUTPUT=800
```

If something does not fit, Tessera should split the work into smaller calls instead of blindly truncating context.

---

### Many small LLM calls

Tessera is designed to trade speed for feasibility.

Instead of one giant prompt, it can make many small calls:

```text
SCOUT_FILE
SCOUT_DIRECTORY
SUMMARIZE_SYMBOL
FIND_RELEVANT_FILES
PLAN_STEP
GENERATE_PATCH
REVIEW_PATCH
ANALYZE_TEST_FAILURE
COMPACT_MEMORY
VERIFY_DONE
```

This allows small local models to work on larger projects.

---

### Structured outputs

Operational LLM responses should be JSON validated against schemas.

The model should not freely decide to execute shell commands or modify files. It should propose structured actions. Tessera validates them, applies policy, asks for approval when needed, and only then executes.

---

### External memory

The complete history should live outside the prompt.

Default backend:

- SQLite

Optional backend:

- Redis

The prompt receives only the smallest useful slice of context.

---

### Summaries are not source of truth

Summaries help route attention.

They should not be used as the only basis for edits.

Before modifying a file, Tessera should reread the real file contents.

---

### Tests are the verifier

The first major stop condition is:

```text
a unit test was executed and passed
```

If the required language tool is missing, Tessera should report that clearly and stop. It should not install global toolchains.

---

## Planned slash commands

Inside the interactive session:

```text
/help
/doctor
/index
/status
/diff
/approve
/deny
/memory
/calls
/context
/config
/replay
/clear
/new
/exit
```

Examples:

```text
› /status
```

```text
Run: run-2026-06-23-001
Mode: existing_project
Stack: Go
Model: qwen2.5-coder:7b
Calls: 24/100
Steps: 6/20
Memory: sqlite
Last action: go test ./...
Last status: failed
```

```text
› /diff
```

```diff
diff --git a/internal/sum/sum.go b/internal/sum/sum.go
new file mode 100644
+package sum
+
+func Sum(a int, b int) int {
+    return a + b
+}
```

```text
› /approve
```

Approves the pending action.

```text
› /deny
```

Denies the pending action.

---

## Tech stack

### Core

- Go
- Cobra for CLI commands
- SQLite as the default memory backend
- Redis as an optional memory backend
- Ollama HTTP API as the first local LLM provider
- Tree-sitter for structural code indexing

### Terminal UI

MVP:

- plain interactive transcript UI
- copy-friendly output
- styled sections
- approval prompts
- diff rendering
- input history

Suggested Go libraries:

- Cobra for CLI
- Lip Gloss for styling
- Glamour for Markdown rendering
- Bubble Tea later for optional full-screen TUI

### Optional later

- llama.cpp server support
- OpenAI-compatible local servers
- MCP support
- richer sandboxing
- plugin system
- full-screen TUI mode

---

## Configuration

Tessera should be configurable through CLI flags, environment variables, and a local config file.

Priority:

```text
CLI flags > environment variables > .tessera/config.toml > defaults
```

Example:

```bash
TESSERA_MODEL=qwen2.5-coder:7b
TESSERA_LLM_PROVIDER=ollama
TESSERA_OLLAMA_URL=http://localhost:11434

TESSERA_MAX_CONTEXT=4096
TESSERA_MAX_INPUT=3000
TESSERA_MAX_OUTPUT=800
TESSERA_CHUNK_TOKENS=1200
TESSERA_MAX_CALLS=100
TESSERA_MAX_STEPS=20

TESSERA_MEMORY_BACKEND=sqlite
TESSERA_SQLITE_PATH=.tessera/memory.sqlite
TESSERA_REDIS_URL=redis://localhost:6379

TESSERA_REQUIRE_APPROVAL=true
TESSERA_ALLOW_DOCKER=false
TESSERA_ALLOW_INSTALL_DEPS_WITH_APPROVAL=true
TESSERA_FORBID_GLOBAL_INSTALL=true
TESSERA_BLOCK_SENSITIVE_FILES=true

TESSERA_SESSION_RESUME=true
TESSERA_UI_MODE=plain
TESSERA_TUI_ENABLED=false
```

---

## Local memory layout

Default local layout:

```text
.tessera/
  memory.sqlite
  sessions/
    current.json
  runs/
    run-001/
      prompt-001.json
      response-001.json
      stdout-001.txt
      stderr-001.txt
      patch-001.diff
  indexes/
  summaries/
  config.toml
```

SQLite stores structured metadata.

Large payloads such as prompts, responses, logs, and patches may be stored as files and referenced from SQLite.

---

## Architecture

```text
cmd/tessera
  │
  ▼
CLI Bootstrap
  │
  ├── one-shot commands
  │
  └── interactive session
          │
          ▼
Session Manager
  │
  ▼
Interactive Shell
  │
  ▼
Orchestrator
  │
  ├── Config Manager
  ├── Context Budget Manager
  ├── Project Profiler
  ├── Tree-sitter Indexer
  ├── Memory Store
  ├── LLM Adapter
  ├── Policy Engine
  ├── Tool Executor
  ├── Patch Manager
  └── Test Runner
```

The UI should render events emitted by the core. It should not own the agent logic.

Example events:

```text
SessionStarted
SessionResumed
ProjectProfileStarted
ProjectProfiled
IndexStarted
IndexFinished
LLMCallStarted
LLMCallFinished
ContextSplit
CommandProposed
ApprovalRequested
ApprovalGranted
ApprovalDenied
CommandStarted
CommandFinished
PatchProposed
PatchApplied
TestStarted
TestFinished
RunFinished
RunAborted
ErrorOccurred
```

---

## Suggested repository structure

```text
tessera/
  cmd/
    tessera/
      main.go
  internal/
    cli/
    config/
    contextbudget/
    executor/
    interactive/
      shell.go
      commands.go
      prompt.go
    llm/
      ollama/
    memory/
      sqlite/
      redis/
    orchestrator/
    patch/
    policy/
    project/
    session/
      manager.go
      state.go
    treesitter/
    testrunner/
    ui/
      plain/
        renderer.go
        approval.go
        diff.go
      tui/
        app.go
  templates/
    node-vitest/
    python-pytest/
    go-test/
    elixir-exunit/
    rust-cargo/
  docs/
    architecture.md
    schemas.md
    security.md
    ui.md
  .github/
    workflows/
  go.mod
  README.md
```

---

## Project profiling

Tessera should detect project type without using the LLM when possible.

Initial heuristics:

```text
package.json       → Node
pnpm-lock.yaml     → Node / pnpm
yarn.lock          → Node / yarn
mix.exs            → Elixir
pyproject.toml     → Python
requirements.txt   → Python
Cargo.toml         → Rust
go.mod             → Go
pom.xml            → Java / Maven
build.gradle       → Java / Gradle
docker-compose.yml → Docker
```

The profiler should also detect:

- empty directory;
- existing project;
- Git repository;
- likely test command;
- dependency manager;
- presence of test files.

---

## Tree-sitter indexing

Tessera uses Tree-sitter to create a compact structural map of the codebase.

A file summary may look like:

```json
{
  "path": "src/user_service.ts",
  "language": "typescript",
  "symbols": [
    {
      "name": "createUser",
      "kind": "function",
      "start_line": 12,
      "end_line": 42
    }
  ],
  "imports": ["./db", "./user"],
  "exports": ["createUser"],
  "has_tests_nearby": true
}
```

This allows Tessera to:

- select relevant files;
- avoid reading full files unnecessarily;
- split large files by symbol;
- build repo maps;
- keep prompts small.

---

## Empty project flow

When the current directory is empty or nearly empty:

```text
BOOT
PROFILE_PROJECT
CLASSIFY_PROJECT → EMPTY_PROJECT
ASK_TEMPLATE_PLAN
REQUEST_APPROVAL
CREATE_FILES
INSTALL_PROJECT_DEPS_WITH_APPROVAL
RUN_TESTS
ANALYZE_FAILURE if needed
FINISH when test passes
```

Initial templates:

```text
node-vitest
python-pytest
go-test
elixir-exunit
rust-cargo
```

Rules:

- prefer minimal project setup;
- ask for approval before installing dependencies;
- do not install global toolchains;
- stop clearly if required tools are missing.

---

## Existing project flow

When the current directory already contains a project:

```text
BOOT
PROFILE_PROJECT
CLASSIFY_PROJECT → EXISTING_PROJECT
INDEX_PROJECT
ROUTE_CONTEXT
READ_RELEVANT_FILES
PLAN_STEP
REQUEST_APPROVAL
APPLY_PATCH
RUN_TESTS
ANALYZE_FAILURE if needed
REPAIR
FINISH when test passes
```

Rules:

- do not send the entire repository to the LLM;
- use Tree-sitter summaries for routing;
- read real files before editing;
- apply patches only after approval;
- use test failures as the main feedback loop.

---

## Command policy

Tessera classifies commands before executing them.

### Safe commands

Examples:

```text
pwd
ls
tree
find
rg
git status
git diff
git log
cat with limits
go test when go.mod exists
npm test when package.json exists
mix test when mix.exs exists
```

### Approval required

Examples:

```text
npm install
pnpm install
yarn add
pip install
poetry add
mix deps.get
cargo add
go get
docker compose up
create file
modify file
delete file
```

### Blocked by default

Examples:

```text
sudo
curl | sh
wget | sh
rm -rf /
chmod -R 777
reading .env
reading secret files
installing global toolchains
```

---

## Example LLM action schema

```json
{
  "action": "run_command | read_files | write_file | apply_patch | ask_user | finish | abort",
  "reason": "string",
  "requires_approval": true,
  "commands": [
    {
      "cmd": "string",
      "reason": "string"
    }
  ],
  "files": [
    {
      "path": "string",
      "reason": "string"
    }
  ],
  "finish": {
    "status": "success | blocked | failed",
    "reason": "string"
  }
}
```

---

## Example test observation schema

```json
{
  "command": "npm test",
  "exit_code": 1,
  "status": "failed",
  "summary": "Test runner exists, but one assertion failed.",
  "important_output": "Expected 2, received 3",
  "next_hint": "Inspect src/sum.ts and test/sum.test.ts"
}
```

---

## MVP roadmap

### Milestone 0 — Skeleton + interactive session

- Create Go project.
- Add Cobra.
- Make `tessera` open an interactive session.
- Add `/help` and `/exit`.
- Add `.tessera/`.
- Add session file at `.tessera/sessions/current.json`.
- Add `tessera doctor`.

Acceptance:

```bash
tessera
```

opens a prompt, accepts text, supports `/help`, and exits with `/exit`.

---

### Milestone 1 — Copy-friendly terminal UI

- Add event renderer.
- Add session header.
- Show model, context, call count, and cwd.
- Render commands.
- Render outputs.
- Render approval prompts.
- Render diffs.
- Preserve terminal scrollback.

Acceptance:

- code and commands are easy to copy;
- the terminal does not clear the screen;
- output is readable in plain terminals.

---

### Milestone 2 — Memory

- Add SQLite memory backend.
- Persist sessions.
- Persist runs.
- Persist LLM calls.
- Persist observations.
- Add Redis backend behind the same interface.

Acceptance:

```text
› /status
› /memory
```

show persisted session and run state.

---

### Milestone 3 — LLM adapter

- Add Ollama adapter.
- Support model configuration.
- Support structured JSON responses.
- Store prompts and responses.
- Add timeouts.

Acceptance:

```bash
tessera doctor
```

validates the Ollama connection when configured.

---

### Milestone 4 — Project profiler

- Detect empty vs existing project.
- Detect stack.
- Detect manifests.
- Detect likely test runner.
- Show profile inside the session.

Acceptance:

```text
› /status
```

shows project mode, stack, and likely test runner.

---

### Milestone 5 — Tree-sitter indexer

- Add Tree-sitter.
- Support at least Go and JavaScript / TypeScript first.
- Save symbols and file summaries.
- Add `/index`.

Acceptance:

```text
› /index
```

saves an index with files, symbols, and line ranges.

---

### Milestone 6 — Context budget manager

- Enforce context limits.
- Split work into subcalls.
- Avoid blind truncation.
- Persist intermediate summaries.
- Add `/context`.
- Add `/calls`.

Acceptance:

- Tessera never exceeds `TESSERA_MAX_INPUT`;
- large work is split into smaller calls;
- `/calls` shows call count and budget usage.

---

### Milestone 7 — Empty project flow

- Add minimal templates.
- Create files after approval.
- Install project dependencies after approval.
- Run first test.
- Stop when test passes or blocker is found.

Acceptance:

```bash
mkdir demo
cd demo
tessera
```

```text
› create a Node project with one unit test
```

ends with a passing test or a clear blocker.

---

### Milestone 8 — Existing project flow

- Index project.
- Select relevant files.
- Read real files.
- Generate patch.
- Review patch.
- Apply patch after approval.
- Run tests.
- Analyze failures.
- Add `/diff`, `/approve`, and `/deny`.

Acceptance:

- in a small existing project, Tessera can add or fix a simple test without sending the whole repository to the LLM.

---

### Milestone 9 — Safety and UX

- Add robust command policy.
- Block sensitive files.
- Add clear approvals.
- Add diff preview.
- Add timeouts.
- Add `--dry-run`.
- Add `--yes`.
- Add `--max-calls`.
- Add `--max-steps`.

Acceptance:

- Tessera never runs sensitive commands without approval;
- Tessera never reads `.env` by default;
- blockers are reported clearly.

---

### Milestone 10 — Replay and debug

- Add `tessera replay <run-id>`.
- Add `/replay <run-id>`.
- Inspect prompts.
- Inspect responses.
- Inspect decisions.
- Export runs as JSONL.

Acceptance:

- a previous execution can be inspected and partially replayed.

---

### Milestone 11 — Optional full-screen TUI

- Add `tessera --tui`.
- Add chat panel, status panel, and activity panel.
- Support approval.
- Support diff view.
- Keep plain mode as default.

Acceptance:

- `tessera` still opens copy-friendly plain mode;
- `tessera --tui` opens the full-screen UI;
- both modes use the same core event system.

---

## Development bootstrap

```bash
mkdir tessera
cd tessera

go mod init github.com/alef-mac/tessera

mkdir -p cmd/tessera
mkdir -p internal/{cli,config,contextbudget,executor,interactive,orchestrator,patch,policy,project,session,treesitter,testrunner}
mkdir -p internal/llm/ollama
mkdir -p internal/memory/sqlite
mkdir -p internal/memory/redis
mkdir -p internal/ui/plain
mkdir -p internal/ui/tui
mkdir -p templates/{node-vitest,python-pytest,go-test,elixir-exunit,rust-cargo}
mkdir -p docs

touch cmd/tessera/main.go
touch README.md
```

---

## First command to implement

```bash
tessera
```

Desired output:

```text
╭────────────────────────────────────────╮
│ Tessera                                │
│ local-first coding agent               │
╰────────────────────────────────────────╯

Project: /path/to/project
Model:   qwen2.5-coder:7b
Memory:  sqlite
Context: 4096 tokens
Calls:   0/100

Type your task or /help.

›
```

---

## Second command to implement

```bash
tessera doctor
```

Desired output:

```text
Tessera Doctor

OS: linux
Arch: amd64
CWD: /path/to/project
LLM Provider: ollama
Model: qwen2.5-coder:7b
Ollama URL: http://localhost:11434
Memory: sqlite
SQLite Path: .tessera/memory.sqlite
Max Context: 4096
Max Input: 3000
Max Output: 800
Max Calls: 100
Require Approval: true
Docker Allowed: false
Session Resume: true
UI Mode: plain
Status: ready
```

---

## Non-goals for the MVP

Tessera should not initially try to:

- replace full IDE agents;
- support every language;
- implement a complex sandbox;
- do large cross-repository refactors;
- install global toolchains;
- depend on a remote LLM;
- optimize for speed over reliability.

---

## License

TBD.