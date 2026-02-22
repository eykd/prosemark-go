---
name: skill-improver
description: Use when: (1) auditing existing skills for best practices, (2) improving skill descriptions to focus on WHEN to use, (3) reorganizing skill resources for token efficiency, (4) applying user feedback to skill improvements.
---

# Skill Improver

Review and improve Claude Skills based on best practices and user feedback.

## Workflow

1. **Read** the target skill's SKILL.md
2. **Audit** against best practices checklist (see [references/best-practices.md](references/best-practices.md))
3. **Gather** user input on specific improvements needed
4. **Apply** improvements and present changes

## Audit Process

When reviewing a skill, check each category in order:

### 1. Description Quality (Critical)

The description determines when Claude invokes the skill. Common problems:

| Problem                 | Example                      | Fix                                                                                          |
| ----------------------- | ---------------------------- | -------------------------------------------------------------------------------------------- |
| Describes WHAT not WHEN | "A tool for processing PDFs" | "Use when: (1) filling PDF forms, (2) extracting text from PDFs, (3) merging/splitting PDFs" |
| Too vague               | "Helps with documents"       | List specific file types and operations                                                      |
| Missing triggers        | No usage scenarios           | Add numbered trigger list                                                                    |

**Good description formula**: "Use when: (1) scenario, (2) scenario, (3) scenario. Supports: file types, frameworks, tools."

### 2. Token Efficiency

- Does SKILL.md contain info Claude already knows? Remove it.
- Is detailed info in references/ or duplicated in SKILL.md? Move to references.
- Are there README.md, CHANGELOG.md, or setup guides? Remove them.

### 3. Structure Appropriateness

Match structure to skill purpose:

- **Sequential process** → Workflow-based with numbered steps
- **Multiple operations** → Task-based with sections per operation
- **Standards/specs** → Reference/guidelines format

### 4. Progressive Disclosure

- SKILL.md should be <500 lines
- Detailed info belongs in references/
- References should be one level deep
- Files >100 lines need table of contents

### 5. Resource Organization

| Directory   | Contains          | Load behavior       |
| ----------- | ----------------- | ------------------- |
| scripts/    | Executable code   | Run without loading |
| references/ | Documentation     | Load on-demand      |
| assets/     | Templates, images | Copy to output      |

## Improvement Report Template

After auditing, present findings as:

```markdown
## Skill Audit: [skill-name]

### Description Assessment

Current: [current description]
Issues: [list problems]
Suggested: [improved description]

### Best Practice Violations

1. [Issue]: [explanation] → [fix]
2. [Issue]: [explanation] → [fix]

### User-Requested Changes

[List any specific changes the user wants]

### Recommended Actions

1. [Specific change to make]
2. [Specific change to make]
```

## Applying Improvements

After user approves:

1. Edit SKILL.md with approved changes
2. Reorganize resources if needed (move content to references, delete unnecessary files)
3. Verify changes against checklist
4. Present diff summary to user
