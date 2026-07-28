---
name: port-omnibus-software
description: Convert a Chef Omnibus Ruby software definition to gomnibus YAML
---

Convert the following Chef Omnibus Ruby software definition to a gomnibus YAML
software definition and save it to `config/software/`.

Ruby source:
```ruby
{{ruby_content}}
```

Use the conversion table in `.claude/skills/port-from-omnibus.md`.

Important notes:
- Do not lose any version blocks — map each `version "x" do … end` to a
  `versions:` entry.
- Translate `whitelist_file /pattern/` to a `whitelist_files:` entry (as a
  YAML string, not a regex literal).
- If there is Ruby logic in the build block (conditionals, loops), preserve the
  intent as a `command:` step using shell syntax where possible.
- After writing the file, run `gomnibus validate {{project}}` to confirm.
