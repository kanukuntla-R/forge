# forge Claude Code Plugin

Adds forge scaffolding and codebase visualization to Claude Code.

## What it does

This plugin teaches Claude Code to:

- Scaffold projects using forge blueprints (Next.js, FastAPI, Python CLI, Go CLI)
- Visualize codebase structure with `forge visualize`
- Extend existing forge projects with `forge add`

## Installation

In Claude Code:

```
/plugin marketplace add https://github.com/kanukuntla-R/forge
/plugin install forge
```

## Requirements

Users need forge installed on their system:

```bash
curl -fsSL https://raw.githubusercontent.com/kanukuntla-R/forge/main/install.sh | bash
```

## Slash commands

- `/forge-new` — Scaffold a new project
- `/forge-visualize` — Visualize the current codebase

The skill also triggers automatically when the user mentions scaffolding, project setup, or codebase visualization.

## License

Plugin content is MIT licensed.
The forge CLI itself is licensed under PolyForm-NC-1.0.0.
