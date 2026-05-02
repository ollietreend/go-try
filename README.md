# go-try

Go rewrite of [tobi/try](https://github.com/tobi/try) — an ephemeral workspace manager. Single static binary, no runtime dependency.

## Why?

The original `try` is a single-file Ruby script. Beautiful, but requires Ruby. This version compiles to one static binary you can drop anywhere.

## Architecture

`try` opens an interactive TUI. You select a directory. It prints shell commands to stdout. Your shell wrapper captures and evals them.

This two-step design is not a hack — it's required by the OS:

```
$ try exec project
mkdir -p '/Users/you/src/tries/2026-05-02-project' && \
  cd '/Users/you/src/tries/2026-05-02-project'
```

**A child process cannot change its parent's working directory.** `cd` is a shell builtin (there is no `/bin/cd`), and process working directories are per-process. The only way to change the shell's directory is for the shell itself to do it. So `try init` installs a tiny shell function that wraps `try exec` and `eval`s the output.

### Flow

```
┌─ shell function ──────────────────────────────────┐
│  1. calls:  try exec <query>                      │
│  2. try:     opens TUI, user picks dir            │
│  3. try:     prints "cd /chosen/path && ..."      │
│  4. shell:   captures stdout, evals it            │  ← shell changes dir
└────────────────────────────────────────────────────┘
```

## Install

```bash
go install github.com/ollietreend/go-try/cmd/try@latest
```

Then add to your shell config:

```bash
# bash/zsh (~/.zshrc or ~/.bashrc)
eval "$(try init ~/src/tries)"

# fish (~/.config/fish/config.fish)
eval (try init ~/src/tries | string collect)
```

## Usage

```bash
try                    # Open interactive selector
try redis              # Filter for redis projects
try clone <url>        # Clone repo into dated directory
try worktree <name>    # Create git worktree
try . <name>           # Worktree shorthand
```

## Development

```bash
go run ./cmd/try/ --help
go build -o try ./cmd/try/
go test ./... -v
```

Uses `golang.org/x/term` for raw terminal I/O. No other dependencies.

## License

MIT
