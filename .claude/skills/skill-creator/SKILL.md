---
name: skill-creator
description: Create or update Claude Skills that extend capabilities with specialized knowledge, workflows, or tool integrations. Use when users want to build a new skill, improve an existing skill, or package a skill for distribution.
---

# Skill Creator

## Skill Structure

```
skill-name/
├── SKILL.md           # Required: frontmatter + instructions
├── scripts/           # Optional: executable code
├── references/        # Optional: docs loaded on-demand
└── assets/            # Optional: templates, images, fonts
```

## Creation Workflow

1. **Understand** → Gather concrete usage examples from user
2. **Plan** → Identify reusable resources (scripts/references/assets)
3. **Initialize** → `scripts/init_skill.py <name> --path <dir>`
4. **Implement** → Create resources, write SKILL.md
5. **Package** → `scripts/package_skill.py <skill-folder>`
6. **Iterate** → Test and refine

## Writing SKILL.md

**Frontmatter** (required):

```yaml
---
name: kebab-case-name
description: What it does + WHEN to use it (triggers). Max 1024 chars.
---
```

**Body**: Instructions for using the skill. Keep under 500 lines.

## Core Principles

- **Token efficiency**: Only add what Claude doesn't already know
- **Progressive disclosure**: Metadata always loaded → SKILL.md on trigger → references on demand
- **Degrees of freedom**: Match specificity to task fragility (narrow bridge = guardrails, open field = flexibility)

## Reference Files

Consult these for detailed patterns:

| Need                   | Reference                                                                    |
| ---------------------- | ---------------------------------------------------------------------------- |
| Workflow patterns      | [references/workflows.md](references/workflows.md)                           |
| Output templates       | [references/output-patterns.md](references/output-patterns.md)               |
| Progressive disclosure | [references/progressive-disclosure.md](references/progressive-disclosure.md) |
| Resource organization  | [references/resources.md](references/resources.md)                           |

## Quick Reference

**Do include**: Procedural knowledge, domain-specific details, reusable scripts/templates
**Don't include**: README.md, CHANGELOG.md, setup guides, user-facing docs

**Description tips**: Include file types, task triggers, specific scenarios. Example: "Comprehensive document creation and editing for .docx files: creating, modifying, tracked changes, comments."
