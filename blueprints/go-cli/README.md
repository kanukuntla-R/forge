# go-cli

Minimal Go CLI starter using cobra for commands and slog for structured logging.

## Usage

```bash
forge new go-cli my-cli --var module_path=github.com/username/my-cli
```

Scaffolds a working CLI with one example command (`hello`) and a preconfigured slog logger.

## What you get

- `main.go` — entry point; version embedded via Makefile ldflags
- `cmd/root.go` — root cobra command, `Execute(version)` entry point
- `cmd/hello.go` — example subcommand with `--name` and `--shout` flags
- `internal/logger/logger.go` — preconfigured slog JSON logger (opt-in)
- `Makefile` — build/test/install/clean targets

## Running the CLI

After scaffolding and `go mod tidy`:

```bash
make build
./my-cli hello --name World
# Hello, World!

./my-cli hello --name World --shout
# HELLO, WORLD!
```

## Adding a new command

1. Create `cmd/<command_name>.go` with a `cobra.Command` var
2. In its `init()` function, call `rootCmd.AddCommand(yourCmd)`
