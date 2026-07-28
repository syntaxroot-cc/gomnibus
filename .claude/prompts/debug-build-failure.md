---
name: debug-build-failure
description: Diagnose and fix a failing gomnibus build
---

The gomnibus build for **{{project}}** is failing. Here is the error output:

```
{{error_output}}
```

Please:
1. Identify which stage failed: fetch / build / health-check / packaging.
2. Identify the software component involved.
3. Diagnose the root cause.
4. Propose and apply a targeted fix to the relevant YAML definition or Go source.
5. Explain what was wrong and how the fix resolves it.
6. Confirm the fix by running `gomnibus validate {{project}}`.

For health check failures specifically, check whether the offending library
should be added as a dependency (preferred) or added to `whitelist_files:`.
