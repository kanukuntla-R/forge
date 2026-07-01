# forge — development notes

Informal tracking for items that don't fit neatly into a commit or milestone doc.
Items here are candidates for future milestones; delete when picked up.

---

## Future work

### Path-segment templating in meta-blueprints (v0.4+ candidate)

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

**Milestone**: v0.4 blueprint authoring improvements

---

### Additional blueprints (v0.3 / v0.4)

- `go-cli` — standard Go CLI with Cobra, goreleaser config, testable `RunE` pattern
- `astro-site` — Astro + content collections (framework detector already handles routes)
- `openclaw-skill` — forge skill for the OpenClaw agent framework

---

### Caching with content hashes (v0.3)

Blueprint template files are re-read and re-rendered on every scaffold. For large blueprints
or agent loops calling forge repeatedly, a content-hash cache on the rendered output would
reduce disk I/O. Planned for v0.3.
