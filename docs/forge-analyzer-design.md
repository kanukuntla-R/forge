# forge live code analyzer & visualizer — design document

Owner: Ruthvik Kanukuntla
Status: design complete, M8.1 next
Target: v0.2 release
Estimate: 5-7 weeks of focused work for M8.1 through M8.6

## What we're building

A developer tool, integrated into forge, that:

1. Scans any codebase (forge-scaffolded or arbitrary)
2. Extracts architectural information — files, modules, imports, calls, cross-stack relationships
3. Produces a JSON file containing the complete extracted information
4. Hosts a local web dashboard that renders this information as an interactive visualization

This replaces `forge visualize`'s current dependency on the external "understand-anything" tool. forge becomes self-contained for code visualization.

## Why this matters

The current `forge visualize` shells out to a third-party tool that the user has to install separately. That's friction. It also means forge only knows about its own scaffolded blueprints — once a user modifies their code, forge has no way to know what's there.

A real live analyzer means:
- forge users get visualization out of the box, no extra installs
- The graph reflects reality (what code actually exists), not just intent (what the blueprint declared)
- The tool works on *any* codebase, even ones not scaffolded by forge — useful for understanding inherited projects, debugging architecture, onboarding new team members

## Architecture overview

Three logical components:
```
┌────────────────┐     ┌────────────────┐     ┌────────────────┐
│   Analyzer     │ ──▶ │   JSON file    │ ──▶ │   Dashboard    │
│  (reads code)  │     │  (snapshot)    │     │  (renders it)  │
└────────────────┘     └────────────────┘     └────────────────┘
Go                  on disk            embedded HTML/JS
                                       served by Go server
```

Each piece has a clear input/output contract. They can be developed independently.

### The analyzer (Go)

A Go package that walks a directory, identifies file types and structure, and produces a structured representation. Lives at `internal/analyzer/`.

The analyzer is built with a pluggable architecture from day one. It supports adding new language adapters (Python, Go, Rust, etc.) and framework detectors (Django, Express, Gin, etc.) without changing core code. For v0.2, only the TypeScript/TSX/JavaScript adapter and Next.js framework detector ship. Other languages are M8.7+ work after v0.2 lands.

### The JSON file (on disk)

The contract between analyzer and dashboard. Written to `.forge/analysis.json` in the project being analyzed. Contains:
- File inventory (all source files)
- Module relationships (imports/exports)
- Function/class declarations
- Function calls (when traceable)
- Routes (Next.js conventions)
- Frontend-to-backend connections (matched fetch calls to API routes)

The schema is versioned so we can evolve it without breaking the dashboard.

### The dashboard (HTML + JavaScript)

A web app that reads the JSON and renders an interactive visualization. Embedded into the forge binary using `//go:embed` (same pattern as blueprints). When the user runs `forge visualize`, forge:

1. Starts a small HTTP server on a free local port (e.g., `localhost:8765`)
2. Serves the embedded HTML/JS at `/`
3. Serves the JSON at `/api/analysis`
4. Opens the user's browser to the local URL

The dashboard is a single-page web app that fetches the JSON and renders nodes/edges with a graph layout library. Exact framework choice (vanilla JS + Cytoscape vs. small React app) decided at M8.5.

## Architecture: language adapters

```
┌─────────────────┐
│   File walker   │  (generic, language-agnostic)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Language       │  (per-language adapters)
│  detector       │  ┌──────────────────┐
└────────┬────────┘  │ TypeScript/TSX   │  ← ships in v0.2
         │           │ JavaScript       │  ← ships in v0.2
         ▼           │ Python           │  ← M8.7+
┌─────────────────┐  │ Go               │  ← M8.7+
│   Adapter       │ ─┤ Rust             │  ← M8.7+
│   dispatch      │  └──────────────────┘
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Unified JSON  │  (same schema regardless of language)
└─────────────────┘
```

### Language adapter contract

```go
type LanguageAdapter interface {
    // Name returns the language this adapter handles ("typescript", "python", etc.)
    Name() string

    // FileExtensions returns the extensions this adapter handles ([".ts", ".tsx"])
    FileExtensions() []string

    // Detect returns true if this adapter should handle the given file.
    // Allows for more nuanced detection than just extension matching.
    Detect(path string, content []byte) bool

    // Analyze parses a file and returns extracted information.
    Analyze(path string, content []byte) (*FileAnalysis, error)
}

type FileAnalysis struct {
    Imports      []Import
    Exports      []Export
    Declarations []Declaration  // functions, classes, components
    Calls        []Call          // function/API calls
    Metadata     map[string]any  // language-specific extras
}
```

### Framework detector contract

Frameworks (Next.js, Django, Express, etc.) are separate from languages. A framework detector recognizes patterns specific to a framework and enriches the analysis with that framework's structural concepts (routes, pages, controllers, etc.).

```go
type FrameworkDetector interface {
    Name() string

    // Detect returns true if this framework is in use.
    // Usually checks for marker files (next.config.js, manage.py, etc.)
    Detect(projectRoot string) bool

    // EnrichAnalysis adds framework-specific structure to the analysis
    // (e.g., identifies routes from file paths, marks pages, etc.)
    EnrichAnalysis(analysis *ProjectAnalysis) error
}
```

## The JSON schema

```json
{
  "version": "1",
  "generated_at": "2026-06-18T12:34:56Z",
  "cached_from": "2026-06-18T12:30:00Z",
  "content_hash": "sha256:...",
  "project": {
    "root": "/path/to/project",
    "name": "my-app",
    "languages": ["typescript"],
    "frameworks": ["nextjs"]
  },
  "files": [
    {
      "id": "app/page.tsx",
      "path": "app/page.tsx",
      "language": "typescript",
      "size_bytes": 1234,
      "exports": [
        { "name": "default", "type": "function", "line": 5 }
      ],
      "imports": [
        { "source": "@/components/UserCard", "resolved": "components/UserCard.tsx", "names": ["UserCard"] },
        { "source": "next/link", "resolved": null, "names": ["Link"], "external": true }
      ]
    }
  ],
  "frameworks": {
    "nextjs": {
      "routes": [
        {
          "id": "app/api/users",
          "path": "/api/users",
          "file": "app/api/users/route.ts",
          "methods": ["GET", "POST"]
        }
      ],
      "pages": [
        {
          "id": "app/dashboard",
          "path": "/dashboard",
          "file": "app/dashboard/page.tsx"
        }
      ],
      "components": [
        {
          "id": "components/UserCard",
          "name": "UserCard",
          "file": "components/UserCard.tsx",
          "used_by": ["app/page.tsx", "app/dashboard/page.tsx"]
        }
      ],
      "api_calls": [
        {
          "from_file": "app/page.tsx",
          "to_route": "/api/users",
          "method": "GET",
          "line": 23,
          "confidence": "high"
        }
      ]
    }
  },
  "external_dependencies": [
    { "package": "next", "used_by": ["app/page.tsx", "app/layout.tsx"] },
    { "package": "@supabase/supabase-js", "used_by": ["lib/supabase/client.ts"] }
  ]
}
```

The schema separates what exists (files, routes, components) from how things connect (imports, api_calls). The dashboard can render either view or both.

The `frameworks` section is keyed by framework name. Each framework can add its own structured data without polluting the base schema. The dashboard renders framework-specific views when those sections are present.

The `confidence` field on `api_calls` is important: matching frontend fetches to backend handlers is heuristic. Sometimes we can be sure (literal string `/api/users` matches the route). Sometimes we can guess (`fetch(/api/${variable})` could match many things). The `confidence` field surfaces this uncertainty to the user instead of hiding it.

## TypeScript parsing approach

We use tree-sitter via Go bindings (`github.com/smacker/go-tree-sitter`). tree-sitter is a fast, incremental parser used by Neovim, GitHub, and other production tools. It has grammars for TypeScript, TSX, JavaScript, and most languages.

Rationale:
- forge stays a single static Go binary (matches design philosophy)
- Real AST parsing, not pattern matching (robust to code styles)
- Same approach scales to other languages later (Python, Go, Rust all have tree-sitter grammars)
- The Go bindings are stable and used in production tools

Cost: tree-sitter grammars are large. The TypeScript grammar is ~10MB of compiled C. forge's binary will grow from ~10MB to ~30-40MB. Acceptable for the capability gained.

## Dashboard architecture

Three main views:

1. **Graph view**: nodes (files, components, routes) with edges (imports, calls, references). Click a node to see details. This is the headline visualization.
2. **File tree view**: hierarchical browser of the project, annotated with metadata (size, type, number of imports/exports).
3. **Stack view** (the cross-stack analysis): frontend pages on one side, backend routes on the other, lines connecting matched API calls. Shows the request flow at a glance.

The dashboard reads `/api/analysis` (served by the embedded Go server) and renders. It's built once with a JS bundler, then embedded into forge via `//go:embed`.

Framework choice (vanilla JS + Cytoscape vs. small React app) decided at M8.5.

## CLI surface

```
forge visualize                  # Analyze cwd, start dashboard, open browser
forge visualize ./my-project     # Analyze a specific path
forge visualize --no-open        # Don't auto-open the browser
forge visualize --port 8080      # Use a specific port (default: random free port)
forge visualize --json           # Output JSON to stdout, no dashboard
forge visualize --analyze-only   # Write analysis.json, don't start dashboard
```

The default flow (`forge visualize`) is "give me a dashboard for my current directory."

## Caching strategy

The analysis JSON gets written to `.forge/analysis.json` in the project. On each `forge visualize`:

1. Check if cache exists
2. Walk the project and compute a content hash of all source files (fast — just hashing bytes)
3. If hash matches the cached `content_hash` field, use the cached JSON (skip analysis)
4. If hash differs, run full analysis and update cache
5. Either way, start the dashboard

Re-running `forge visualize` on an unchanged codebase is near-instant. First runs and re-runs after edits do the full work.

Incremental analysis (only re-analyze changed files) is not in v0.2 scope. Full re-analyze on any change. Revisit when we have performance data showing we need incremental.

## Stop mechanism

Two ways to stop the dashboard, both work simultaneously:

1. **Ctrl+C in terminal**: forge handles SIGINT, shuts down the server cleanly, prints "Dashboard stopped."
2. **Browser close detection**: the dashboard sends a heartbeat to the server every few seconds. If the server doesn't receive a heartbeat for ~15 seconds, it assumes the browser is closed and shuts down.

Edge case: if user opens two browser tabs, both heartbeat. Server stays up until all tabs close. Acceptable behavior.

## Multi-project support

Multiple `forge visualize` instances can run simultaneously, each on a different free port. No shared state between them. Each manages its own server lifecycle independently.

## Roadmap (milestones)

### M8.1: Generic file walker and adapter scaffolding (~3-4 days)

Build the file walker, the `LanguageAdapter` interface, and the dispatch mechanism. Ship with one trivial adapter (call it "basic" — just records file existence, size, no parsing). This proves the architecture without committing to parser implementation yet.

**Deliverable:** `forge analyze` writes `.forge/analysis.json` with file inventory across all files.

### M8.2: TypeScript adapter via tree-sitter (~1-2 weeks)

Implement the `LanguageAdapter` for TypeScript/TSX/JavaScript. Use tree-sitter Go bindings. Extract imports, exports, declarations. This is the biggest engineering chunk in M8.

Includes: respecting `tsconfig.json` path aliases (`@/` etc.) for import resolution.

**Deliverable:** full module-level analysis for TS projects.

### M8.3: Next.js framework detector (~3-4 days)

Implement `FrameworkDetector` for Next.js. Detect routes (`app/**/page.tsx`), API routes (`app/api/**/route.ts`), components, layouts. Add to the `framework` section of the JSON.

**Deliverable:** Next.js projects get rich structural data.

### M8.4: API call detection (~1 week)

Find `fetch()` and similar calls in frontend code. Extract URLs. Match to routes. Surface confidence levels. Framework-specific (each framework's "what's an API call" pattern is different) but for v0.2 we just handle Next.js + standard fetch.

**Deliverable:** cross-stack relationships in the JSON.

### M8.5: Dashboard MVP (~1-2 weeks)

Build the web UI. Three views (graph, file tree, stack view). Local server. Browser auto-open. Stop mechanism (both modes). Heartbeat-based browser close detection.

**Deliverable:** working interactive dashboard.

### M8.6: Caching + polish + replace forge visualize (~3-4 days)

Implement the content-hash caching. Tear out understand-anything fallback. Polish error messages. Update README.

**Deliverable:** v0.2 release of forge with full live analysis.

### M8.7+ (post-v0.2): additional language adapters

Pick a second language (Python, Go, etc.). Implement the adapter. Validate the pluggable architecture by adding a real second language. Not v0.2 scope.

## Open questions resolved

- **JSON schema versioning**: schema is versioned. Dashboard checks version on load. Update JSON in real-time on each run, but use cached version when content hasn't changed.
- **Caching**: yes, cache by content hash. Re-run on unchanged code is near-instant.
- **Non-target languages**: pluggable architecture from day one. v0.2 only ships TypeScript. Other languages are post-v0.2 work using the same architecture.
- **Multiple simultaneous projects**: each gets its own port. No shared state.
- **Stop mechanism**: both Ctrl+C and browser close detection.
- **External link resolution**: read `tsconfig.json` and respect path aliases. Implemented in M8.2.
- **Dashboard tech stack**: decided at M8.5. Currently leaning vanilla JS + Cytoscape.js for minimal embedded footprint.
