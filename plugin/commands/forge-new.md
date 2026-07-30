---
description: Scaffold a new project using forge
---

Ask the user which forge blueprint to use if they haven't specified:

- **hackathon-app** — Next.js 14 + Tailwind + shadcn/ui
- **python-fastapi** — FastAPI with async, uv, Ruff
- **python-cli** — Python CLI using Typer
- **go-cli** — Go CLI using cobra + slog
- **blueprint-starter** — Meta-blueprint for authoring custom blueprints

Then run: `forge new <blueprint> <project-name>` in the current directory.

For hackathon-app, ask about optional features:
- `--var with_database=true` (Supabase)
- `--var with_auth=true` (Supabase Auth)
- `--var with_ai=true` (Anthropic SDK)

For python-fastapi:
- `--var with_database=true`
- `--var with_auth=true`
- `--var with_docker=true`
- `--var with_openai=true`

Use `--yes` to skip prompts once flags are decided.

Show the user the command before running it. If they approve, execute in bash.
