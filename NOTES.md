# forge — roadmap

Informal tracking for items that don't fit neatly into a commit or milestone doc.

---

## Completed in v0.4

- Python file parsing via tree-sitter-python
- Import extraction (all forms including relative, star, aliased)
- Declaration extraction (functions, classes, module variables with decorators)
- FastAPI framework detector with prefix resolution and reachability tracking
- Python HTTP call detection (requests, httpx, urllib) with route matching
- Cross-language route matching — TypeScript pages calling Python backends and vice versa
- Dashboard visibility for FastAPI (orange router nodes, both frameworks render together)
- Improved graph layout (cose-bilkent, better spacing for larger projects)

---

## Completed in v0.5

- Database detection (Prisma + Drizzle) ✓
- Query call detection ✓
- Dashboard visibility for databases (yellow table nodes, gold query edges, filter toggles)
- Header badge shows detected databases alongside frameworks
- Theme switcher (Light/Dark/Auto) ✓

---

## Deferred to v0.5.1

### Cross-language route matching for queries

TypeScript query calls to Python DB queries (e.g. a Next.js page calling a FastAPI route that
issues a SQLAlchemy query) aren't correlated into a single graph edge — the TS→Python route
match and the Python-side DB query are both detected independently but not chained together.

### SQLAlchemy detector (Python)

Mirror the Prisma/Drizzle detectors (`internal/analyzer/prisma.go`, `internal/analyzer/drizzle.go`)
for SQLAlchemy: schema parsing (declarative models), session/query call detection.

### Supabase JS client detector

Detect `supabase.from(table).select()/.insert()/.update()` call patterns as database queries,
similar to the Prisma/Drizzle method-chain detection.

### Firebase client detector

Detect Firestore/Realtime Database client calls (`db.collection(...).get()`, etc.) as database
queries.

### Utility file query source nodes

Queries issued from files that aren't pages/routes/handlers (e.g. a plain `lib/db-helpers.ts`)
currently have no graph node to attach to, so `findQuerySourceNode` (`internal/dashboard/static/app.js`)
drops the edge rather than inventing a generic "file" node type. Add a node type for these.

### Foreign key relationships in database schema

`DatabaseTable` (`internal/analyzer/database.go`) has no relationship/foreign-key field yet —
schema parsing only extracts table names and (for some detectors) fields, not cross-table
references.

### Table field/column detection

Database tables currently only carry a name — column/field names and types aren't extracted
from Prisma/Drizzle schemas yet.

### Client-variable tracking for HTTP call detection

**Problem**: M10.5's Python HTTP call detector only recognizes direct calls like
`requests.get(...)` or `httpx.get(...)`. It doesn't track client instances assigned to
variables, so patterns like:

```python
client = httpx.AsyncClient()
await client.get("/users")
```

or nested context-manager forms:

```python
async with httpx.AsyncClient() as client:
    await client.get("/users")
```

aren't detected, since `client.get(...)` doesn't syntactically reference `httpx` or `requests`
directly.

**Scope**:
- Module-level and function-scope client variable tracking
- `async with X() as client:` binding
- Deferred from M10.5 to keep the first HTTP-call-detection milestone scoped to the common
  direct-call case

---

## Deferred to v0.5.2+

### Path-segment templating in meta-blueprints (deferred)

**Problem**: When a blueprint produces another blueprint (meta-blueprint), the render engine
processes ALL `{{ }}` expressions in template file paths and `.tmpl` file contents during the
outer scaffolding pass. This means extension templates with dynamic output filenames (e.g.
`{{ .PageName }}.html.tmpl`) cannot live inside a meta-blueprint's `template/` directory —
they would be evaluated with the outer blueprint's variable context and fail.

**Concrete symptom**: `blueprint-starter`'s page extension always creates `page.html` instead
of the user-supplied page name (e.g. `about.html`), because the path-segment template
`{{ .PageName }}.html.tmpl` would be rendered during `forge new blueprint-starter` rather than
during `forge add page`.

**Proposed fix**: Add a render-engine option (or a reserved filename pattern) that defers
path-segment expansion, with a skip-unknown-vars mode so unresolved `{{ }}` expressions in a
path segment are left as-is rather than erroring. Possible approaches:
- `--raw` / `raw:` prefix on a template directory path in the manifest to skip path rendering
- A `{{ }}` escape syntax for path segments (e.g. double-braces `{{{ .PageName }}}`)
- A `defer_path_render: true` flag per extension in the manifest

**References**: `docs/blueprint-authoring.md` §Dynamic extension filenames, `blueprints/blueprint-starter/template/extensions/page/template/page.html` (comment block)

### Next.js detector monorepo support (still deferred)

**Problem**: The Next.js framework detector assumes the project root is also the Next.js
root. Monorepos that nest the Next.js app under `frontend/`, `web/`, or `client/` aren't
detected.

### More framework detectors

- TypeScript side: Astro, Remix, SvelteKit, Vue
- Python side: Django, Flask

### Caching with content hashes

Blueprint template files are re-read and re-rendered on every scaffold, and analyzer runs
re-parse every file on every `forge visualize` refresh. For large projects or agent loops
calling forge repeatedly, a content-hash cache on rendered/parsed output would reduce disk
I/O and CPU. Deferred from v0.3; still not started.

---

## Deferred to v0.6+

- `astro-site` blueprint — Astro + content collections (framework detector already handles routes)
- `openclaw-skill` blueprint — forge skill for the OpenClaw agent framework
