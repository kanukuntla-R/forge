# forge

A meta-CLI for scaffolding projects from blueprints. Single static Go binary.

Status: under active development. v0.1 in progress.

## What's working

- `forge --help` and subcommand help text
- `forge list` and `forge list --json` (lists embedded blueprints)
- `forge new <blueprint> <name>` with full input contract:
  - Defaults from manifest
  - `--var KEY=VALUE` repeatable flags
  - JSON stdin via `--json`
  - Interactive walkthrough form (when run in a TTY)
  - `--yes` to suppress prompts
- Atomic writes via stage-then-rename
- Conditional file inclusion in templates
- Hackathon-app blueprint: Next.js 14 + Tailwind + shadcn, with optional Anthropic AI integration and dark mode toggle (auth + database coming next)

## What's coming next

See [`docs/forge-design.md`](docs/forge-design.md) for the full v0.1 plan.
