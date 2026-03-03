# Prosemark CLI — Product Requirements Document

## 1. Overview

Prosemark is a CLI tool for managing long‑form prose projects such as novels, essays, books, or research documents. It provides structural project management while delegating actual writing to an external editor.

The system uses a Markdown "binder" file to represent the hierarchical structure of the manuscript. Individual nodes in the binder correspond to Markdown files containing draft text and optional notes.

The CLI enables a user to:

- manage the outline
- create and organize nodes
- open draft and note files in an external editor
- validate project structure
- compile a manuscript by joining node files in binder order

Prosemark does **not** provide an interactive writing interface or TUI in this phase.

---

## 2. Product Goals

The CLI must allow a user to fully manage a long‑form prose project through the terminal while writing in their editor of choice.

The tool must:

- treat Markdown as the canonical storage format
- treat the binder as the authoritative structure
- support both generated UUID nodes and human‑named nodes
- maintain compatibility with manual editing of project files

---

## 3. Non‑Goals

The following are explicitly out of scope for this phase:

- TUI writing interfaces
- freewriting tools
- export formats beyond simple Markdown compilation
- formatting or layout tools

---

## 4. Core Concepts

### 4.1 Binder

The binder is a Markdown document named:

```
_binder.md
```

It defines the hierarchical structure of the manuscript using Markdown list syntax.

Example:

```
# Salvage of Empire

- [Part I](part1.md)
  - [Chapter 1](chapter1.md)
  - [Chapter 2](chapter2.md)
- [Part II](part2.md)
```

The binder is treated as the canonical representation of the manuscript structure.

Multiple outline sections may appear in the binder. All outlines are treated as part of a single continuous tree.

Free text that does not match expected outline syntax is ignored by the parser.

---

### 4.2 Nodes

Each binder entry represents a **node**.

A node corresponds to a Markdown file containing the draft text.

Example:

```
chapter1.md
```

Nodes may exist before they are referenced in the binder, or may be created automatically by CLI commands.

---

### 4.3 Placeholders

A placeholder node may appear in the binder using an empty link target:

```
- [Chapter 3]()
```

This represents a structural placeholder that has not yet been materialized into a file.

Lines that are not Markdown links are treated as free text and ignored:

```
- Chapter 3
```

---

### 4.4 Notes Files

When a node is materialized by the CLI, a notes file is automatically created.

Draft file:

```
uuid.md
```

Notes file:

```
uuid.notes.md
```

The notes file contains a backlink to the draft using an Obsidian‑style link:

```
[[uuid]]
```

Notes files are not referenced in the binder.

---

## 5. Node Identity

Prosemark supports two node identity models.

### 5.1 Generated Nodes (Preferred)

Nodes created by CLI commands use UUIDv7 filenames.

Example:

```
018f4c5c-1a23-7b8e-9f00-7aab12c4e1aa.md
```

These nodes receive full validation from the system.


### 5.2 Human‑Named Nodes

Users may manually create nodes with human‑readable filenames.

Example:

```
chapter3.md
introduction.md
```

These nodes are fully supported by the binder parser and selector logic.

The doctor audit may emit warnings for these nodes but they remain valid.

---

## 6. CLI Commands

The CLI command namespace is:

```
pmk
```

### 6.1 init

```
pmk init
```

Initializes a prosemark project in the current directory.

Creates:

```
_binder.md
```

---

### 6.2 add

```
pmk add <parent>
```

Adds a new child node under the specified parent in the binder.

Behavior:

- generates a UUIDv7 node
- creates a draft file
- creates a notes file
- inserts the node into the binder outline

---

### 6.3 delete

```
pmk delete <node>
```

Removes a node from the binder.

Draft and notes files remain on disk unless explicitly removed.

---

### 6.4 move

```
pmk move <node> <target>
```

Moves a node to a new position within the binder hierarchy.

---

### 6.5 materialize

```
pmk materialize <placeholder>
```

Converts a placeholder binder entry into a real node.

Behavior:

- generates a UUID node
- replaces the placeholder link target
- creates draft and notes files

---

### 6.6 edit

```
pmk edit <node>
```

Opens the node draft file in `$EDITOR`.

---

### 6.7 notes edit

```
pmk notes edit <node>
```

Opens the node notes file in `$EDITOR`.

---

### 6.8 structure

```
pmk structure
```

Displays the binder hierarchy as a Markdown outline.

Example output:

```
# Salvage of Empire

- Part I
  - Chapter 1
  - Chapter 2
- Part II
  - Chapter 3
```

The command may optionally support JSON output.

---

### 6.9 parse

```
pmk parse
```

Parses the binder and outputs a JSON representation of the structure.

This command is intended for machine use.

---

### 6.10 doctor

```
pmk doctor
```

Validates project integrity.

Checks include:

- binder structure
- node references
- missing files
- invalid filenames

Warnings may be emitted for human‑named nodes.

---

### 6.11 compile

```
pmk compile
```

Generates a manuscript by concatenating node draft files in binder order.

Behavior:

1. Parse the binder
2. Traverse nodes in order
3. Read each node's draft file
4. Strip frontmatter
5. Append the body to the output

Between nodes, insert:

```
\n\n
```

Placeholder nodes are skipped.

The compiled manuscript is written to stdout.

Users may redirect output:

```
pmk compile > manuscript.md
```

---

## 7. Project Structure

A typical project directory:

```
project/

_binder.md

018f...uuid.md
018f...uuid.notes.md

chapter3.md
```

The binder defines structure while node files store content.

---

## 8. Editing Workflow

A typical workflow:

1. Define structure in `_binder.md`
2. Use `pmk add` to create nodes
3. Use `pmk edit` to write drafts
4. Use `pmk notes edit` for planning
5. Use `pmk compile` to produce the manuscript

The editor remains responsible for all writing.

---

## 9. Acceptance Criteria

The CLI must allow a user to:

- create and manage a prose project
- manipulate binder structure
- create and edit node drafts
- maintain node notes
- validate project integrity
- compile a manuscript from binder order

All project files remain valid Markdown and editable outside the CLI.

