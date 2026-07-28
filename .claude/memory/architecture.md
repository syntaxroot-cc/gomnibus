---
name: gomnibus-architecture
description: Core architecture, package layout, and key design decisions for gomnibus
type: project
---

# gomnibus Architecture

## What it is

gomnibus is a Go rewrite of Chef Omnibus — a tool for building self-contained,
full-stack software installers. It fetches upstream sources, builds them in
dependency order, and packages the result as .deb, .rpm, .pkg, .msi, or .tar.gz.

## Package layout

```
cmd/gomnibus/         CLI entry point (cobra commands)
internal/
  config/             gomnibus.yaml loader → config.Config struct
  project/            config/projects/*.yaml → project.Definition
  software/           config/software/*.yaml → software.Definition + Registry
  fetcher/            Interface + dispatch; sub-packages: git, net, path, s3
  builder/            Executes build steps from a software.Definition
  pipeline/           DAG resolver + topological sort for dependency ordering
  packager/           Interface + dispatch; sub-packages: deb, rpm, tar, (pkg, msi)
  cache/              LocalCache (tar.gz) + NopCache; S3Cache TBD
  health/             Shared-library checker (ldd/otool)
  manifest/           JSON version manifest generator
  license/            License collector + checker
  compressor/         (stub) Post-build compression
pkg/
  log/                zap wrapper with gomnibus defaults
  dsl/                (future) HCL/CUE DSL parser
examples/
  myapp/              Working example with zlib, openssl, myapp
```

## Key design decisions

- **YAML DSL** instead of Ruby — declarative, language-agnostic, easier to
  validate and generate tooling for.
- **Interface-based extensibility** — Fetcher, Packager, and Cache are interfaces
  registered via `init()` and blank imports, enabling new backends without
  modifying core code.
- **Topological sort in pipeline.Graph** — guarantees dependency-first build order;
  detects cycles at validation time.
- **Parallel builds** (planned) — the Graph assigns a `Level` to each node; nodes
  at the same level have no inter-dependencies and can build concurrently using
  `golang.org/x/sync/errgroup`.
- **Content-addressable cache keys** — SHA256 of `name:version:definition-content`
  so any change to a definition invalidates the cache entry.
- **No root required** — set `base_dir` and `install_dir` to user-writable paths.

## CLI commands

| Command                        | Purpose                                  |
|-------------------------------|------------------------------------------|
| `gomnibus new PROJECT`         | Generate project skeleton                |
| `gomnibus build PROJECT`       | Full build + package                     |
| `gomnibus manifest PROJECT`    | Print JSON version manifest              |
| `gomnibus validate PROJECT`    | Validate definitions without building    |

## Configuration precedence

1. CLI flags (`--config`, `--log-level`, `--package-type`)
2. `gomnibus.yaml` in current directory
3. Built-in defaults in `config.Default()`

## Omnibus Ruby → gomnibus YAML feature parity

| Ruby Omnibus feature       | gomnibus equivalent                   | Status    |
|----------------------------|---------------------------------------|-----------|
| Software DSL               | config/software/*.yaml                | ✓ done    |
| Project DSL                | config/projects/*.yaml                | ✓ done    |
| Git fetcher                | fetcher/git                           | ✓ done    |
| Net fetcher                | fetcher/net (HTTP + SHA checksums)    | ✓ done    |
| Path fetcher               | fetcher/path                          | ✓ done    |
| S3 fetcher                 | fetcher/s3 (AWS SDK v2, all auth modes)| ✓ done   |
| Git caching                | cache.LocalCache (tar.gz)             | ✓ done    |
| S3 caching                 | cache.S3Cache + ChainCache            | ✓ done    |
| Dep-ordered builds         | pipeline.Graph                        | ✓ done    |
| Version overrides          | project.Definition.Overrides          | ✓ done    |
| Version blocks             | software.Definition.Versions          | ✓ done    |
| Health check               | health.Check (ldd/otool)              | ✓ done    |
| Whitelist files            | software.Definition.WhitelistFiles    | ✓ done    |
| Version manifest           | manifest.Generate                     | ✓ done    |
| License collection         | license.Collect                       | ✓ done    |
| .deb packaging             | packager/deb                          | ✓ done    |
| .rpm packaging             | packager/rpm                          | ✓ done    |
| .tar.gz packaging          | packager/tar                          | ✓ done    |
| .pkg packaging (macOS)     | packager/pkg (pkgbuild+productbuild)  | ✓ done    |
| .msi packaging (Windows)   | packager/msi (WiX v3/v4, auto-detect)| ✓ done    |
| Changelog generation       | —                                     | planned   |
| Parallel builds            | pipeline.Run (ready-queue, errgroup)  | ✓ done    |
| Test Kitchen integration   | —                                     | planned   |
