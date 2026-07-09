# forge — development notes

Informal tracking for items that don't fit neatly into a commit or milestone doc.
Items here are candidates for future milestones; delete when picked up.

---

## Future work

### Path-segment templating in meta-blueprints (v0.5 candidate)

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
path-segment expansion. Possible approaches:
- `--raw` / `raw:` prefix on a template directory path in the manifest to skip path rendering
- A `{{ }}` escape syntax for path segments (e.g. double-braces `{{{ .PageName }}}`)
- A `defer_path_render: true` flag per extension in the manifest

**References**: `docs/blueprint-authoring.md` §Dynamic extension filenames, `blueprints/blueprint-starter/template/extensions/page/template/page.html` (comment block)

**Milestone**: v0.5 blueprint authoring improvements

---

### Client-variable tracking for HTTP call detection (v0.5 candidate)

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

**Milestone**: v0.5 candidate

---

### Next.js detector monorepo support (v0.5 candidate)

**Problem**: The Next.js framework detector assumes the project root is also the Next.js
root. Monorepos that nest the Next.js app under `frontend/`, `web/`, or `client/` aren't
detected.

**Milestone**: v0.5 candidate

---

### More framework detectors (v0.5 candidate)

- TypeScript side: Astro, Remix, SvelteKit, Vue
- Python side: Django, Flask

---

### Additional blueprints

- `astro-site` — Astro + content collections (framework detector already handles routes)
- `openclaw-skill` — forge skill for the OpenClaw agent framework

---

### Caching with content hashes (v0.5 candidate)

Blueprint template files are re-read and re-rendered on every scaffold, and analyzer runs
re-parse every file on every `forge visualize` refresh. For large projects or agent loops
calling forge repeatedly, a content-hash cache on rendered/parsed output would reduce disk
I/O and CPU. Deferred from v0.3; still not started.
