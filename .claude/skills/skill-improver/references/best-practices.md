# Skill Best Practices Checklist

Use this checklist when auditing skills. Check each item and note violations.

## Description (Max 1024 chars)

- [ ] Focuses on WHEN to use, not WHAT it is
- [ ] Uses numbered trigger list format: "Use when: (1)..., (2)..., (3)..."
- [ ] Includes specific scenarios, file types, or task triggers
- [ ] Avoids vague language ("helps with", "a tool for")
- [ ] Is concise (under 1024 characters)

**Good examples:**

- "Use when: (1) filling PDF forms, (2) extracting text from PDFs, (3) merging/splitting PDFs"
- "Comprehensive document creation and editing for .docx files: creating, modifying, tracked changes, comments"

**Bad examples:**

- "A helpful tool for working with documents" (vague)
- "PDF Processing Skill" (describes what, not when)

## SKILL.md Body (<500 lines)

- [ ] Under 500 lines total
- [ ] Contains only procedural knowledge Claude doesn't already have
- [ ] No duplication with references/ content
- [ ] Structure matches skill purpose (workflow/task/reference-based)
- [ ] Clear workflow or decision tree for complex skills

## What to Include

- [ ] Procedural knowledge specific to the domain
- [ ] Domain-specific details Claude wouldn't know
- [ ] Reusable scripts for deterministic operations
- [ ] Templates for consistent output format
- [ ] Reference docs loaded on-demand

## What NOT to Include

- [ ] No README.md files
- [ ] No CHANGELOG.md files
- [ ] No setup/installation guides
- [ ] No user-facing documentation
- [ ] No information Claude already knows (general programming, common libraries)

## Resource Organization

### scripts/

- [ ] Contains only executable code
- [ ] Used for deterministic operations
- [ ] Used for frequently-rewritten code
- [ ] Examples: `validate_schema.py`, `fill_form.py`

### references/

- [ ] Contains detailed documentation
- [ ] One level deep from SKILL.md
- [ ] Files >100 lines have table of contents
- [ ] No overlap with SKILL.md content

### assets/

- [ ] Contains files used in output (not loaded to context)
- [ ] Examples: templates, images, fonts, boilerplate

## Progressive Disclosure

- [ ] Metadata (~100 words) always in context
- [ ] SKILL.md body (<5k words) loaded on trigger
- [ ] References loaded only when needed
- [ ] Large reference files include grep patterns in SKILL.md

## Degrees of Freedom

- [ ] High-stakes/fragile tasks → strict templates and guardrails
- [ ] Flexible/creative tasks → guidelines with room for judgment
- [ ] Match specificity to task fragility
