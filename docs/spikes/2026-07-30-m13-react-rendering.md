# M13.0 Spike: React Component Rendering Approaches

**Date:** 2026-07-30
**Status:** Complete
**Time spent:** ~1 day (came in under the 2-3 day budget — Playwright worked on the first real attempt, which cut short the need for extended troubleshooting on the other two)

## Question

Can forge take a `.tsx` file it has already analyzed and produce actual visual output (screenshot/SVG) for it, so v0.6 can show pages "Figma-style" instead of just a component tree? Three approaches were prototyped against three fixture components (plain inline styles, Tailwind classes, nested child components). Prototype code lives in `spike/m13.0-react-render/` (gitignored, not committed — throwaway per the spike's own rules).

## Verdict table

| Approach | Rendered all 3 fixtures? | Feasibility | Prod time estimate | Biggest blocker |
|---|---|---|---|---|
| **1. Playwright (headless Chromium)** | Yes — all 3, first try | **High** | 1-2 weeks | Runtime dependency size (see below) |
| **2. React DOM SSR + JSDOM** | HTML only; zero real layout on all 3 | **Low** | N/A — dead end without adding a real layout engine | JSDOM has no layout engine at all, not a tuning problem |
| **3. RSC + Next.js pipeline** | HTML only; same layout gap as #2, plus framework lock-in | **Low-Medium** | 2-3 weeks, and only covers Next.js projects | Still needs a browser downstream for actual pixels; adds Next.js version/toolchain fragility on top |

**Recommendation: Approach 1 (Playwright) for M13.1.**

## What we learned about React outside a browser

**Rendering React to a string is the easy 10%.** Both `react-dom/server`'s `renderToStaticMarkup` and Next's RSC pipeline happily turned all three fixture components into correct HTML — including the nested-component case (`Form` → `Button` → `Button`), which just recurses through function components with no surprises. This part of "render React outside a browser" is a solved problem and has been for years.

**Layout is the other 90%, and nothing here does it except a real browser.** This was the central finding of the spike. `getBoundingClientRect()` on any element in JSDOM returns all zeros — `{x:0, y:0, width:0, height:0, ...}` — for every fixture, including the plain inline-style one. JSDOM parses CSS text and will hand back individual computed *property values* (e.g. it correctly reported `padding: 20px` for inline styles), but it has no box model, no flow algorithm, no flexbox/grid engine — there's nothing computing where a box actually sits or how big it actually is. This isn't a missing feature we could work around with more JSDOM configuration; JSDOM is explicitly documented as not implementing layout, and no amount of extra CSS handed to it changes that. For the Tailwind fixture, JSDOM couldn't resolve the utility classes to *any* values at all (empty string for padding/border) because we never loaded a real stylesheet — but even a computed style is not the same as a laid-out box.

**RSC doesn't change this calculus — it just moves the same wall one layer over.** Next's `next build` prerenders to static HTML the same way `renderToStaticMarkup` does; the class-based Tailwind fixture came out with unresolved `class="p-5 border ..."` attributes and no computed geometry, because we never wired up a real Tailwind/PostCSS build (that's an orthogonal, solvable problem, but it's *additional* work RSC doesn't remove). RSC's actual contribution — server vs. client component boundaries — is invisible for all three fixtures because none of them are RSC-specific; they're plain components that render the same whether or not a "use client" pragma exists. So RSC bought nothing for the layout problem and cost real setup pain (see blockers below).

**Only a real browser engine computes real layout.** This is why Playwright — which drives actual Chromium — is the only approach that produced meaningful pixels. `page.locator('#root').screenshot()` captured properly laid-out, padded, bordered, styled elements on the first attempt for all three fixtures, no layout approximation code needed, because Chromium's layout engine did the work forge would otherwise have to reimplement.

## Approach 1: Playwright — details

**What worked:** esbuild bundled each `.tsx` fixture (React + ReactDOM + component, dev build) into a single self-contained HTML file in well under a second per component. Playwright loaded that file via `file://`, waited for React to mount, and screenshotted the root element. All three fixtures — plain styles, Tailwind (via the Tailwind CDN script, no real build step), and nested child components — rendered correctly and visually matched expectations.

**What didn't need solving:** no layout approximation, no CSS engine, no manual box-model math. Chromium did all of it.

**Blocker — runtime dependency size (the concern you flagged):** the actual measured footprint is larger than the ~150MB estimate:

- `chromium-1234/`: 389 MB
- `chromium_headless_shell-1234/`: 262 MB
- `ffmpeg-1011/`: 4.9 MB
- **Total Playwright browser cache: 656 MB** (this system isn't in Playwright's list of officially supported OSes, which is part of why it pulled both a full Chromium *and* a headless-shell fallback build — a supported OS would likely only need one, closer to 260-390 MB)
- `node_modules` for the Playwright toolchain itself (playwright, esbuild, react, react-dom, jsdom): 52 MB, separate from the browser cache

This is a real productionization question for M13.1, not a blocker for the spike's core question. Options, roughly in order of how much they fit forge's "single static Go binary, no phone-home, nothing global" philosophy:

1. **Require a separate `forge visualize --install-browser` step** (or reuse an existing system/user Chrome/Chromium if present) rather than bundling the browser at all. Keeps the forge binary itself small and matches how Playwright/Puppeteer are normally distributed. Visualization becomes an opt-in, not something every `forge new` run pays for.
2. **Graceful degradation**: if no browser is available and the user hasn't opted in, fall back to the existing non-visual dashboard (component tree, no screenshots) rather than failing the command.
3. **Bundle it** — rejected as a default; 300-650 MB is disproportionate to forge's current footprint and conflicts with "does not modify global system state" / small-binary conventions already in CLAUDE.md.

## Approach 2: React DOM SSR + JSDOM — details

`renderToStaticMarkup` produced correct HTML for all three fixtures with zero issues (including the nested `Button` components). The dead end is entirely on the JSDOM side: `getBoundingClientRect()` returns all-zero geometry unconditionally, because JSDOM implements the DOM and CSSOM but not layout. This is a structural limitation, not a configuration gap — closing it would mean embedding an actual CSS layout engine (something like Chromium's layout code, or a from-scratch flexbox/block-layout implementation), which is a multi-month undertaking in its own right and still wouldn't handle arbitrary CSS the way a real browser does. Not worth pursuing further; noted here for the record since the spike explicitly asked us to test it to disprove or confirm this exact assumption.

## Approach 3: RSC + Next.js pipeline — details

Cost more setup time than the other two combined, for a result no better than Approach 2:

- Next.js 14.2 crashed the build worker outright on this system's Node version (26.4.0) with an internal `"id" argument must be of type string`  error — a Next/Node compatibility issue, not anything in our fixture code.
- Next.js 15.5 fixed that, but then rejected the auto-installed TypeScript 7 (`typescript@7.0.2 is not supported`), requiring a manual pin to `typescript@^5.6.3`.
- Once building, it correctly prerendered all three fixtures to static HTML (including the nested-component case) — but with the identical missing-geometry problem as Approach 2, since prerendering is still just HTML generation. Getting pixels back out would still require loading that HTML in a browser, i.e. Playwright again, just with Next.js's build toolchain (`rsc-approach/node_modules` alone: 350 MB) sitting in front of it for no rendering benefit.
- It's also the narrowest approach: only applies to Next.js App Router projects, while forge scaffolds multiple blueprint types (Go CLI, Python/FastAPI, etc.) that have no RSC pipeline at all.

Net: RSC adds toolchain fragility and framework lock-in without solving the actual hard problem (layout), so it doesn't earn its complexity for M13.1.

## Recommended approach for M13.1

**Playwright + esbuild bundling**, matching the spike prototype's shape:

1. For each page/component forge's existing analyzer has already found, generate a minimal entry module (`createRoot(...).render(<Component/>)`).
2. Bundle it with esbuild (already fast, zero-config for this use case) into a self-contained HTML file — no need for a real webpack/vite project per component.
3. Load it in headless Chromium via Playwright, screenshot at one or more viewport sizes.
4. Cache output keyed by content hash (this is already on the v0.5.2+ roadmap for other reasons — reuse the same mechanism).

## Rough production architecture sketch

```
forge analyze (existing)
     │  produces: component graph, JSX tree, per-component import list
     ▼
forge visualize --render (new, M13.1)
     │
     ├─ for each renderable component/page:
     │     ├─ codegen a small entry file (imports the component, calls createRoot().render())
     │     ├─ esbuild.build() → single HTML file, react+react-dom inlined
     │     └─ Playwright: page.goto(file://...) → screenshot → write PNG/SVG
     │
     └─ dashboard (existing Understand-Anything-compatible JSON) gains an
        optional `screenshot: "path/to.png"` field per node, consumed by
        the frontend to render a thumbnail instead of/alongside the node shape
```

Open questions to resolve as part of M13.1 planning, not this spike:
- How does forge supply mock/default values for components that need props, context providers, or data fetching to render meaningfully? (All three spike fixtures were prop-free by design.)
- Browser install/availability strategy (see the dependency-size discussion above) — this is the one architectural decision that should be made *before* writing M13.1 code, since it changes whether visualization is bundled-by-default or opt-in.
- Real Tailwind/CSS-framework builds (the spike used the Tailwind CDN script as a shortcut; a production version needs to run the project's actual PostCSS/Tailwind config, or fall back to unstyled-but-structurally-correct output when it can't).

## Fallback options (not needed — included for completeness per the spike brief)

Since Playwright cleared the bar cleanly on all three fixtures, none of these are necessary for M13.1. Recorded in case the prop/context/data-fetching open question above turns out to be a bigger wall than expected:

- **Storybook integration**: if forge could detect and drive an existing Storybook config, it'd inherit that project's existing mock data / decorators — solves the "component needs props" problem for free, but only for projects that already use Storybook.
- **Screenshot-only (no forge-driven rendering)**: let the user supply their own screenshots (e.g., from their running dev server) and have forge just attach them to the graph. Zero rendering-correctness risk, but loses the "automatic, day one" promise.
- **Component-tree-only (status quo)**: ship v0.6 without visual thumbnails, keep the existing graph-only dashboard. Always available as the floor.
