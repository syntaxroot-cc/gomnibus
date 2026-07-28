# add-software

Add a new software component to this gomnibus project.

## Usage

```
/add-software <name> [version] [url]
```

## Steps

1. Search for the software's latest stable release if version not provided.
2. Download the tarball and compute its SHA256 checksum.
3. Identify its build system (autoconf, CMake, Go, Make, meson, etc.).
4. Identify runtime dependencies that are already defined as gomnibus software.
5. Write `config/software/<name>.yaml` using the software-author agent.
6. Add `<name>` to the `dependencies:` list in the appropriate project YAML.
7. Run `gomnibus validate <project>` to confirm the graph is consistent.

## Template reference

See `examples/myapp/config/software/` for working examples of:
- `zlib.yaml` — autoconf project with SHA256 checksum
- `openssl.yaml` — autoconf with version blocks and zlib dependency
- `myapp.yaml` — Go project with git source

## Checksum verification

```bash
curl -L <url> | sha256sum
```

Or for known releases with a .sha256 file:
```bash
curl -L <url>.sha256
```
