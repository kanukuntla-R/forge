# forge release notes

## v0.3.0 — Blueprint expansion

Released: [date when actually tagged]

Four new blueprints for common project types. The blueprint library now covers Next.js, FastAPI, Python CLI, Go CLI, and meta-blueprint scaffolding.

### New blueprints

- **`blueprint-starter`** — Meta-blueprint that scaffolds the structure for creating new forge blueprints. Includes a working note-taking demo app (HTML/CSS/JS + localStorage), an example extension, and a knowledge graph. Use this when authoring your own blueprint.

- **`python-fastapi`** — Minimal FastAPI starter with async support, uv for dependency management, Ruff, and pytest-asyncio. Feature toggles for database (SQLAlchemy 2.0 async + Alembic), auth (JWT), Docker, OpenAI/Anthropic SDK, and mypy strict type checking. Enabling auth automatically enables database.

- **`python-cli`** — Minimal Python CLI starter using Typer, uv, and Ruff. Includes hatchling build backend so `uv run <cli-name>` works out of the box. One example command demonstrating options and flags.

- **`go-cli`** — Minimal Go CLI starter using cobra and slog (Go stdlib structured logging). Standard project layout, Makefile with version embedding, and one example command demonstrating flags and structured logging.

### Engine additions

- **New `replace` template function** — String substitution in templates. Enables patterns like `{{ .Name | replace "-" "_" }}` for cases where hyphens need to become underscores (e.g., PostgreSQL identifiers, Python module names).

### Improvements

- Fixed Makefile `vv` quirk — `make build` no longer produces `vv0.3.0` in version output. Same fix baked into the go-cli blueprint's Makefile.
- New documentation: `docs/blueprint-authoring.md` covering the two-level scaffolding constraint that meta-blueprints hit.
- Development notes tracking in `NOTES.md` for items deferred to future milestones.

### Internals

- 4 new commits (M9.1–M9.4)
- All tests green across 11+ packages, including new blueprint-specific tests
- Binary size unchanged (~19MB)

---

## v0.2.0 — Analyzer + Dashboard

Released: 2026-06-28

forge can now analyze projects and visualize their structure in real time. The major addition over v0.1 is the M8 milestone: a TypeScript/JavaScript analyzer plus a live-updating dashboard.

### New commands

- `forge analyze [path]` — runs static analysis and writes `.forge/analysis.json`
- `forge visualize [path]` — analyzes the project, then opens an interactive dashboard

### Analyzer

- File walker with ignore patterns (`node_modules`, `.git`, `.next`, etc.)
- TypeScript parsing via tree-sitter (handles `.ts`, `.tsx`, `.js`, `.jsx`)
- Import extraction with externality detection and resolution via `tsconfig.json` path aliases
- Export extraction with re-export tracking
- Top-level declaration extraction with component detection (functions, classes, variables, arrow functions, React.forwardRef, React.memo, React.lazy)
- Next.js framework detector — identifies pages, API routes, layouts, components, and their relationships
- HTTP API call detection — recognizes `fetch`, `axios`, `ky`, plus heuristic detection of other patterns
- Route matching — connects detected API calls to their corresponding route handlers

### Dashboard

Three views accessible via tabs:

- **Files view** — collapsible directory tree with details panel showing each file's imports, exports, declarations, and API calls
- **Routes view** — connections (page → route), pages list, API routes list
- **Graph view** — force-directed visualization using cytoscape.js, with three node types (pages, routes, components) and three edge types (API calls, imports, component usage) with toggleable visibility

Live updates:

- File watcher detects changes via fsnotify, debounced at 300ms
- WebSocket pushes update notifications to the browser
- Browser re-fetches and re-renders automatically
- Server shuts down gracefully when no browser is connected for 30 seconds

### Branding

- ASCII banner displayed at install, `forge --help`, and `forge --version`
- Dark blue + orange color scheme
- `NO_COLOR` environment variable respected

### Improvements

- `forge visualize` always re-analyzes on startup (fixed a confusing staleness bug)
- Component detection recognizes `React.forwardRef`, `React.memo`, `React.lazy`, and their bare forms
- Dynamic node sizing in graph view — routes and components fit their label content
- Edge deduplication — when import and usage edges exist between the same pair, usage takes priority
- Better empty state messages throughout the dashboard

### Internals

- 64 commits on `main`
- 11 packages, all tests green
- Two new dependencies: `github.com/fsnotify/fsnotify` (file watching) and `github.com/gorilla/websocket` (live updates)
- Binary size grew from ~18MB to ~19MB

---

## v0.1.0 — Initial release

The original v0.1 ship. Scaffolding from blueprints, extensions via `forge add`, blueprint installation via `forge install`, post-create hooks, and the `hackathon-app` blueprint with optional AI/dark-mode/auth/database features.
