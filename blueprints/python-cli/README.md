# python-cli

Minimal Python CLI starter using Typer.

## Usage

```bash
forge new python-cli my-cli
```

Scaffolds a working CLI with one example command (`hello`).

## What you get

- `pyproject.toml` — uv-managed dependencies (Typer, pytest, Ruff) with hatchling build backend
- `src/main.py` — Typer app that wires up commands
- `src/commands/hello.py` — example command demonstrating options and flags
- `tests/test_hello.py` — tests using Typer's CliRunner

## Running the CLI

After scaffolding and `uv sync --extra dev`:

```bash
# Via module (always works)
uv run python -m src.main hello --name World

# Via installed entry point (works because hatchling is the build backend)
uv run my_cli hello --name World
```

## Adding a new command

1. Create `src/commands/<command_name>.py` with a function decorated using `typer.Option`
2. Register in `src/main.py`: `app.command()(<module>.<function>)`
