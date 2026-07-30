# gomnibus — Claude Code Guide

## What this project is

gomnibus is a Go-based rewrite of [Chef Omnibus](https://github.com/chef/omnibus).
It builds self-contained, cross-platform software packages (.deb, .rpm, .pkg, .msi,
.tar.gz) by fetching upstream sources, building them in dependency order, and
assembling the result into an install root that is then packaged.

## Quick start

```bash
go build ./cmd/gomnibus           # build the CLI
gomnibus new myapp                # scaffold a new project
gomnibus validate myapp           # validate definitions
gomnibus build myapp              # full build + package
gomnibus manifest myapp           # print JSON version manifest
```

## Key files to know

| Path | Purpose |
|------|---------|
| `cmd/gomnibus/main.go` | CLI commands (cobra) |
| `internal/config/config.go` | Global config loader |
| `internal/software/definition.go` | Software YAML parser + Registry |
| `internal/project/definition.go` | Project YAML parser |
| `internal/pipeline/graph.go` | Dependency DAG + topological sort |
| `internal/fetcher/` | Fetch interfaces; sub-packages: git, net, path |
| `internal/builder/builder.go` | Build step executor |
| `internal/packager/` | Package interfaces; sub-packages: deb, rpm, tar |
| `internal/cache/cache.go` | Build artefact cache |
| `internal/health/check.go` | Shared-library health checker |
| `internal/manifest/manifest.go` | JSON version manifest |
| `internal/license/collector.go` | License collector |

## Available skills (invoke with `/`)

| Skill | Purpose |
|-------|---------|
| `/add-software` | Add a new upstream software component |
| `/add-packager` | Implement a new package format backend |
| `/port-from-omnibus` | Convert Ruby Omnibus DSL to gomnibus YAML |

## Available agents

| Agent | Use when |
|-------|----------|
| `software-author` | Writing software definition YAMLs |
| `packager-author` | Implementing new packager backends |
| `build-debugger` | Diagnosing failed builds |

## Coding conventions

- Pure stdlib + explicit deps; no reflect-heavy frameworks.
- New fetcher/packager/cache backends register via `init()` — add blank imports
  to `cmd/gomnibus/main.go`.
- Build step types are defined as fields on `software.BuildStep` — add new step
  types there and handle them in `internal/builder/builder.go`.
- Error messages follow `"<context>: <cause>"` wrapping with `%w`.
- No comments explaining what — only comments explaining non-obvious *why*.

## Software library (gomnibus-software)

`vendor/gomnibus-software` is a git submodule pointing to
[syntaxroot-cc/gomnibus-software](https://github.com/syntaxroot-cc/gomnibus-software),
a curated library of reusable YAML definitions (zlib, openssl, curl, libffi, etc.).

To use it in a project, list `config/software` **before** the vendor path in `gomnibus.yaml`
so that project-local definitions take precedence over the library (first-found wins in the registry):

```yaml
# gomnibus.yaml
software_dirs:
  - config/software
  - ../../vendor/gomnibus-software/config/software
```

After cloning gomnibus, initialize the submodule:

```bash
git submodule update --init
```

## Test commands

```bash
go test ./...
go vet ./...
```

## Feature parity tracker

See `.claude/memory/architecture.md` for the full Ruby Omnibus → gomnibus
feature parity table.
