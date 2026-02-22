# Progressive Disclosure Patterns

## Three-Level Loading

1. **Metadata** (~100 words) - Always in context
2. **SKILL.md body** (<5k words) - When skill triggers
3. **Bundled resources** (unlimited) - As needed

## Pattern 1: High-level Guide with References

```markdown
# PDF Processing

## Quick start

Extract text with pdfplumber: [code example]

## Advanced features

- **Form filling**: See [FORMS.md](FORMS.md)
- **API reference**: See [REFERENCE.md](REFERENCE.md)
```

Claude loads FORMS.md only when needed.

## Pattern 2: Domain Organization

For multi-domain skills, organize by domain:

```
bigquery-skill/
├── SKILL.md (overview + navigation)
└── references/
    ├── finance.md
    ├── sales.md
    └── product.md
```

User asks about sales → Claude reads only sales.md.

## Pattern 3: Framework Variants

```
cloud-deploy/
├── SKILL.md (workflow + selection)
└── references/
    ├── aws.md
    ├── gcp.md
    └── azure.md
```

User chooses AWS → Claude reads only aws.md.

## Guidelines

- Keep references one level deep from SKILL.md
- For files >100 lines, include table of contents at top
- Information lives in SKILL.md OR references, not both
- Move detailed info to references; keep SKILL.md for workflow
