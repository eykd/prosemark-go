---
description: Audit GWT acceptance specs for implementation language leakage
---

## User Input

```text
$ARGUMENTS
```

## Instructions

Run the spec-guardian agent to audit GWT acceptance specs for implementation leakage.

**If a specific file is provided**: Audit only that file.
**If no file is provided**: Audit all files in `specs/*.txt`.

### Steps

1. Check that `specs/` directory exists. If not, report: "No specs directory found. Create specs using `/sp:05-tasks` first."

2. Read the spec file(s) to audit.

3. For each file, parse every GIVEN, WHEN, and THEN statement and check for implementation leakage:

   | Category | Examples |
   |----------|----------|
   | Code references | function names, variable names, package paths |
   | Infrastructure | HTTP, REST, database, SQL, endpoint, cache |
   | Framework language | Cobra, handler, middleware, context, goroutine |
   | Technical protocols | JSON, gRPC, WebSocket |
   | Data structures | array, map, slice, struct, interface |

4. Output a summary table of findings (file, line, statement, category, suggested rewrite).

5. If no violations found, report: "All specs use clean domain language."
