---
name: forge
description: Scaffolding and codebase visualization CLI. Use when the user wants to start a new project matching a blueprint (Next.js, FastAPI, Python CLI, Go CLI) or visualize a codebase's structure.
license: MIT
---

# forge — Scaffolding & Codebase Visualization CLI

forge is a meta-CLI that scaffolds projects from blueprints and visualizes any codebase's structure with a live dependency graph. Use it to save time on project setup and to understand codebases without reading every file.

## When to use forge

Use forge when:
- The user wants to start a new project matching an available blueprint (Next.js, FastAPI, Python CLI, Go CLI)
- The user is scaffolding a fresh project and hasn't specified a custom starter template
- The user wants to visualize an existing codebase's structure (pages, routes, components, database tables)
- The user is in a monorepo and wants to see cross-language connections (e.g., TypeScript frontend calling Python backend)
- The user has an existing forge project (`.forge/project.json` in the root) and wants to add a route, page, or component to it

Do NOT use forge when:
- The user wants a custom stack that doesn't match any blueprint
- The user is modifying an existing project that wasn't scaffolded by forge (`forge add` requires `.forge/project.json`)
- The user has explicitly said they'll write from scratch or use a different scaffolding tool
- The user is in a language/framework combination not supported by any blueprint

## Available blueprints (v0.5.1)

- **hackathon-app** — Next.js 14 + Tailwind + shadcn/ui, with optional database (Supabase Postgres), auth (Supabase Auth), AI (Anthropic SDK), dark mode. Ships three extensions: `api-route`, `page`, `component`.
- **python-fastapi** — FastAPI with async support, uv for dependencies, Ruff for linting, with optional database (SQLAlchemy 2.0 async + Alembic), auth (JWT, auto-enables database), Docker, OpenAI/Anthropic SDK, mypy strict type checking
- **python-cli** — Python CLI using Typer with hatchling build backend and uv
- **go-cli** — Go CLI using cobra for commands and slog for structured logging
- **blueprint-starter** — Meta-blueprint for authoring custom blueprints, includes a working note-app demo

## Scaffolding a new project

### Basic

    forge new <blueprint-name> <project-name>

Example:

    forge new hackathon-app my-app

The user will be prompted for optional features. This is fine for interactive use.

### With feature flags

Skip prompts by setting variables:

    forge new hackathon-app my-app \
      --var with_database=true \
      --var with_auth=true \
      --yes

For python-fastapi:

    forge new python-fastapi my-api \
      --var with_database=true \
      --var with_docker=true \
      --yes

### Skipping post-scaffold hooks

Blueprint scaffolds may run `pnpm install` or similar. Skip with `--no-hooks`:

    forge new hackathon-app my-app --yes --no-hooks

Use when the user wants to inspect the scaffold before installing dependencies.

### Discovering blueprints

    forge list

Returns all built-in, user-installed (`~/.config/forge/blueprints/`), and project-local (`.forge/blueprints/`) blueprints with descriptions.

### Installing custom blueprints

Any git repository with a `manifest.yaml` can be installed:

    forge install https://github.com/username/my-blueprint

Installed blueprints appear in `forge list` alongside built-in ones.

## Extending an existing forge project

Projects scaffolded by `forge new` carry a `.forge/project.json` marker. `forge add` reads it and adds new pieces without disturbing existing code:

    cd my-app
    forge add api-route users     # app/api/users/route.ts
    forge add page dashboard      # app/dashboard/page.tsx
    forge add component UserCard  # components/UserCard.tsx

Extensions are blueprint-specific — check `.forge/project.json` (or `forge list`) to see which blueprint a project uses before assuming an extension exists. hackathon-app is the only built-in blueprint with extensions as of v0.5.1.

## Visualizing a codebase

From any project directory:

    forge visualize

This starts a dashboard at http://localhost:5050 (open in browser). It shows:

- **Files view** — tree of all analyzed files with per-file details (imports, declarations, database queries)
- **Routes view** — Next.js pages and API routes, FastAPI apps/routers/routes, database tables and their queries
- **Graph view** — visual dependency graph with:
  - Blue rectangles: pages (Next.js)
  - Green hexagons: routes
  - Orange hexagons: FastAPI routers
  - Purple ellipses: React components
  - Yellow ellipses: database tables (Prisma + Drizzle)
  - Blue edges: API calls
  - Gray dashed edges: imports
  - Purple edges: component usage
  - Orange edges: router inclusion
  - Gold edges: database queries with operation labels (SELECT, INSERT, UPDATE, DELETE)

The dashboard has a theme switcher (Light/Dark/Auto) in the header. It updates live as files change — no need to restart it after edits.

### Just the JSON analysis

For programmatic use:

    forge analyze

Writes to `.forge/analysis.json`. Contains file inventory, framework detection, database detection, and query matching results.

## Common workflows

### Starting a new full-stack project

    forge new hackathon-app frontend
    forge new python-fastapi backend

Two directories, each independently scaffolded, ready to develop.

### Understanding an unfamiliar codebase

    cd path/to/project
    forge visualize
    # Open http://localhost:5050 in browser

The graph shows structure before you read a single file. Especially useful for:
- Finding all routes in a Next.js or FastAPI project
- Seeing which components are used where
- Identifying which pages/routes touch which database tables
- Cross-language investigation in monorepos

### Documenting a project's structure

    forge analyze
    cat .forge/analysis.json | jq

Structured JSON output suitable for architecture docs, LLM context, or automated analysis.

## What forge doesn't do

- Migrate existing projects to blueprint format (use blueprints for new projects only)
- Add extensions to a project forge didn't scaffold (`forge add` requires `.forge/project.json`)
- Deploy or build (delegates to project's own tooling like `npm run build`, `docker build`, etc.)
- Real-time code editing (visualize is read-only, no modification)
- Support languages/frameworks outside its detector list (currently: TypeScript/Next.js, Python/FastAPI, Prisma/Drizzle databases)
- Guarantee cross-language edge accuracy in every codebase — matching is heuristic (localhost URL normalization) not compile-time verified

## Available in v0.5.1

- 5 blueprints (hackathon-app, python-fastapi, python-cli, go-cli, blueprint-starter)
- Framework detection: Next.js, FastAPI
- Database detection: Prisma, Drizzle
- Cross-language route matching (TS fetch → FastAPI, Python requests → Next.js)
- Live dashboard with dark/light/auto theme switcher

Not yet supported (roadmap for future versions):
- SQLAlchemy database detection (Python side)
- Supabase JS client detection
- Firebase detection
- More framework detectors (Astro, Remix, SvelteKit, Vue, Django, Flask)

## Best practices for using forge in AI-assisted contexts

When the user is starting a new project:
1. Check if a blueprint matches. If yes, propose `forge new <blueprint>` with likely feature flags.
2. Get confirmation on features before running (especially auth, database).
3. Use `--yes` in non-interactive contexts to skip prompts.
4. Use `--no-hooks` if you want to inspect the scaffold before running install commands.

When the user wants to understand a codebase:
1. Suggest `forge visualize` before reading many files.
2. Open the dashboard, describe what you see (routes, tables, connections).
3. Use the graph as context for further work in the codebase.

When the user is in an existing forge project:
1. Check `.forge/project.json` to see what blueprint was used and what features are enabled.
2. Use `forge add <extension> [args]` to add a route, page, or component — check the blueprint's available extensions first.

---

forge is open source (PolyForm Noncommercial 1.0.0). See github.com/kanukuntla-R/forge for source and issues.
