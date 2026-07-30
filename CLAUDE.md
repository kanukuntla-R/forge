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

**v0.5.1 release in progress** — Claude Code plugin complete; cross-platform builds and tag pending.

All milestones M1 through M12.3 are complete:

- **M1-M8.6a** (walking skeleton through v0.2) ✅
- **M9.1-M9.5** (blueprint expansion, v0.3) ✅
- **M10.1** (Python adapter skeleton) ✅
- **M10.2** (Python import extraction) ✅
- **M10.3** (Python declaration extraction) ✅
- **M10.4** (FastAPI framework detector) ✅
- **M10.5** (Python HTTP call detection) ✅
- **M10.5.5** (cross-language route matching) ✅
- **M10.6** (dashboard visibility for FastAPI) ✅
- **M10.6.5** (graph layout spacing) ✅
- **M10.6.6** (cose-bilkent layout) ✅
- **M11.1** (database detector infrastructure) ✅
- **M11.2** (Prisma detector) ✅
- **M11.3** (Drizzle detector) ✅
- **M11.4** (dashboard visibility for databases) ✅
- **M11.4.5-6** (theme switcher + polish) ✅
- **M11.4.7** (theme switcher effect ordering fix) ✅
- **M11.5** (v0.5 polish + edge cases) ✅
- **M12.1-M12.3** (Claude Code plugin restructure, v0.5.1) ✅

95 commits on `main`. All tests green across 14 packages including database detection tests.

v0.5 adds database detection alongside existing framework analysis:
- Database detector infrastructure with pluggable detector interface
- Prisma detector: parses `schema.prisma`, detects clients, matches queries to models
- Drizzle detector: extracts tables from TS files, detects chained queries via MethodChains extractor
- Dashboard visibility: yellow ellipse table nodes, gold query edges with operation labels
- Theme switcher: Light/Dark/Auto with segmented control, localStorage persistence, prefers-color-scheme detection
- Multi-database support: projects can have Prisma + Drizzle together (both detected, both rendered)

v0.5.1 adds Claude Code plugin distribution:
- Plugin lives at `plugin/` with proper `.claude-plugin/plugin.json` metadata
- Skill file at `plugin/skills/forge/SKILL.md` with YAML frontmatter
- Slash commands `/forge-new` and `/forge-visualize` for explicit invocation
- Marketplace metadata at `.claude-plugin/marketplace.json` for discovery
- Users install via `/plugin marketplace add https://github.com/kanukuntla-R/forge` + `/plugin install forge`
- Verified end-to-end: skill auto-triggers on scaffolding prompts, guides through blueprint selection

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

All milestones M1 through M12.3 complete. See "Current status" above for the detailed breakdown.

Post-v0.5.1 roadmap:
- **v0.5.2+**: SQLAlchemy detector, Supabase JS client detector, Firebase detector, cross-language route matching for database queries, foreign key relationships, table field/column detection, client-variable tracking for HTTP call detection, more framework detectors (Astro, Remix, SvelteKit, Vue, Django, Flask), path-segment templating for meta-blueprints, monorepo Next.js support, caching with content hashes
- **v0.6+**: astro-site blueprint, openclaw-skill blueprint

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
