# Tessera

> A local-first interactive coding agent for small machines.

Tessera is an experimental command-line coding agent designed to work with local LLMs and limited hardware.

It is built around a simple idea:

> Instead of sending an entire codebase to a huge model, Tessera breaks the work into small steps and builds understanding over time.

Tessera is for developers who want an agent-like coding workflow in the terminal, but prefer to keep their code local, control what gets executed, and avoid depending on massive context windows.

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
```

Tessera then works step by step:

1. Looks at the current project.
2. Builds a compact understanding of the relevant files.
3. Asks the local model for the next small action.
4. Shows what it wants to do.
5. Requests approval before changing files or running risky commands.
6. Applies changes.
7. Runs tests.
8. Stops when the task is complete or when it finds a clear blocker.

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

Tessera can inspect the current folder, detect whether it is an empty directory or an existing project, identify likely test commands, and focus only on the relevant files.

---

### Tests as the source of truth

Tessera uses tests to verify progress.

A typical task is considered complete when the relevant test command runs successfully.

If a required tool is missing, Tessera should report the blocker clearly instead of silently installing global tools.

---

### Persistent local memory

Tessera keeps a local record of sessions, actions, prompts, responses, command results, patches, and observations.

This makes it easier to inspect what happened, resume work, or debug a previous run.

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

Type your task or /help.

› create the first unit test
```

Tessera may respond:

```text
● Inspecting project
  Found an existing Node project.

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

Task completed.
```

---

## Slash commands

Inside the interactive session, Tessera may support commands such as:

```text
/help
/status
/diff
/approve
/deny
/context
/calls
/memory
/clear
/exit
```

These commands make the session easier to inspect and control without leaving the terminal.

---

## What Tessera is good for

Tessera is intended for tasks such as:

- creating a minimal project structure;
- adding the first unit test;
- fixing simple or medium test failures;
- understanding a small or medium codebase;
- making localized changes;
- reviewing proposed diffs before applying them;
- working with local LLMs on limited hardware.

---

## What Tessera is not trying to be

At least initially, Tessera is not trying to be:

- a full IDE replacement;
- a cloud-first coding agent;
- a tool that edits your project without approval;
- a system that depends on massive context windows;
- an agent that installs global toolchains automatically;
- a magic solution for large, risky refactors.

Tessera favors control, locality, and incremental progress.

---

## Project status

Tessera is early-stage and experimental.

The goal is to build a practical local coding agent that prioritizes:

- small context windows;
- interactive terminal UX;
- user approval;
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

> How far can we get with a small local model, careful context management, and good verification?

---

## License

TBD.