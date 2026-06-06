# forge — Design Document (v0.1)

A meta-CLI for scaffolding projects, services, tools, and agent skills. Single static Go binary. Callable interactively by humans and programmatically by agents (Claude Code, OpenClaw) and scripts. Every scaffolded project ships with a knowledge-graph JSON compatible with Understand-Anything, so visualization is free on day one.

## What forge is

Forge is a general-purpose scaffolding platform. Given a **blueprint** (a recipe for a kind of project) and a set of **variables** (the choices that customize that recipe), it produces a real project on disk. Blueprints can be for anything: a hackathon web app, a Go CLI, an OpenClaw skill, a Python pipeline, a browser extension. Forge itself has no opinions about what you build — the blueprints carry the opinions.

Every command is built to be driven two ways: interactively by a human, or non-interactively by a script/agent/CI job via flags or `--json` stdin. This dual-mode design is the foundation that makes future automation possible.

## Goals (v0.1)

- One binary, three invocation modes:
  - **Interactive**: you type `forge new` and it walks you through choices with prompts.
  - **Flag-driven**: `forge new hackathon-app my-idea --with-ai --no-database` — all choices supplied as arguments.
  - **JSON-driven**: `cat vars.json | forge new hackathon-app --json` — choices read from stdin, results emitted as structured JSON.
- One concrete blueprint shipped: `hackathon-app` (Next.js + Tailwind + shadcn/ui, with optional database/auth/AI). Framework designed so additional blueprints (`go-cli`, `openclaw-skill`, etc.) can be added without touching framework code.
- Emit `.understand-anything/knowledge-graph.json` for every scaffolded project, matching Understand-Anything's schema. `forge visualize` opens that project in Understand-Anything's dashboard.
- Sub-50ms startup so agent loops calling forge repeatedly don't bog down.
- Cross-compiles to `linux/amd64` (Arch) and `darwin/arm64` / `darwin/amd64` (Mac) with a single `make` target.

## Non-goals (v0.1)

- No bundled LLM. Generation is deterministic template rendering. The intelligence comes from whoever is calling forge — you, an agent, or a script.
- No fork or vendor of Understand-Anything. We consume its JSON schema as a contract, not its code.
- No `forge upgrade` (re-render a project against a newer blueprint version). The marker file is laid down so this is possible later.
- No remote blueprint registry. `forge install <git-url>` clones blueprints into `~/.config/forge/blueprints/`; discovery is local.

---

## Architecture

Single Go module, internal packages for separation of concerns.

```
forge/
├── go.mod
├── go.sum
├── Makefile                       # build, cross-compile, test targets
├── embedded.go                    # package forge; //go:embed all:blueprints — see note below
├── cmd/
│   └── forge/
│       └── main.go                # entry: ~30 lines, calls into internal/cli
├── internal/                      # not importable by external code (Go convention)
│   ├── cli/
│   │   ├── root.go                # cobra root command + global flags
│   │   ├── new.go                 # forge new
│   │   ├── add.go                 # forge add
│   │   ├── visualize.go           # forge visualize
│   │   ├── list.go                # forge list
│   │   └── install.go             # forge install
│   ├── manifest/
│   │   ├── manifest.go            # struct definitions for manifest.yaml
│   │   └── parse.go               # load + validate from disk
│   ├── blueprint/
│   │   ├── blueprint.go           # Blueprint type + methods
│   │   ├── registry.go            # discovery: embedded, user, project-local
│   │   └── embedded.go            # //go:embed for built-in blueprints
│   ├── render/
│   │   ├── engine.go              # text/template wrapper, custom funcs
│   │   ├── values.go              # variable resolution (defaults → JSON → flags → prompts)
│   │   └── walk.go                # walk template/ tree, render each file
│   ├── graph/
│   │   ├── schema.go              # UA-compatible types (KnowledgeGraph, Node, Edge, etc.)
│   │   ├── emitter.go             # render graph.yaml → JSON
│   │   └── merge.go               # used by forge add to merge graph fragments
│   ├── hooks/
│   │   └── runner.go              # execute post_create hooks
│   ├── project/
│   │   └── marker.go              # .forge/project.json read/write
│   └── fsutil/
│       └── atomic.go              # render to tempdir, then rename into place
├── blueprints/                    # built-in, embedded via //go:embed
│   └── hackathon-app/
│       ├── manifest.yaml
│       ├── graph.yaml
│       ├── template/              # full Next.js project tree
│       │   ├── package.json.tmpl
│       │   ├── next.config.mjs.tmpl
│       │   ├── tailwind.config.ts
│       │   ├── tsconfig.json
│       │   ├── README.md.tmpl
│       │   ├── .env.example.tmpl
│       │   ├── .gitignore
│       │   ├── app/
│       │   │   ├── layout.tsx.tmpl
│       │   │   ├── page.tsx.tmpl
│       │   │   ├── globals.css
│       │   │   └── api/
│       │   │       ├── hello/
│       │   │       │   └── route.ts.tmpl
│       │   │       └── ai/
│       │   │           └── route.ts.tmpl    # only if with_ai
│       │   ├── components/
│       │   │   └── ui/                       # shadcn base components
│       │   │       ├── button.tsx
│       │   │       └── dialog.tsx
│       │   └── lib/
│       │       ├── utils.ts
│       │       ├── db.ts.tmpl                # only if with_database
│       │       └── ai.ts.tmpl                # only if with_ai
│       └── extensions/
│           ├── api-route/                    # forge add api-route <name>
│           ├── page/                         # forge add page <slug>
│           └── component/                    # forge add component <name>
└── (tests live next to code as *_test.go — Go convention)
```

**Why `internal/` instead of `pkg/`:** Go treats packages inside `internal/` as private — no other module can import them. This is deliberate. Forge isn't a library yet; its packages are implementation details. If we later want to publish a public library (say `forge-schema` for blueprint authors to validate their YAML), we'd add a `pkg/` directory.

**Why there is an `embedded.go` at the project root:** Go's `//go:embed` directive requires paths relative to the source file and forbids `..`. `blueprints/` is kept at the project root because it is first-class content that blueprint authors read and edit — burying it inside `internal/blueprint/` would make it hard to find. Since `internal/blueprint/embedded.go` cannot embed `../../blueprints`, the embed directive lives in a thin root-level file (`embedded.go`, `package forge`) that holds only `//go:embed all:blueprints` and a `BlueprintsFS() embed.FS` accessor. `internal/blueprint` imports `github.com/kanukuntla-r/forge` to call `forge.BlueprintsFS()` and sub into it. The root package has no other contents — it is a packaging workaround, not a domain package. The `all:` prefix is required so that template files beginning with `.` (e.g., `.gitignore`, `.env.example`) are included in the binary; without it, Go silently skips them.

**Key dependencies:**

| Concern | Library | Why |
|---|---|---|
| CLI parsing | `spf13/cobra` | Standard for Go CLIs (Docker, kubectl, gh). Generates `--help`, handles subcommands cleanly. |
| Templating | Standard library `text/template` | Already in Go. No extra dependency. Sufficient for our needs. |
| YAML | `gopkg.in/yaml.v3` | The de facto YAML library for Go. |
| JSON | Standard library `encoding/json` | Built in. |
| Interactive prompts | `charmbracelet/huh` | Modern, ergonomic prompt library. Handles text, select, confirm, multi-select. |
| Embedded blueprints | Standard library `embed` | Native Go feature; `//go:embed blueprints/*` bakes blueprint files into the binary. |
| Colors / styling | `charmbracelet/lipgloss` | Optional, for friendly terminal output. Defer until v0.1.5 if budget tight. |
| Errors | Standard library `errors` + `fmt.Errorf` with `%w` | Idiomatic Go error wrapping. |
| Filesystem walks | Standard library `io/fs` + `path/filepath` | Built in. |

Deliberately not pulling in fancier libraries (no logrus, no viper, no pflag — cobra brings what we need). Fewer dependencies = faster builds, smaller binary, fewer surprises.

---

## Manifest schema (`manifest.yaml`)

Every blueprint has one. This is the contract between blueprint authors and forge.

```yaml
# Identity
name: hackathon-app                    # unique blueprint id (kebab-case)
display_name: "Hackathon Web App"
description: "Next.js 14 + Tailwind + shadcn/ui starter with optional database, auth, and AI."
version: "0.1.0"
authors: ["kanukuntla-r"]

# Kind dictates graph emission conventions and target path defaults.
# One of: agent-skill | cli-tool | service | app | library | pipeline
kind: app

# Stack metadata — used by `forge list --filter <tag>`.
stack:
  language: typescript
  runtime: node
  framework: nextjs
  tags: [web, nextjs, tailwind, hackathon]

# Variables collected from the user (interactive), CLI flags, or --json stdin.
# `name` is always implicit and required — don't redeclare it unless customizing.
variables:
  - name: description
    prompt: "One-line description of the app"
    type: string
    default: "A hackathon app"

  - name: with_database
    prompt: "Include a database? (SQLite via libsql)"
    type: bool
    default: false

  - name: with_auth
    prompt: "Include authentication? (Clerk)"
    type: bool
    default: false

  - name: with_ai
    prompt: "Include AI integration? (OpenAI + Anthropic SDK)"
    type: bool
    default: true

  - name: with_dark_mode
    prompt: "Include dark mode toggle?"
    type: bool
    default: true

  - name: package_manager
    prompt: "Package manager"
    type: choice
    choices: [pnpm, npm, yarn, bun]
    default: pnpm

# Target path for the scaffolded project.
target:
  default_path: "./{{ .Name }}"

# Hooks run after files are rendered. Failures abort and clean up.
post_create:
  - name: "Initialize git"
    shell: "git init && git add -A && git commit -m 'Initial commit from forge'"
    optional: true

  - name: "Install dependencies"
    shell: "{{ .PackageManager }} install"
    optional: false                       # required: no deps = broken project

# Extensions exposed via `forge add <extension>`.
extensions:
  - name: api-route
    description: "Add a new API route under app/api/"
    template: extensions/api-route/template
    args:
      - name: route_name
        prompt: "Route name (kebab-case)"
        pattern: "^[a-z][a-z0-9-]*$"
    graph_fragment: extensions/api-route/graph-fragment.yaml

  - name: page
    description: "Add a new page under app/"
    template: extensions/page/template
    args:
      - name: slug
        prompt: "Page slug (kebab-case)"
        pattern: "^[a-z][a-z0-9-]*$"

  - name: component
    description: "Add a new React component under components/"
    template: extensions/component/template
    args:
      - name: component_name
        prompt: "Component name (PascalCase)"
        pattern: "^[A-Z][A-Za-z0-9]*$"
```

**Variable types**: `string` (default), `bool`, `int`, `choice`, `path`. `pattern` (regex) applies to `string` and `path`; `min`/`max` apply to `int`; `choices` required for `choice`.

**Templating syntax**: Go's `text/template` — uses `{{ .VarName }}` with leading dot and PascalCase. Variable names are snake_case in YAML for readability but exposed to templates in PascalCase (`with_database` → `{{ .WithDatabase }}`). The variable resolver handles this conversion.

**Conditional rendering** (resolved from question 1): files whose template content evaluates to empty are not written. The `when:` field on layers, nodes, and edges in `graph.yaml` controls graph inclusion. We do not support `{{ if ... }}` in directory or file names — keep template paths static for v0.1.

**Template file conventions**: Files in `template/` mirror the target directory structure exactly. Files with a `.tmpl` extension get rendered; the `.tmpl` is stripped from the output. Files without `.tmpl` are copied verbatim.

---

## Graph schema (`graph.yaml`)

The templated knowledge graph. Rendered at `forge new` time with the same variable context as file templates. Output is written to `.understand-anything/knowledge-graph.json`.

```yaml
project:
  name: "{{ .Name }}"
  description: "{{ .Description }}"
  languages: [typescript]
  frameworks: [nextjs, tailwind, shadcn]

# Architectural layers — color-coded groupings in the dashboard.
layers:
  - id: pages
    name: "Pages"
    description: "Route entry points (app/page.tsx, app/*/page.tsx)"
  - id: api
    name: "API Routes"
    description: "Backend endpoints (app/api/*/route.ts)"
  - id: components
    name: "Components"
    description: "Reusable UI"
  - id: lib
    name: "Library"
    description: "Helpers, clients, utilities"
  - id: config
    name: "Config"
    description: "Project configuration files"

nodes:
  - id: home-page
    type: file
    name: "Home page"
    file_path: "app/page.tsx"
    summary: "Top-level landing page for {{ .Name }}."
    layer: pages
    tags: [page, root]
    complexity: simple

  - id: root-layout
    type: file
    name: "Root layout"
    file_path: "app/layout.tsx"
    summary: "App-wide layout: html/body, font setup, providers."
    layer: pages

  - id: hello-api
    type: file
    name: "Hello API"
    file_path: "app/api/hello/route.ts"
    summary: "Example API route returning JSON."
    layer: api

  - id: ai-api
    type: file
    name: "AI API"
    file_path: "app/api/ai/route.ts"
    summary: "Calls OpenAI/Anthropic and streams a response."
    layer: api
    when: "{{ .WithAi }}"

  - id: db-lib
    type: file
    name: "Database client"
    file_path: "lib/db.ts"
    summary: "SQLite (libsql) connection and query helpers."
    layer: lib
    when: "{{ .WithDatabase }}"

  - id: ai-lib
    type: file
    name: "AI client"
    file_path: "lib/ai.ts"
    summary: "OpenAI/Anthropic SDK setup and helper functions."
    layer: lib
    when: "{{ .WithAi }}"

  - id: utils
    type: file
    name: "Utilities"
    file_path: "lib/utils.ts"
    summary: "Tailwind class merging and small helpers."
    layer: lib

  - id: package-json
    type: file
    name: "package.json"
    file_path: "package.json"
    summary: "Dependencies, scripts, and project metadata."
    layer: config

  - id: tailwind-config
    type: file
    name: "tailwind.config.ts"
    file_path: "tailwind.config.ts"
    summary: "Tailwind theme and content config."
    layer: config

edges:
  - source: root-layout
    target: home-page
    type: contains
    weight: 1.0

  - source: ai-api
    target: ai-lib
    type: imports
    when: "{{ .WithAi }}"

  - source: home-page
    target: hello-api
    type: calls
    weight: 0.7

# Optional guided tour for the dashboard.
tour:
  - order: 1
    title: "Start with app/page.tsx"
    description: "The home page is where you'll edit the main UI first."
    node_ids: [home-page]
  - order: 2
    title: "Backend endpoints live in app/api/"
    description: "Each folder under api/ with a route.ts file becomes an HTTP endpoint."
    node_ids: [hello-api]
  - order: 3
    title: "AI integration"
    description: "Call the AI route from your UI to hook up LLM features."
    node_ids: [ai-api, ai-lib]
    when: "{{ .WithAi }}"
```

**Mapping to Understand-Anything JSON**: snake_case keys become camelCase in JSON (`file_path` → `filePath`, `node_ids` → `nodeIds`). Forge-internal fields (`when`, `for_each`, `as`) are processed during emission and never appear in output.

**Schema version pinning**: `internal/graph/schema.go` defines a `UASchemaVersion` constant. If UA bumps its schema, only the emitter changes.

---

## CLI commands

### `forge new`

```
forge new <blueprint> [name] [options]

Examples:
  forge new hackathon-app my-app
  forge new hackathon-app my-app --with-ai --with-database
  forge new hackathon-app my-app --no-auth --package-manager=bun
  cat vars.json | forge new hackathon-app --json
  forge new hackathon-app --dry-run

Args:
  <blueprint>             Blueprint name (see `forge list`)
  [name]                  Project name; if omitted, prompted (or required with --yes)

Options:
  --path <dir>            Override default_path
  --json                  Read variables from stdin as JSON, emit results as JSON
  --dry-run               Render to temp, print file list, don't move into place
  --no-graph              Skip emitting knowledge-graph.json
  --no-hooks              Skip post_create hooks (skip npm install etc)
  --yes                   Accept defaults for unspecified variables (suppress prompts)
  --var KEY=VALUE         Set a variable (repeatable; e.g. --var with_ai=false)
  -q, --quiet             Suppress non-error output
  -v, --verbose           Trace every file rendered
```

Boolean variables also get auto-generated `--with-X` / `--no-X` flags from the manifest (e.g. `--with-ai`, `--no-database`).

**Variable resolution order** (last wins):
1. Defaults from `manifest.yaml`
2. `--json` stdin payload
3. `--var KEY=VALUE` flags
4. Generated boolean flags (`--with-ai`, etc.)
5. Interactive prompts (only for missing required vars; suppressed by `--yes` or `--json`)

**Output in `--json` mode** (stdout):
```json
{
  "blueprint": "hackathon-app",
  "path": "/home/user/code/my-app",
  "files_created": ["app/page.tsx", "app/layout.tsx", "package.json", "..."],
  "graph_path": ".understand-anything/knowledge-graph.json",
  "variables": {
    "name": "my-app",
    "with_ai": true,
    "with_database": false,
    "package_manager": "pnpm"
  }
}
```

### `forge add`

```
forge add <extension> [args] [options]

Examples:
  forge add api-route users
  forge add page dashboard
  forge add component UserCard

Options:
  --to <path>             Operate on a different project root (defaults to cwd)
  --no-graph              Skip graph update
  --json                  Read args from stdin, emit results as JSON
  --var KEY=VALUE         Set an extension arg
```

**Project detection**: walks up from cwd looking for `.forge/project.json`. Reads the blueprint name and version, then finds extensions defined in that blueprint's manifest.

**Graph update**: extension's `graph-fragment.yaml` rendered with the project's variable context plus extension args, merged into `knowledge-graph.json`. De-duplicate by node/edge ID; warn on collision (resolved from question 3).

### `forge visualize`

```
forge visualize [path]

Args:
  [path]                  Project directory (defaults to cwd)

Behavior:
  1. Verify .understand-anything/knowledge-graph.json exists.
     If missing, suggest re-running `forge new` or (future) `forge sync`.
  2. Resolve the visualizer (in order):
     a. `understand-anything` binary on PATH → exec with project as cwd.
     b. `~/.openclaw/skills/understand-anything` exists → invoke via openclaw.
     c. `npx @understand-anything/dashboard` available → use it.
     d. None of the above → print path to knowledge-graph.json + install instructions.
```

### `forge list`

```
forge list [--filter <kind|tag>] [--json]

Lists built-in and user-installed blueprints with their kind, description, and version.
Reads from: embedded blueprints + ~/.config/forge/blueprints/ + .forge/blueprints/.
```

### `forge install`

```
forge install <git-url> [--name <name>]

Clones a blueprint repo to ~/.config/forge/blueprints/<name>/.
Validates manifest.yaml before accepting the install.
```

---

## The `hackathon-app` blueprint (v0.1 contents)

Lives at `blueprints/hackathon-app/`, embedded into the binary via `//go:embed`.

**Files emitted** (with `with_ai=true`, `with_database=false`, `with_auth=false`, `with_dark_mode=true`):

```
{{ target }}/
├── .forge/
│   └── project.json              # marker: blueprint, version, variables, extensions applied
├── .understand-anything/
│   └── knowledge-graph.json      # emitted from graph.yaml
├── .env.example                  # placeholder for OPENAI_API_KEY, ANTHROPIC_API_KEY
├── .gitignore
├── README.md
├── package.json
├── next.config.mjs
├── tailwind.config.ts
├── tsconfig.json
├── app/
│   ├── layout.tsx                # html/body, font, theme provider (dark mode)
│   ├── page.tsx                  # landing page with "It works!" hero
│   ├── globals.css               # tailwind base + shadcn vars
│   └── api/
│       ├── hello/
│       │   └── route.ts          # GET → { ok: true }
│       └── ai/
│           └── route.ts          # POST → streams Anthropic completion
├── components/
│   ├── theme-provider.tsx        # dark mode (next-themes)
│   ├── theme-toggle.tsx          # toggle button
│   └── ui/
│       ├── button.tsx            # shadcn base
│       └── dialog.tsx            # shadcn base
└── lib/
    ├── utils.ts                  # cn() helper for class merging
    └── ai.ts                     # Anthropic client setup
```

**Key file: `app/page.tsx.tmpl`** (the landing page):

```tsx
import { ThemeToggle } from "@/components/theme-toggle"

export default function Home() {
  return (
    <main className="min-h-screen flex flex-col items-center justify-center p-8">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>
      <h1 className="text-4xl font-bold mb-4">{{ .Name }}</h1>
      <p className="text-muted-foreground mb-8">{{ .Description }}</p>
      <p className="text-sm text-muted-foreground">
        Edit <code className="bg-muted px-1 py-0.5 rounded">app/page.tsx</code> to get started.
      </p>
    </main>
  )
}
```

**Key file: `package.json.tmpl`** (dependencies vary by flags):

```json
{
  "name": "{{ .Name }}",
  "version": "0.1.0",
  "description": "{{ .Description }}",
  "private": true,
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "lint": "next lint"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "tailwindcss": "^3.4.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.3.0",
    "lucide-react": "^0.400.0"{{ if .WithDarkMode }},
    "next-themes": "^0.3.0"{{ end }}{{ if .WithAi }},
    "@anthropic-ai/sdk": "^0.24.0",
    "openai": "^4.50.0"{{ end }}{{ if .WithDatabase }},
    "@libsql/client": "^0.6.0"{{ end }}{{ if .WithAuth }},
    "@clerk/nextjs": "^5.0.0"{{ end }}
  },
  "devDependencies": {
    "typescript": "^5.4.0",
    "@types/react": "^18.3.0",
    "@types/node": "^20.12.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0"
  }
}
```

**`.forge/project.json`** (written by forge, not in `template/`):

```json
{
  "blueprint": "hackathon-app",
  "blueprint_version": "0.1.0",
  "forge_version": "0.1.0",
  "created_at": "2026-05-18T14:32:18Z",
  "variables": {
    "name": "my-app",
    "description": "A hackathon app",
    "with_database": false,
    "with_auth": false,
    "with_ai": true,
    "with_dark_mode": true,
    "package_manager": "pnpm"
  },
  "extensions_applied": []
}
```

---

## Implementation milestones

### M1 — Walking skeleton (1-2 days)

- `go mod init`, set up directory structure.
- Cobra root command + stubs for all subcommands.
- `forge --help` prints usage.
- `manifest.Parse()` loads the example `manifest.yaml` and unit test passes.
- `forge list` enumerates embedded blueprints.

### M2 — Render path (3-4 days)

- `text/template` engine wired up with helper functions (`seq`, snake-to-Pascal converter).
- `forge new hackathon-app <name>` renders the `template/` tree to a target dir.
- Interactive prompts via `huh` for unspecified required variables.
- Auto-generated `--with-X` / `--no-X` flags for boolean variables.
- Atomic writes: render to tempdir, move into place on success; clean up on failure.
- Flags wired: `--dry-run`, `--yes`, `--var`, `--path`.

### M3 — Graph emission (1-2 days)

- `graph/schema.go` types defined matching UA's JSON schema.
- `graph/emitter.go` renders `graph.yaml`, processes `when` and `for_each`, emits JSON.
- `.understand-anything/knowledge-graph.json` written alongside the scaffolded project.

### M4 — Hooks and project marker (1 day)

- `post_create` hooks execute, with `when:` and `optional:` honored.
- `.forge/project.json` written with full variable context.
- Failure paths cleaned up properly.

### M5 — Visualize and JSON mode (1 day)

- `forge visualize` resolves and execs UA per the resolution order above.
- `--json` mode on `new`: stdin parsing for variables, structured stdout.

### M6 — `forge add` (2 days)

- Project detection (walk up from cwd).
- Extension args collected (interactive or via flags/JSON).
- Files rendered into existing project.
- Graph fragment merged into existing `knowledge-graph.json` (de-dupe by ID).
- `.forge/project.json` updated with new entry in `extensions_applied`.

### M7 — Install + polish (1 day)

- `forge install <git-url>` clones into `~/.config/forge/blueprints/`.
- Manifest validation with friendly errors.
- README, install script for Arch + Mac.

**Total**: ~12 working days for v0.1. M2 grows because the hackathon-app template has more files than the openclaw-skill version would have.

After v0.1 lands, second blueprint is `go-cli` (dogfood) or `openclaw-skill` (your original goal). Authoring the second blueprint stress-tests the schema.

---

## Resolved questions (from previous design rounds)

1. **Conditional file rendering**: manifest-level `when:` only for v0.1. Path-level conditionals deferred.
2. **Variable validation**: single validator, called from all input paths (prompts, flags, JSON). Fail loud with helpful messages.
3. **Graph fragment merging**: de-duplicate by ID, warn on collision.
4. **OpenClaw skill registration**: not applicable for v0.1 (no openclaw-skill blueprint shipping). When that blueprint is built later, the registration mechanism gets verified at that time.

---

## Handoff notes for Claude Code

When you start implementation:

1. Create the directory structure and `go.mod` first. Get `go run ./cmd/forge --help` working.
2. Implement `internal/manifest/` and `internal/blueprint/` before any rendering. Write table-driven tests for manifest parsing on the example YAML in this doc.
3. Use the `hackathon-app` blueprint as the only test fixture for M1–M5. Don't generalize the framework until M6 forces it.
4. After M5, run `forge new hackathon-app test-app --path ./tmp/test-app --with-ai` against a scratch directory, then `cd ./tmp/test-app && pnpm install && pnpm dev` to verify the scaffolded project actually runs. That's the v0.1 demo.
5. Commit `blueprints/hackathon-app/` to the repo; it's embedded into the binary via `//go:embed blueprints/*`.
6. Match the UA JSON schema exactly — the dashboard is unforgiving about missing fields.
7. Standard Go conventions throughout: `gofmt`, `golangci-lint`, table-driven tests, error wrapping with `%w`, no panics in library code, contexts for anything that could block.
