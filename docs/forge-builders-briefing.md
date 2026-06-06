# forge — Builder's Briefing

What you should know before we start writing code. Read this once, refer back when you hit unfamiliar terrain. Pairs with `forge-design.md` (which is the technical spec); this document is the *why* and the *context*.

---

## 1. The landscape: you're not alone in this space

Scaffolding tools have been around for over a decade. Knowing what came before tells you what to copy, what to avoid, and where forge fits.

**The three landmark tools:**

*Yeoman* (~2012, Node.js) — invented the modern scaffolder. Each scaffolder is a "generator" that you publish to npm. To create a Yeoman generator, you have to write JavaScript. Its descendants live on in `create-react-app`, `create-next-app`, and similar `create-*` tools. The big lesson: requiring people to *program* to make a template is too much friction.

*Cookiecutter* (~2013, Python) — fixed Yeoman's biggest problem. Templates are just folders with Jinja-templated files plus a `cookiecutter.json` config. You write zero code to create a template. This is closer to what we're building. ~4000+ public templates on GitHub.

*Copier* (~2018, Python) — the modern successor. YAML config (not JSON), supports updating projects when the template evolves, supports migrations between template versions. This is the most sophisticated scaffolder in production today. Our `manifest.yaml` design borrows directly from Copier's `copier.yml`.

**Where forge fits:** Copier-shaped, but with three distinguishing properties: it's a Go binary (no Python/Node runtime), it emits knowledge graphs alongside projects (visualization story), and it's designed from the ground up to be agent-callable, not just human-callable. That last point is the unique part — neither Yeoman, Cookiecutter, nor Copier was designed with AI agents as a first-class user.

**The honest competitive landscape for our first blueprint:** for "scaffold a Next.js app," `create-next-app` exists, `create-t3-app` exists, and individual developers ship project-specific CLIs every week (saw a recent one called `create-samrose-app` that does what our hackathon-app blueprint will do). What makes forge different isn't being a better Next.js scaffolder — it's being one tool that does Next.js *and* CLI tools *and* OpenClaw skills *and* whatever else. The platform is the moat, not any single blueprint.

**An important distinction nobody states clearly: scaffold vs boilerplate.** A *scaffold* gives you a folder structure and starts you with `create-next-app` plus a few file moves — saves you 30 minutes. A *boilerplate* gives you full auth flows, billing integration, multi-tenancy, error monitoring — saves you days or weeks. We are building scaffolders in v0.1. Boilerplate-quality blueprints are a v0.3+ direction once the framework is solid. Don't mix these up in conversations.

---

## 2. The Go + Cobra ecosystem: what you're walking into

Cobra is the de facto standard for Go CLI tools. It powers `kubectl`, `docker`, `gh` (GitHub CLI), `hugo`, `helm`, and roughly every modern Go CLI you've used. This is comforting — you're not picking an obscure library; you're picking the one everyone reaches for.

**Things you should know about Cobra without needing to learn them deeply:**

The unit of organization is a `cobra.Command` — a struct with fields like `Use` (the command name), `Short` (one-line description), `RunE` (the function that does the work). You create one per subcommand and call `AddCommand` to nest them. The whole CLI is a tree of commands.

In 2026 idiomatic Go, you use `RunE` (returns an error) rather than the older `Run` (no return value). Errors flow up and Cobra handles printing them. This is one of those small details where being slightly out of date can make your code look amateur.

Cobra integrates with `viper` for configuration management (env vars, config files). We are deliberately not using viper for v0.1 — forge doesn't need a config file; everything flows through flags and stdin. Adding viper later if we need it is one line.

**The `huh` library for prompts** — `charmbracelet/huh` is the modern choice. It does single-select, multi-select, text input, confirms, all with good visual design and keyboard handling. The older choice `survey` still works but feels dated. We'll use huh.

**The `embed` package** is the magic that lets forge be a single binary. You write `//go:embed blueprints/*` above a variable declaration and the entire `blueprints/` folder gets compiled into your binary. At runtime, your code reads from that embedded filesystem the same way it would read from disk. This was added to Go in 1.16 (2021) and works flawlessly. The compiled binary will end up maybe 15-25MB once we have a few blueprints — totally fine.

**Performance characteristics you can take for granted:** Go binaries start in 5-30ms. The Go runtime is fast enough that you'll never wait on the language — file I/O and the `pnpm install` step in post-create hooks will be the slow parts. Don't waste time optimizing forge itself; optimize the user experience around the slow parts (e.g., spinners during install, async file writes).

**Cross-compilation is trivial.** From your Arch box: `GOOS=darwin GOARCH=arm64 go build -o forge-mac ./cmd/forge` and you have a Mac binary. No Mac required. Reverse direction works identically.

---

## 3. CLI design: the principles you should absorb

This section is the most underrated. Most first-time CLI builders ship something that works but feels amateur. The difference between "works" and "feels professional" is a handful of principles you can internalize in an hour. The canonical reference is `clig.dev` — read it once, end-to-end, when you have a free hour. It's worth more than any Go tutorial.

The short version of what matters most for forge:

**Design for humans first, but stay scriptable.** This is the entire game. A command should be pleasant for you typing it interactively (good prompts, clear output, helpful errors) AND consumable by a script or agent (predictable JSON output via `--json`, exit codes that mean something, no decorative output when stdout is not a TTY). The way you achieve both: detect whether stdout is a terminal (`isatty`), and switch output formats accordingly. Pretty colored output to humans, plain JSON to pipes.

**Verb-noun grammar that matches what people already know.** We're using `forge new`, `forge add`, `forge list`, `forge visualize` — and these read like English while matching patterns from git, docker, kubectl. Don't get clever. Don't invent `forge spawn` when `forge new` is what people expect. Consistency across our commands matters more than originality: if `new` creates, then `delete` should remove (not `remove`, not `destroy` — pick one verb family and stick with it).

**Make the common case short.** `forge new hackathon-app my-idea` is what you'll type most often. Every flag has a reasonable default so this short form just works. The full kitchen-sink invocation with every flag set is rare and OK to be verbose. Optimize for the median use, not the maximum.

**Error messages should help the user fix the problem, not just describe it.** Bad: "Error: invalid manifest." Good: "manifest.yaml line 23: variable 'agents_count' has min=0 but default is -1; either raise min or change the default." The user has zero context about what your code thinks; tell them what to do next, not what went wrong internally.

**Show what you're doing.** When forge is creating 25 files, run `pnpm install`, and writing a knowledge graph, don't be silent for 30 seconds. Print "Rendering files...", "Installing dependencies...", "Done. Project at ./my-app" with a tiny progress indicator. Silence makes users wonder if it's frozen. Charm's `lipgloss` and `bubbles` libraries make this look professional in 20 lines of code.

**Help text is interface, not afterthought.** `forge new --help` is read more than `forge new` is run. Spend time on it. Every flag gets a one-line description that's actually useful. Examples section at the bottom showing the 3-4 common invocations. This is your tool's tutorial.

**Composability: respect the conventions.** stdin/stdout are inputs/outputs that should chain with other tools. `--json` output of one forge command should be valid input for another. Exit code 0 means success, nonzero means failure. Don't print errors to stdout (only stderr). These are unspoken contracts that make your tool feel like it belongs in a Unix pipeline.

---

## 4. Scaffolders have two modes you should architect for

There's an architectural insight from John Freeman's CLI design writing that's worth knowing: scaffolders come in two flavors, and most tools only do one well.

*Project scaffolders* create a whole new project from scratch and walk away. `create-next-app`, `cookiecutter`, `create-t3-app`. They run once, give you a starting point, and you're on your own from there.

*Component scaffolders* add things incrementally to an existing project. `rails generate`, Angular CLI's `ng generate`, `hygen`. They run many times during a project's life.

Forge does *both*. `forge new` is project scaffolding. `forge add` is component scaffolding. The reason most tools do only one is that the second is harder — it requires the project to *know* it was forge-scaffolded (which is why we drop `.forge/project.json` as a marker), and it requires extensions to compose without conflicting.

The deeper insight: if we ever want a `forge upgrade` command that re-renders a project against a newer blueprint version, that's a *third* mode (lifecycle management) and only Copier really pulls it off. We're not building it in v0.1, but the marker file is there to support it later. Notice how many architectural decisions cascade from "we want this to be possible someday."

---

## 5. Specific things first-time CLI builders get wrong

I want to flag these explicitly because every one of them is something you'd otherwise learn the hard way.

**Putting too much logic in `main.go`.** Your `cmd/forge/main.go` should be ~30 lines, basically just calling into `internal/cli`. Real work lives in tested packages. If your `main.go` is doing string manipulation, your architecture is already wrong.

**No tests until "the end."** This is the classic trap. You'll write 2000 lines, then try to add tests, then realize your code isn't testable because everything depends on `os.Stdin` or hardcoded paths. The fix: write packages with explicit dependencies (pass `io.Reader` for input, `io.Writer` for output, pass paths as arguments — never read directly from `os.Stdin` or `os.Args` in your packages). Then unit tests pass mock readers/writers and assert outputs. Top-level `main` is the only place that touches the real outside world.

**Mixing config concerns.** Variables can come from: defaults, JSON stdin, flags, prompts. There's a strong temptation to write code like "if the flag is set use it, else read stdin if it's there, else prompt." This logic gets gnarly fast. Better: write a `Values` struct that gets populated from each source in priority order, with each source being its own function. Compose them in one place.

**Forgetting about Windows.** You don't run Windows, so forge probably doesn't need to. But if you ever publish it, Windows users exist. Use `filepath.Join` not string concatenation with `/`. Use `path/filepath` not `path` for OS paths. The Go standard library makes this almost automatic if you use the right packages.

**Premature flag explosion.** It's tempting to add a flag for every possible behavior. Resist. Defaults should cover 90% of cases. Add a flag only when there's a real use case where the default is wrong. Every flag is a maintenance burden, a doc entry, and a potential bug source.

**Output that breaks pipes.** This is subtle. If your tool prints colored text and someone pipes it into `grep`, the grep sees ANSI escape codes and gets confused. Fix: check `os.Stdout` is a TTY before applying colors. Libraries like `lipgloss` handle this for you if you set them up right.

**Inconsistent JSON output.** When `--json` mode is active, *all* output should be JSON, including errors. Common failure: success is JSON but errors go to stderr as plain text. Decide on a JSON error envelope and use it everywhere in JSON mode.

**Forgetting the dry-run path.** `--dry-run` exists so users (and agents) can preview what forge would do without committing. This is critical for agents — they want to verify before acting. Build it from day one, not as an afterthought, because retrofitting "don't actually write files" into a codebase that assumes file writes is painful.

**Not budgeting time for the "polish" pass.** When the tool "works" you'll be 70% done. The remaining 30% is the difference between something you'd publish and something embarrassing: good error messages, help text, examples, README, install script, a demo GIF. Budget for it.

---

## 6. The execution plan, with realistic expectations

The design doc says ~12 working days for v0.1. That's an honest estimate if you're focused. As a first-timer, expect 1.5-2x that — call it 3 weeks at part-time pace. This isn't a bad thing. Here's how that time breaks down emotionally:

**Week 1: foundations and tedium.** M1 and M2. Setting up the workspace, learning Cobra patterns, writing manifest parsing, getting the first template to render. This is the unsexy part — feels like nothing visible is happening, lots of plumbing. The breakthrough moment is when `forge new hackathon-app test-app` produces an actual file on disk. Push through this; it's the hardest emotionally.

**Week 2: the satisfying middle.** M3, M4, M5. Graph emission, hooks, JSON mode. By the end of this week you have a tool that actually scaffolds a working Next.js app. You'll be tempted to ship it. Don't yet — add hasn't landed.

**Week 3: extensions and polish.** M6 and M7. `forge add` is more complex than it sounds because you're modifying existing projects without breaking them. Then polish, README, install script.

**The right success criterion for v0.1**: you scaffold a fresh Next.js project with forge, run `pnpm dev`, see a working page, then run `forge add api-route hello`, see a new route appear, and forge visualize opens an Understand-Anything dashboard showing the project map. If that demo works end-to-end, ship it and start blueprint #2.

---

## 7. What you should learn vs delegate to Claude Code

You don't need to become a Go expert to build forge. You need to be able to read Go code, understand the structure, and make architectural calls. Here's the realistic division:

**Worth learning yourself (a few hours each):**

- Reading basic Go syntax: structs, methods, interfaces, error handling with `if err != nil`. The official Tour of Go (`tour.golang.org`) is 90 minutes and covers what you need.
- How Cobra commands are structured: `cobra.Command` struct, `AddCommand`, `RunE`. Skim the README.
- How `go:embed` works: one paragraph in the design doc covers it.
- The clig.dev guide. Worth a one-hour read. You'll think about CLIs differently after.

**Delegate to Claude Code without guilt:**

- Writing the actual Go code from the design spec.
- Tests. Claude Code writes solid Go tests faster than you'd write them by hand.
- The boilerplate-heavy bits like loading YAML into structs, walking a filesystem tree, executing shell commands with proper error handling.
- The hackathon-app blueprint's template files (the actual Next.js code for the landing page, the AI route handler, etc.).
- Cross-compilation Makefile, GitHub Actions for CI if we add it later.

**Always keep yourself:**

- The architecture decisions (this is where humans add value AI can't replicate yet).
- The specific blueprint choices (you know what *you* want from a hackathon scaffold).
- The user-facing copy: error messages, help text, README. These are the personality of your tool.
- Reviewing each milestone before moving on — read the code Claude writes, even if you can't fully evaluate it. You'll start to see patterns.

---

## 8. Resources to bookmark

The ones you'll actually use:

- `clig.dev` — CLI design principles. One-time read, lasting impact.
- `cobra.dev` and the Cobra GitHub repo — for syntax lookups during M1-M2.
- `pkg.go.dev` — Go's package docs. Search any standard library package here.
- `github.com/charmbracelet/huh` — for prompt library usage examples.
- `github.com/charmbracelet/lipgloss` — for terminal styling when we add polish.
- `tour.golang.org` — the canonical Go intro. Do it once.
- Copier's docs (`copier.readthedocs.io`) — for inspiration on the manifest format and update flows. You're not using Copier, but you're stealing its ideas.
- The Understand-Anything repo — for the exact JSON schema we need to match. Critical for M3.

The ones not to drown in:

- "Awesome Go" lists. Too much noise.
- Go conference talks. Save them for later.
- Anything about Go generics. We don't need them for this project.

---

## 9. The mindset

A few things to keep in mind that don't fit anywhere else:

**Forge will not be perfect at v0.1, and that's the point.** Ship something that works for the one blueprint, then iterate. Every extra week spent making v0.1 "complete" is a week you're not learning from real use.

**The blueprint will teach you more than the framework.** When you author the hackathon-app blueprint's templates, you'll discover things the manifest schema can't yet express. That's data. Note them down; fix them in v0.2.

**Don't optimize what you haven't measured.** People obsess about Go performance because the language invites it. Forge will be fast enough without you ever thinking about performance. Spend that energy on UX instead.

**Excited beats stressed.** You said you're excited. Stay there. When you hit a wall (and you will), the answer is almost always "ask Claude Code, read the error message, take a walk." Not "rewrite everything." Building a tool is hard; getting through the hard parts is most of the value.

You're building real software that you'll use for years. That's a different vibe than coursework or tutorials. Treat it like that — your tool, your choices, your standards.

---

## What to do next

Three concrete next steps after you've read this:

1. Read `clig.dev` once. Yes, the whole thing. It's two hours and it's the highest-leverage time you'll spend.
2. Skim the Tour of Go (tour.golang.org) up through "Methods and Interfaces". Skip generics, concurrency, anything that smells deep — you can return to it as needed.
3. Come back and tell me you're ready, and we'll start M1 together: `go mod init`, the first `main.go`, the directory structure, getting `forge --help` to print.

That's the briefing. You should feel oriented now. Ask anything that's still fuzzy.
