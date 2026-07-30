# Contributing to gomnibus

## Dev setup

```bash
git clone https://github.com/syntaxroot-cc/gomnibus
cd gomnibus
go build ./cmd/gomnibus
```

### Git hooks

A pre-commit hook enforces `gofmt` on staged Go files. Enable it once after cloning:

```bash
git config core.hooksPath .githooks
```

The hook checks only the files you're committing and tells you exactly what to fix if anything is misformatted. The same check runs in CI (golangci-lint), so enabling the hook avoids a wasted CI run.

## Running tests

```bash
go test ./...
go vet ./...
```

The Kitchen integration tests (Docker-based, multi-platform) run in CI automatically on pushes that touch `internal/`, `cmd/`, or `examples/`. To run them locally, see `examples/myapp/.kitchen.yml` and the `gomnibus kitchen` subcommand.

## Adding a new software component

Use the `/add-software` skill in Claude Code, or follow the YAML schema in `internal/software/definition.go`.

## Adding a new package format

Use the `/add-packager` skill in Claude Code. New packagers register themselves via `init()` and need a blank import in `cmd/gomnibus/main.go`.

## Code conventions

- No comments explaining *what* — only non-obvious *why*.
- New fetcher/packager/cache backends register via `init()`.
- Error messages: `"<context>: <cause>"` with `%w` wrapping.
- `gofmt` is enforced; run it before committing.
