# forge release notes

## v0.2.0 — Analyzer + Dashboard

Released: [date when actually tagged]

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
