# GWT Spec Writing Guide

## Format Syntax

### File Structure

```
;=============================================
; Scenario: First scenario description
;=============================================
GIVEN precondition one.
WHEN action one.
THEN outcome one.

;=============================================
; Scenario: Second scenario description
;=============================================
GIVEN precondition two.
WHEN action two.
THEN outcome two.
```

### Rules

- **Separator lines**: Only `;` and `=` characters (e.g., `;=============================================`)
- **Description lines**: Start with `;` followed by text (between separator pairs)
- **Step keywords**: `GIVEN`, `WHEN`, `THEN` — ALL CAPS, at the start of the line
- **Periods**: Every step line ends with a period
- **Blank lines**: Allowed between scenarios, ignored by parser
- **No trailing whitespace**: Keep lines clean
- **One scenario per separator block**: Opening separator, description line(s), closing separator, then steps

### Multi-Scenario Files

A single spec file can contain multiple scenarios. Each scenario gets its own
test function when generated. Use multiple scenarios to cover variations of
the same user story (happy path, edge cases, error cases).

## Domain Language Discipline

Specs must use the vocabulary of the **user's domain**, not the implementation.

### lmk Domain Vocabulary

| Domain Term | NOT This |
|-------------|----------|
| project | repository, working directory, git repo |
| chapter | markdown file, .md file |
| section | heading, H2 |
| status | cobra command output, stdout |
| the user runs | CLI invokes, command handler executes |
| output shows | stdout contains, fmt.Println writes |
| error message | stderr, exit code, os.Exit |
| project file | .lmk config, YAML manifest |

### Good/Bad Examples

**1. Describing preconditions**

Good:
```
GIVEN a project with two chapters named "intro" and "rising-action".
```

Bad:
```
GIVEN a directory containing intro.md and rising-action.md files.
```

**2. Describing user actions**

Good:
```
WHEN the user runs lmk compile.
```

Bad:
```
WHEN exec.Command invokes the compile subcommand.
```

**3. Describing outcomes**

Good:
```
THEN the output shows the combined text of both chapters in order.
```

Bad:
```
THEN stdout contains the concatenated string content of both file buffers.
```

**4. Describing errors**

Good:
```
THEN an error message says the project has no chapters.
```

Bad:
```
THEN the process exits with code 1 and stderr prints "no chapters found".
```

**5. Multi-step setup**

Good:
```
GIVEN a project with one chapter named "draft".
GIVEN the chapter contains a section named "opening".
WHEN the user runs lmk status.
THEN the output lists one chapter with one section.
```

Bad:
```
GIVEN os.MkdirAll creates a temp directory.
GIVEN os.WriteFile writes draft.md with ## opening header.
WHEN the status handler reads the directory.
THEN JSON output includes chapters[0].sections[0].name == "opening".
```

## Spec Review Checklist

Before committing a spec file, verify:

1. **File name** matches `specs/US<N>-<kebab-case-title>.txt`
2. **Separator syntax** uses only `;` and `=` characters
3. **Keywords** are ALL CAPS (`GIVEN`, `WHEN`, `THEN`)
4. **Every step** ends with a period
5. **No implementation language** — no code identifiers, no infrastructure terms
6. **Scenarios are independent** — each can run without the others
7. **Outcomes are observable** — describe what the user sees, not internal state
8. **Error preconditions covered** — If a command requires setup (e.g., project init), include a scenario for what happens without it
9. **Bootstrap scenarios exist** — If a command can be the user's first interaction, spec the cold-start path

Run `/spec-check` to automate leakage detection on committed specs.
