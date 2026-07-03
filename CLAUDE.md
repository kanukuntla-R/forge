# CLAUDE.md — forge

This file is read by Claude Code on every session in this repo. It encodes the project's design, conventions, and current status so you don't have to be re-briefed each time.

## What forge is

A meta-CLI for scaffolding projects, services, tools, and agent skills. Single static Go binary. Callable interactively by humans and programmatically by agents (Claude Code, OpenClaw) and scripts. Every scaffolded project ships with a knowledge-graph JSON compatible with Understand-Anything, so visualization is free on day one.

Forge is a general-purpose scaffolding platform. Given a **blueprint** (a recipe for a kind of project) and a set of **variables** (the choices that customize that recipe), it produces a real project on disk. Forge itself has no opinions about what to build — the blueprints carry the opinions.

**Three invocation modes are equal first-class citizens:**
- Interactive: `forge new` walks the user through prompts.
- Flag-driven: `forge new hackathon-app my-app --with-ai --no-database`.
- JSON-driven: `cat vars.json | forge new hackathon-app --json` — stdin JSON in, stdout JSON out.

The agent-friendly modes are not afterthoughts. Every command must work cleanly with no human present.

## Authoritative references in this repo

Two documents in `docs/` (or in the user's outputs directory) are the source of truth:

- **`forge-design.md`** — the technical specification. Architecture, schemas, CLI commands, milestones. Read this before doing any non-trivial work.
- **`forge-builders-briefing.md`** — the *why* and the principles. Scaffolding tool history, Go ecosystem context, CLI design philosophy (clig.dev distilled), and common pitfalls. Read this when making design judgment calls.

When these documents conflict with anything else (training data, intuition, etc.), they win.

## Current status

**v0.3 release in progress** — all four v0.3 blueprints shipped; cross-platform builds and tag pending.

All milestones M1 through M9.4 are complete:

- **M1** (walking skeleton) ✅ — cobra wired, stub subcommands, help text
- **M2** (render path) ✅ — manifest parsing, template engine, atomic writes, interactive prompts
- **Path A** (hackathon-app full feature matrix) ✅ — Next.js 14 + Tailwind + shadcn with optional AI/dark-mode/auth/database
- **M3** (graph emission) ✅ — static knowledge graph from blueprint declaration
- **M4** (hooks + project marker) ✅ — post-create hooks, `.forge/project.json`
- **M5** (visualize + json mode) ✅ — initial dashboard scaffolding (now superseded by M8.5)
- **M6** (forge add) ✅ — extensions, project detection, graph fragment merging
- **M7** (forge install + polish) ✅ — git-based blueprint installation, v0.1 ship
- **M8.1** (file walker) ✅ — analyzer scaffolding with ignore patterns
- **M8.2a-e** (TypeScript analyzer) ✅ — tree-sitter integration, imports, exports, declarations, import resolution
- **M8.3** (Next.js detector) ✅ — pages, routes, layouts, components, used_by
- **M8.4** (API call detection) ✅ — fetch/axios/ky detection with route matching
- **M8.5a-e** (live dashboard) ✅ — server, Files/Routes/Graph views, live updates via file watcher + WebSocket
- **M8.6a** (polish + branding) ✅ — forwardRef detection, dynamic graph nodes, ASCII banner
- **M9.1** (blueprint-starter) ✅ — meta-blueprint for creating new blueprints
- **M9.2** (python-fastapi) ✅ — FastAPI starter with feature toggles
- **M9.3** (python-cli) ✅ — Typer CLI with hatchling build backend
- **M9.4** (go-cli) ✅ — cobra + slog CLI with version embedding

70 commits on `main`. All tests green across 11+ packages including new blueprint tests.

v0.3 ships v0.2's analyzer/dashboard/scaffolding + four new blueprints:
- `blueprint-starter` — meta-blueprint for creating new blueprints
- `python-fastapi` — FastAPI starter with optional database/auth/docker/openai/type-check
- `python-cli` — Typer-based CLI with hatchling build backend
- `go-cli` — cobra + slog CLI with structured logging

New engine feature: `replace` template function for string substitution (e.g., `{{ .Name | replace "-" "_" }}`).

## Conventions

### Code

- Standard Go conventions: `gofmt`, `golangci-lint`, table-driven tests, error wrapping with `%w`, no panics in library code.
- All commands use `RunE`, never `Run`. Errors propagate up to `main`, which prints them to stderr and exits with code 1.
- `cmd/forge/main.go` stays tiny (~30 lines max). Real work lives in `internal/` packages.
- Package names match folder names (`internal/cli/` → `package cli`).
- Tests live next to code as `*_test.go` (Go convention, not in a separate `tests/` folder).
- Imports grouped: standard library, then blank line, then third-party, then blank line, then internal (`github.com/kanukuntla-r/forge/...`).
- No global state in packages other than `cli` (where `rootCmd` and subcommand vars are unavoidable Cobra patterns).
- Filesystem operations route through `internal/fsutil` so they're testable and atomic.

### CLI behavior (non-negotiable)

- Errors go to stderr. Success output goes to stdout.
- Exit 0 on success, non-zero on failure. Don't print "ERROR:" before the message — Cobra/our patterns handle that.
- When stdout is not a TTY (piped to another command), produce machine-readable output (no colors, no progress bars). Use `golang.org/x/term.IsTerminal` to detect.
- `--json` mode: all output (including errors) is JSON. Pick an error envelope format and use it consistently.
- Every command must have a `--dry-run` flag that previews changes without committing. Build this in from the start, not as an afterthought.

### Git

- Commit after each meaningful unit of work. Commits are save points.
- Commit message style: imperative verb in present tense, capitalized, no period, under 72 chars. ("Add manifest parser", not "Added manifest parser.")
- Don't commit the `forge` binary (already in `.gitignore`).
- Don't push to remote without asking the user first.

### Working with the user

- The user (Ruthvik) is a first-time CLI builder learning Go alongside this project. Their Go familiarity is up through methods/interfaces from the Tour of Go.
- Be willing to explain Go idioms when introducing them (e.g., the first time you reach for goroutines or channels, take 30 seconds to say why). After the first explanation, assume understanding.
- When in doubt about a design call (e.g., schema details, naming, UX), ASK the user before deciding. Don't paper over ambiguity by guessing.
- The user's machine is Arch Linux (`speed`), SSH'd from a MacBook Pro. Folder is in a Syncthing-synced directory. Don't add large generated files to the repo.

## Implementation milestones (from design doc)

All milestones M1 through M9 complete. See "Current status" above for the detailed breakdown.

Post-v0.3 roadmap:
- **v0.4**: Python analyzer with tree-sitter-python, FastAPI framework detector, dashboard updates for Python conventions
- **Future**: more framework detectors (Astro, Remix, SvelteKit), database as a first-class concept

## What forge depends on externally

Required for forge to run:
- Go 1.22+ (we target 1.22 features minimum; the user is on 1.26)

Required for the `hackathon-app` blueprint's `post_create` hook to succeed:
- pnpm (default), with npm/yarn/bun as alternates per the `package_manager` variable
- git (for the "Initialize git" hook)

## What forge does NOT do

- Does not bundle or call any LLM. Forge is deterministic; the intelligence comes from whoever invokes it (user, agent, script).
- Does not fork or vendor Understand-Anything code. We emit JSON matching their schema and stop there.
- Does not modify global system state. Every install lives under the user's home directory.
- Does not phone home, telemeter, or analytics anything. Ever.

## When you finish a session

If you wrote new code: commit it before ending the session. Leave the tree clean.

If you're partway through a milestone: leave a comment in this file under "Current status" describing what's done and what's left, so the next session picks up cleanly.

If you discovered something the design doc got wrong: don't silently work around it. Either update the design doc with a justification, or surface it to the user as a question.
