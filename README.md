# gomnibus

> A Go-based rewrite of [Chef Omnibus](https://github.com/chef/omnibus) — build
> full-stack, self-contained software installers from declarative YAML definitions.

[![Go](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

gomnibus produces cross-platform packages (`.deb`, `.rpm`, `.pkg`, `.msi`,
`.tar.gz`) that embed an application together with all its runtime dependencies.
No system Ruby, no Bundler, no Gemfile — just a single Go binary and YAML
definitions.

---

## Features

- **Declarative YAML DSL** for project and software definitions
- **Multiple fetcher backends**: HTTP(S) with checksum verification, Git (branch/tag/commit), local path, S3
- **All major build systems**: `./configure`, CMake, Make, Go, Gem, and arbitrary shell commands
- **Dependency-ordered builds** via topological sort with cycle detection
- **Build caching** (local content-addressable cache; S3 cache planned)
- **Package formats**: `.deb` (dpkg-deb), `.rpm` (rpmbuild), `.tar.gz` universal; `.pkg` and `.msi` planned
- **Shared-library health check** (`ldd`/`otool`) with per-software whitelist
- **Version manifest** (JSON) for reproducible builds
- **License collection** and compliance checking
- **Version overrides** in project files — pin any dependency to a specific version
- **Multi-version software definitions** — conditional source/build per version

---

## Installation

```bash
go install github.com/syntaxroot-cc/gomnibus/cmd/gomnibus@latest
```

Or build from source:

```bash
git clone https://github.com/syntaxroot-cc/gomnibus
cd gomnibus
go build ./cmd/gomnibus
```

---

## Quick start

```bash
# Create a new project skeleton
gomnibus new myapp

# Edit the generated files
vim config/projects/myapp.yaml
vim config/software/myapp-software.yaml

# Validate without building
gomnibus validate myapp

# Build and package
gomnibus build myapp

# Print the version manifest
gomnibus manifest myapp
```

---

## Project layout

```
gomnibus.yaml                  # global config
config/
  projects/
    myapp.yaml                 # project definition
  software/
    zlib.yaml                  # software component definitions
    openssl.yaml
    myapp.yaml
pkg/                           # output packages
```

---

## Software definition

`config/software/openssl.yaml`:

```yaml
name: openssl
default_version: "3.3.1"
license: OpenSSL

source:
  url: "https://www.openssl.org/source/openssl-3.3.1.tar.gz"
  sha256: "777cd596284c883375a2a7a11bf5d2786fc5413255efab20c50d6ffe6d020b7e"

relative_path: "openssl-3.3.1"
dependencies:
  - zlib

build:
  - command: "./Configure --prefix=${install_dir} shared zlib"
  - make: []
  - make:
      - install
```

---

## Project definition

`config/projects/myapp.yaml`:

```yaml
name: myapp
maintainer: "Acme <packages@acme.com>"
install_dir: /opt/myapp
build_version: "2.1.0"
build_iteration: 1
dependencies:
  - zlib
  - openssl
  - myapp
overrides:
  - name: openssl
    version: "3.3.1"
packages:
  - type: deb
  - type: rpm
```

---

## CLI reference

```
gomnibus new PROJECT            Generate project skeleton
gomnibus build PROJECT          Fetch, build, and package
gomnibus manifest PROJECT       Print JSON version manifest
gomnibus validate PROJECT       Validate definitions (no build)

Flags:
  -c, --config string           Config file (default: gomnibus.yaml)
  -l, --log-level string        debug | info | warn | error (default: info)
  -o, --output string           Output directory for packages (default: pkg)
  -p, --package-type string     Override package type (deb, rpm, pkg, msi, tar)
      --skip-healthcheck        Skip shared-library health check
```

---

## Comparison with Ruby Omnibus

| Feature                    | Ruby Omnibus             | gomnibus                       |
|----------------------------|--------------------------|--------------------------------|
| Definition language        | Ruby DSL                 | YAML                           |
| CLI tool                   | `omnibus`                | `gomnibus`                     |
| Single binary              | No (requires Ruby)       | Yes                            |
| Fetchers                   | url, git, path, s3       | url, git, path, s3             |
| Build systems              | make, configure, rake... | make, configure, cmake, go...  |
| Dep ordering               | ✓                        | ✓                              |
| Version overrides          | ✓                        | ✓                              |
| Multi-version blocks       | ✓                        | ✓                              |
| Health check               | ✓                        | ✓                              |
| Version manifest           | ✓                        | ✓                              |
| License collection         | ✓                        | ✓                              |
| .deb / .rpm packaging      | ✓                        | ✓                              |
| .pkg (macOS)               | ✓                        | planned                        |
| .msi (Windows)             | ✓                        | planned                        |
| S3 caching                 | ✓                        | planned                        |
| Parallel builds            | limited                  | planned                        |
| Test Kitchen integration   | ✓                        | planned                        |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The Claude Code configuration in `.claude/`
contains agents, skills, and prompts to help implement new features.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
