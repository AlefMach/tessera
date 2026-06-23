# Tessera

> A local-first interactive coding agent for small machines.

Tessera is an experimental command-line coding agent designed to work with local LLMs, limited hardware, and real projects that need careful handling.

It is built around a simple idea:

> Instead of sending an entire codebase to a huge model, Tessera breaks the work into small steps and builds understanding over time.

Tessera is for developers who want an agent-like coding workflow in the terminal, but prefer to keep their code local, control what gets executed, review changes before they are applied, and avoid depending on massive context windows.

---

## Why Tessera?

Most coding agents work best when they have access to very large context windows and powerful remote models. That can be expensive, slow, unavailable offline, or unsuitable for private codebases.

Tessera takes a different approach.

It treats a project like a mosaic:

```text
small pieces of code
  ↓
small local model calls
  ↓
structured understanding
  ↓
git-aware safety
  ↓
safe actions
  ↓
tests as verification
```

The name **Tessera** comes from the small tiles used to build mosaics.

---

## What Tessera does

Tessera opens an interactive terminal session where you can ask it to help with coding tasks:

```bash
tessera
```

Inside the session, you can describe what you want:

```text
› create the first unit test for this project
› explain why this test is failing
› add a minimal implementation
› show me the diff
› run the tests again
› suggest a commit message for these changes
```

Tessera then works step by step:

1. Looks at the current project.
2. Detects whether the directory is a Git repository.
3. Checks the current working tree before editing.
4. Builds a compact understanding of the relevant files.
5. Asks the local model for the next small action.
6. Shows what it wants to do.
7. Requests approval before changing files or running risky commands.
8. Applies changes.
9. Saves diffs and run metadata locally.
10. Runs tests.
11. Stops when the task is complete or when it finds a clear blocker.

---

## Key features

### Interactive terminal session

Run Tessera once and stay inside the session:

```bash
tessera
```

No need to keep running long one-shot commands. Tessera is meant to feel like a coding assistant living inside your terminal.

---

### Local-first workflow

Tessera is designed for local models and local repositories.

Your project does not need to be sent to a remote AI service for Tessera's core workflow.

---

### Small-context by design

Tessera is built for machines that cannot comfortably run huge context windows.

Instead of trying to fit everything into one prompt, it uses many smaller steps to understand and modify a project.

This makes it suitable for local LLM setups where memory is limited.

---

### Git-aware safety

Tessera is designed to work safely inside Git repositories.

Before modifying a project, Tessera should be able to:

- detect whether the current directory is a Git repository;
- inspect the working tree;
- warn when there are existing uncommitted changes;
- avoid overwriting user changes without approval;
- show diffs before applying patches;
- save patches inside the local Tessera run history;
- support rollback when Git metadata is available;
- suggest commit messages without committing automatically.

By default, Tessera should not commit, push, or rewrite Git history on its own.

---

### Approval before action

Tessera should never blindly modify your project.

Before changing files, installing project dependencies, or running sensitive commands, it asks for confirmation.

You stay in control.

---

### Copy-friendly terminal output

Tessera's default interface is designed to be readable and easy to copy from.

It should not take over your terminal, clear your scrollback, or make it hard to copy code, commands, logs, or diffs.

---

### Project-aware assistance

Tessera can inspect the current folder, detect whether it is an empty directory or an existing project, identify likely test commands, check for Git, and focus only on the relevant files.

---

### Tests as the source of truth

Tessera uses tests to verify progress.

A typical task is considered complete when the relevant test command runs successfully.

If a required tool is missing, Tessera should report the blocker clearly instead of silently installing global tools.

---

### Persistent local memory

Tessera keeps a local record of sessions, actions, prompts, responses, command results, patches, diffs, and observations.

This makes it easier to inspect what happened, resume work, debug a previous run, or roll back a change when possible.

---

## Example session

```text
$ tessera

╭────────────────────────────────────────╮
│ Tessera                                │
│ local-first coding agent               │
╰────────────────────────────────────────╯

Project: ~/code/example-app
Model:   local-model
Context: bounded
Memory:  local
Git:     clean

Type your task or /help.

› create the first unit test
```

Tessera may respond:

```text
● Inspecting project
  Found an existing Node project.

● Checking Git status
  Working tree is clean.

● Selecting relevant files
  package.json
  src/sum.ts

● Proposed action
  Create a test file for src/sum.ts.

Approve? [y] yes  [n] no  [d] show diff
```

After approval:

```text
▶ npm test

✓ Test passed

Changed files:
  A src/sum.test.ts

Suggested commit:
  test(sum): add initial unit test

Task completed.
```

---

## Slash commands

Inside the interactive session, Tessera may support commands such as:

```text
/help
/status
/diff
/git
/rollback
/commit-message
/approve
/deny
/context
/calls
/memory
/clear
/exit
```

These commands make the session easier to inspect and control without leaving the terminal.

Examples:

```text
› /diff
```

Shows the pending or latest patch.

```text
› /git
```

Shows the detected Git state for the current project.

```text
› /commit-message
```

Suggests a commit message for the current approved changes.

```text
› /rollback
```

Attempts to roll back the latest Tessera-applied change when enough metadata is available.

---

## Planned CLI commands

The main workflow is the interactive session:

```bash
tessera
```

Support commands may include:

```bash
tessera run "create the first unit test"
tessera doctor
tessera index
tessera git status
tessera rollback <run-id>
tessera replay <run-id>
```

The goal is to keep `tessera` as the primary experience and use subcommands for automation, debugging, CI, and recovery.

---

## What Tessera is good for

Tessera is intended for tasks such as:

- creating a minimal project structure;
- adding the first unit test;
- fixing simple or medium test failures;
- understanding a small or medium codebase;
- making localized changes;
- reviewing proposed diffs before applying them;
- running tests after changes;
- suggesting commit messages;
- working with local LLMs on limited hardware.

---

## What Tessera is not trying to be

At least initially, Tessera is not trying to be:

- a full IDE replacement;
- a cloud-first coding agent;
- a tool that edits your project without approval;
- a system that depends on massive context windows;
- an agent that installs global toolchains automatically;
- an agent that commits or pushes code without permission;
- a magic solution for large, risky refactors.

Tessera favors control, locality, and incremental progress.

---

## Safety model

Tessera should treat the local repository as user-owned and sensitive.

The default safety model is:

- read only what is relevant;
- avoid sensitive files by default;
- ask before modifying files;
- ask before installing project dependencies;
- block dangerous commands;
- keep command output and patches in local run history;
- use Git status and diffs as part of the review flow;
- use tests as the main verification signal.

This is especially important when working with local models, where the orchestrator should remain deterministic and the model should not control the whole system directly.

---

## Project status

Tessera is early-stage and experimental.

The goal is to build a practical local coding agent that prioritizes:

- small context windows;
- interactive terminal UX;
- user approval;
- Git-aware safety;
- local memory;
- test-driven verification;
- compatibility with limited hardware.

Expect the project to evolve quickly.

---

## Vision

Tessera aims to make local coding agents more practical by changing the tradeoff:

```text
less VRAM
more time
many small steps
safer actions
local control
```

Instead of asking:

> How large can the context window be?

Tessera asks:

> How far can we get with a small local model, careful context management, Git-aware safety, and good verification?

---

## License

TBD.