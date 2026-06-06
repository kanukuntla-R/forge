# forge

A meta-CLI for scaffolding projects from blueprints. Single static Go binary.

Status: under active development. v0.1 in progress.

## What's working

- `forge --help` and subcommand help text
- `forge list` and `forge list --json` (lists embedded blueprints)
- Variable resolution: defaults → JSON stdin → `--var` flags

## What's coming next

See [`docs/forge-design.md`](docs/forge-design.md) for the full v0.1 plan.
