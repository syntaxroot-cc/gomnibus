---
name: new-software-component
description: Prompt for adding a new upstream software component to bundle
---

Add the software component **{{name}}** version **{{version}}** to this gomnibus
project.

1. Find the official download URL and SHA256 checksum for version {{version}}.
2. Determine whether it uses autoconf, CMake, meson, or a custom build system.
3. Identify any runtime shared-library dependencies that gomnibus must also build
   (check `./configure --help` output or the project's README).
4. Write `config/software/{{name}}.yaml` with correct source, checksum,
   dependencies, and build steps.
5. Add `{{name}}` to the `dependencies:` list in `config/projects/{{project}}.yaml`.
6. Run `gomnibus validate {{project}}` and fix any errors.

If version blocks are needed for multiple supported versions, add them under
`versions:` in the YAML.
