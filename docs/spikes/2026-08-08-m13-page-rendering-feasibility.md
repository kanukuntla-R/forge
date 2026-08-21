# M13.0.5 Spike: Can Forge Render Actual Next.js Pages?

**Date:** 2026-08-08
**Status:** Complete
**Time spent:** ~1 day (well under the 3-4 day budget — the simple case rendered within the first hour once a self-inflicted path-alias config bug was fixed; medium/hard cases needed one additional technique, not new tooling)

> Note on the brief: the brief named this file `2026-07-30-m13-page-rendering-feasibility.md`. That date is M13.0's date, not this spike's — dated `2026-08-08` here (today) instead. Flagging rather than silently following, per CLAUDE.md's "surface it as a question" rule for anything that looks like a doc/spec inconsistency.

## Question

M13.0 proved Playwright + esbuild can render an isolated `.tsx` component with synthetic props. This spike asks the harder question: can that same pipeline take an **actual page file from a real Next.js project** — with its real imports, real (async) data fetching, real ORM calls — and produce a rendered visual, without running Next's dev server or build pipeline?

## Verdict

**Full success.** All three `demo-shop2` test cases — byte-identical copies of the real page files, no source edits — rendered correctly on Playwright/esbuild with only two additions beyond M13.0's approach: (1) pointing esbuild at the project's real `tsconfig.json` for `@/*` path resolution, and (2) swapping the two data-access modules (`@/lib/prisma`, `@/db`) for fixture-returning stubs via esbuild's `alias` option. No page source was modified, no Next.js/Turbopack internals were needed, and the "server component" problem turned out not to require solving at all — see below.

## Per-test-case results

Rubric: compile (TS/JSX syntax transform) → bundle (import resolution) → execute (bundle runs without throwing) → render (expected content appears in the DOM) → production-close (qualitative: how faithful is this to what `next start` would actually show).

| Test case | Compile | Bundle | Execute | Render | Production-close |
|---|---|---|---|---|---|
| **Simple** — `app/page.tsx` (no data fetching) | ✅ | ✅ | ✅ | ✅ — all expected text found | High. No CSS or root layout exists anywhere in `demo-shop2` (confirmed by search — the fixture project has neither), so there's nothing this approach fails to reproduce. |
| **Medium** — `app/users/page.tsx` (async, Prisma) | ✅ | ✅ (after aliasing `@/lib/prisma`) | ✅ | ✅ — all expected text found, including the `name ?? email` null-fallback path (fixture user "carol" has `name: null`) | High for structure/logic; data is obviously synthetic (3 hand-written fixture rows) rather than a real DB snapshot. |
| **Hard** — `app/posts/[id]/page.tsx` (async, Prisma **+** Drizzle, dynamic route param) | ✅ | ✅ (after aliasing `@/lib/prisma` **and** `@/db`) | ✅ | ✅ — all expected text found (post title/body from Prisma, comments from Drizzle) | High. `params.id` was supplied by hand (`{ id: "1" }`) since there's no real router — fine for a single-page preview, would need a value source if forge ever wants to render a dynamic route's several possible instances. |

Screenshots: `spike/m13-0-5-page-rendering/output/{simple,medium,hard}.png` (gitignored, not committed — throwaway per spike convention, same as M13.0).

### Supplementary: `next/link`, `next/image`, `next/navigation`

None of the three real `demo-shop2` fixtures import from `next/link`, `next/image`, or `next/navigation` — `Header.tsx` uses plain `<a>` tags, and nothing in this fixture set uses client-side routing or images. Rather than report that axis as simply "untested," I added three small synthetic single-import fixtures (`spike/m13-0-5-page-rendering/fixtures/synthetic-*-only.tsx`, clearly marked as not from `demo-shop2`) to characterize it directly:

| Import | Result | Detail |
|---|---|---|
| `next/link` | ✅ Renders | Degrades to a plain `<a href="/users">Users</a>` — no client-side navigation (nothing to navigate to outside a real router), but it doesn't crash, and the `href` is correct. |
| `next/navigation` (`usePathname`) | ✅ Renders | Returns `undefined` silently (no provider context, no throw) — renders `<p></p>` instead of `<p>/some/path</p>`. Doesn't block rendering, but the value is wrong/blank without extra mocking. |
| `next/image` | ❌ Blocks | `Element type is invalid: expected a string ... but got: object` — React rejects `Image` as a JSX tag entirely. Bundling the raw `next/image` export outside Next's own build pipeline (which normally rewrites/wraps it via a webpack/SWC loader) produces something React can't use as-is. |

One more blocker surfaced and was cleared along the way: every fixture that pulled in any `next/*` module crashed at execute-time with `process is not defined` (Next's internals reference `process.env` at module-load time). Fixed with an esbuild `banner` injecting `globalThis.process = { env: { NODE_ENV: 'development' } }` — note that this had to be a `banner` (raw text prepended to the compiled output) and not a `globalThis.process = ...` statement written before the imports in the entry file source, because ES module imports execute before any of the importing module's own top-level statements regardless of source order; a same-file shim silently never ran.

Per your instruction to only escalate to Turbopack/Next-internals if Approach 1 hits a wall it could plausibly fix: `next/image`'s failure is exactly that kind of wall (Next's own image-loader machinery is the natural fix), but it's out of scope for this spike's fixtures — none of the three real pages use it. Flagging as a targeted follow-up if `next/image` usage turns out to matter for whatever project forge visualizes next, not pursuing further here.

### Server vs. client components (separate axis, per your instruction)

All three real fixtures are Server Components by default (App Router, no `"use client"` directive anywhere in `demo-shop2`) — two are `async function`s doing data fetching, one is a plain sync function. There are **no genuine Client Components** in this fixture set (nothing with `"use client"` + `useState`/`useEffect`), so this spike doesn't exercise a page that mixes both.

The technique that made async Server Components a non-issue: **call the page's exported function directly and hand its return value to `createRoot().render()`**, rather than trying to render `<Page/>` as a JSX node:

```js
const el = await Page(props)          // Page is `export default async function Page(...)`
createRoot(document.getElementById('root')).render(el)
```

This works because a Server Component's *definition* is nothing more than a function — sync or async — that returns JSX. The RSC "magic" (payload serialization, streaming) is a property of Next's server request pipeline actually invoking that function through its own machinery; it's not a property of the function itself. Calling it directly in a Playwright-controlled page sidesteps that machinery entirely rather than needing to reimplement or fake it. Per the original brief's Approach 4 concern ("server components can't become client components") — correct, and irrelevant: nothing here converts a server component into a client component, it just calls a function and mounts its result.

Open question this spike does **not** answer: what happens when a Server Component page renders a *nested* Client Component (a child with `"use client"` + interactivity)? M13.0 already confirmed plain interactive React components render fine standalone via this same Playwright/esbuild pipeline, so the individual pieces are known to work — but a page that hands off from a manually-awaited server function into a real client-boundary child, inside one bundle, hasn't been tested. Flagging for M13.1 if forge encounters a project using that pattern.

## Technical blockers (summary)

1. **esbuild `tsconfig` path-alias resolution requires exact directory mirroring.** `baseUrl: "."` in a copied `tsconfig.json` resolves relative to wherever that file sits — it has to live at the same relative position to the fixture files as it does in the source project, or `@/*` imports silently fail to resolve. (Self-inflicted on the first run here; not a real barrier once understood — but worth documenting since it'll bite again the first time forge builds this into a real pipeline.)
2. **`process is not defined`** the moment any `next/*` module is bundled for browser — fixable with an esbuild `banner` shim, not fixable with `define` alone or a same-file source-order shim.
3. **`next/image` doesn't survive raw bundling** — the one real, unresolved blocker found. Everything else in the `next/*` surface tested here (`Link`, `usePathname`) degrades gracefully instead of crashing.
4. **Mocking boundary that actually worked:** only the two project-specific data-access wrapper modules (`@/lib/prisma`, `@/db` — ~15-30 lines each), aliased via esbuild, not the underlying ORM packages (`@prisma/client`, `drizzle-orm`, `postgres`) and not any component/JSX code. This answers the brief's "how much do we mock before the page becomes meaningless" question concretely: for these fixtures, mocking stopped at the I/O boundary and never had to touch anything a human would recognize as "the page."

## Recommendation for M13.1

**Pursue Approach 1** (esbuild + Playwright, exactly as M13.0 already recommended) — this spike found no reason to reach for Turbopack/Next-internals or heavier mocking. Concretely, M13.1's page-rendering path needs:

- Resolve the project's real `tsconfig.json` (`paths`/`baseUrl`) when bundling, not a synthesized one.
- For each page module, detect its data-access imports (forge's existing Prisma/Drizzle detectors from v0.5 already know which files these are) and alias them to a generated stub returning either detector-derived shape-correct fixture data or empty arrays, rather than attempting a real DB connection.
- For an async page export, `await` it directly in the generated entry file instead of rendering it as a JSX element — no Suspense boundary or server/client conversion needed.
- For dynamic routes (`[id]`, etc.), supply a placeholder param value (forge already has nothing better to go on without a real request) — good enough for a single preview render; revisit if forge ever wants to preview multiple instances of one dynamic route.
- Treat `next/image` as a known gap: either strip/no-op it via an esbuild plugin (render nothing, or a placeholder box, in its place) for M13.1, or scope a small follow-up spike into how Next's own image loader could be reused, if `next/image` usage turns out to be common in projects forge visualizes.

This is a smaller lift than M13.0's own architecture sketch anticipated for the "how does forge supply data" open question — mocking two small files per project, keyed off detectors forge already has, is enough.

## Fallback options (not needed — included for completeness)

Not required: Approach 1 cleared all three real fixtures cleanly. Recorded per the brief in case a future project's page-rendering hits a wall these fixtures didn't:

- **Component-tree-only (status quo):** M13.0's already-recommended floor — ship without page-level screenshots, keep the graph-only dashboard.
- **Aggressive `next/image` stripping:** if a project's pages depend heavily on `next/image` and the loader issue proves not worth chasing, an esbuild plugin that replaces `next/image` imports with a plain `<img>` shim (dropping Next's optimization/loader behavior but keeping *a* rendered box) would unblock rendering at the cost of layout accuracy for image-heavy pages.
- **Screenshot-only (user-supplied):** unchanged from M13.0 — always available, zero rendering-correctness risk, loses "automatic, day one."
