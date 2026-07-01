# blueprint-starter

A meta-blueprint that scaffolds the structure for creating new forge blueprints.

## Usage

```bash
forge new blueprint-starter my-blueprint
```

Produces a working blueprint directory `my-blueprint/` containing:

| File / Dir | Purpose |
|---|---|
| `manifest.yaml` | Blueprint configuration (variables, hooks, extensions) |
| `graph.yaml` | Knowledge graph for projects scaffolded from this blueprint |
| `README.md` | Blueprint authoring guide |
| `template/` | Files rendered into the user's project on `forge new` |
| `extensions/page/` | Example extension — adds a new HTML page |

The bundled demo produces a minimal note-taking app: HTML form, localStorage persistence, minimalist CSS.

## Testing your scaffolded blueprint

The fastest path is the Go test suite — no git remote required:

```bash
go test ./internal/blueprint/... -run TestBlueprintStarter -v
```

To test manually end-to-end, push to a git remote first, then install:

```bash
cd my-blueprint
git init && git add -A && git commit -m "init"
# push to GitHub / any git host, then:
forge install <git-url>
forge new my-blueprint my-notes-app --yes --no-hooks
open my-notes-app/index.html
```

## Design note: two-level scaffolding

blueprint-starter's template files (`index.html`, `style.css`, `app.js`, `page.html`) are
static — they contain no `{{.Variable}}` expressions. This is because forge's render engine
processes every `.tmpl` file in `template/` when blueprint-starter runs, using blueprint-starter's
own variable context. A template expression meant for the *next* scaffolding level (e.g.
`{{.ProjectName}}` for the note-app blueprint) would fail at the blueprint-starter level.

See `docs/blueprint-authoring.md` for the full explanation and available workarounds.
