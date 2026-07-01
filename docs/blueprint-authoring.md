# Blueprint Authoring Guide

This document covers concepts that aren't obvious from reading `manifest.yaml` schema alone.
The most common surprise is the **two-level scaffolding constraint**, which affects any blueprint
that itself scaffolds a blueprint (meta-blueprints like `blueprint-starter`).

---

## Two-level scaffolding: the core constraint

When `forge new blueprint-starter my-bp` runs, forge's render engine walks every file under
`blueprints/blueprint-starter/template/` and:

1. Renders path segments as Go templates (so `{{ .RouteName }}/route.ts.tmpl` becomes `users/route.ts.tmpl`).
2. Renders the **content** of every `.tmpl` file using the **blueprint-starter variable context**
   (`Name`, `Description`, `Author`).

This means that any `{{ }}` expression inside blueprint-starter's template files is evaluated
**now**, against blueprint-starter's variables — not later when the user runs *their* blueprint.

### What breaks

Suppose blueprint-starter's `template/template/index.html.tmpl` contains:

```html
<title>{{ .ProjectName }}</title>
```

Blueprint-starter's render engine tries to expand `{{ .ProjectName }}`. `ProjectName` is not a
blueprint-starter variable → `missingkey=error` → **the scaffolding fails**.

The same applies to path-segment templates. If the extension template directory contains a file
named `{{ .PageName }}.html.tmpl`, forge tries to render `{{ .PageName }}` as a path segment
during the blueprint-starter scaffolding pass — again failing because `PageName` is not in scope.

### Why blueprint-starter uses static content

`blueprint-starter`'s inner template files (`index.html`, `style.css`, `app.js`, `page.html`) are
plain files with no `{{ }}` expressions. Forge copies them verbatim into the user's blueprint
directory. This avoids the two-level conflict entirely at the cost of a hardcoded demo title.

Users who want their blueprint to produce customized output (e.g. a title that uses `project_name`)
can rename those files to `.tmpl` and add the expressions **after** running blueprint-starter —
the README inside the scaffolded blueprint directory explains how.

---

## Workaround: the double-extension trick

If you need to produce a `.tmpl` file in the output that contains `{{ }}` expressions, you can:

1. Name the source file with a **double extension**: `page.html.tmpl.tmpl`
2. Escape inner template expressions using `{{ "{{" }}` and `{{ "}}" }}`:

```
<!-- page.html.tmpl.tmpl (inside blueprint-starter/template/) -->
<h1>{{ "{{" }}.PageName{{ "}}" }}</h1>
```

When blueprint-starter's pass processes this file:
- The outer `.tmpl` suffix triggers rendering; the inner suffix is stripped → output: `page.html.tmpl`
- `{{ "{{" }}` renders to `{{`; `{{ "}}" }}` renders to `}}`
- The resulting `page.html.tmpl` contains `{{.PageName}}` literally

When the user's blueprint later renders `page.html.tmpl`, `{{.PageName}}` expands correctly.

**Trade-off**: the double extension and escaping syntax are confusing to read. Reserve this
technique for cases where the end-to-end variable substitution is essential.

---

## Dynamic extension filenames

The same constraint blocks dynamic output filenames from extensions in a meta-blueprint.
To produce `about.html` from `forge add page`, the extension template directory needs a file
whose path renders to `about.html` — e.g. `{{ .PageName }}.html.tmpl`. But blueprint-starter
would try to render that path segment during its own scaffolding pass.

**Current status**: blueprint-starter's page extension always creates `page.html`. Dynamic
filenames are tracked as a future engine feature (see `NOTES.md`).

**If you need it now**: after running blueprint-starter, manually rename the template file:

```bash
cd my-blueprint/extensions/page/template
mv page.html '{{ .PageName }}.html.tmpl'
# add {{.PageName}} to the file content as well
```

Then commit. Forge's file walker handles the `{{ }}` in directory/file names on Linux/macOS.

---

## Variable context in templates

Blueprint variables are resolved to a `map[string]any` with **PascalCase keys** before rendering.
`project_name` → `ProjectName`, `with_ai` → `WithAi`. The project name is always available as
`{{ .Name }}` regardless of whether the manifest declares a `name` variable.

## Graph templates

`graph.yaml` at the blueprint root is rendered separately by `graph.Render` using the same
PascalCase context. It is NOT processed by the template file walker. This means `graph.yaml`
can safely contain `{{ .Name }}` even inside a meta-blueprint's `template/` directory — it is
copied verbatim during blueprint-starter's scaffolding pass and only rendered when the user's
blueprint runs `forge new`.

## Extensions and graph fragments

Extension graph fragments (`graph-fragment.yaml`) are read and rendered by `forge add` at
extension-apply time, not during the initial scaffolding. This means `{{ .PageName }}` in a
graph fragment file is safe inside a meta-blueprint's `template/` directory — it is copied
verbatim and only expanded when the user runs `forge add page`.
