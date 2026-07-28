# port-from-omnibus

Convert an existing Chef Omnibus software definition (Ruby DSL) to a gomnibus
YAML software definition.

## Usage

```
/port-from-omnibus <omnibus-ruby-file>
```

## Conversion reference

| Ruby Omnibus DSL           | gomnibus YAML equivalent                          |
|----------------------------|---------------------------------------------------|
| `name "foo"`               | `name: foo`                                       |
| `default_version "1.2.3"`  | `default_version: "1.2.3"`                       |
| `source url: "..."`        | `source:\n  url: "..."`                           |
| `source git: "..."`        | `source:\n  git: "..."`                           |
| `dependency "bar"`         | `dependencies:\n  - bar`                          |
| `relative_path "foo-1.2"`  | `relative_path: "foo-1.2"`                        |
| `whitelist_file /regex/`   | `whitelist_files:\n  - "regex"`                   |
| `build do ... end`         | `build:` (see build step mapping below)           |
| `version "x" do ... end`   | `versions:\n  - version: "x"\n    source: ...`   |

## Build step mapping

| Ruby Omnibus                 | gomnibus YAML                     |
|------------------------------|-----------------------------------|
| `command "..."`              | `- command: "..."`                |
| `make`                       | `- make: []`                      |
| `make "install"`             | `- make:\n    - install`          |
| `configure "--prefix=..."`   | `- configure:\n    - "--prefix=..."` |
| `mkdir "..."`                | `- mkdir: "..."`                  |
| `delete "..."`               | `- delete: "..."`                 |
| `copy "src", "dst"`          | `- copy:\n    src: src\n    dst: dst` |
| `move "src", "dst"`          | `- move:\n    src: src\n    dst: dst` |
| `link "a", "b"`              | `- link:\n    target: a\n    link: b` |

## Steps

1. Read the Ruby file provided.
2. Map each DSL element using the tables above.
3. Write the resulting YAML to `config/software/<name>.yaml`.
4. Note any Ruby-specific logic (blocks, conditionals) — translate to the nearest
   YAML equivalent or leave a `# TODO` comment.
5. Run `gomnibus validate <project>` to confirm.
