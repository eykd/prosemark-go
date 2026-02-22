# Resource Organization

## scripts/

Executable code for tasks requiring deterministic reliability or frequently rewritten.

**When to include**: Same code rewritten repeatedly, deterministic reliability needed
**Examples**: `rotate_pdf.py`, `fill_form.py`, `validate_schema.py`
**Benefits**: Token efficient, deterministic, can execute without loading to context
**Note**: Scripts may still need reading for patching or environment adjustments

## references/

Documentation loaded into context on-demand.

**When to include**: Detailed info Claude should reference while working
**Examples**:

- `schema.md` - database schemas
- `api_docs.md` - API specifications
- `policies.md` - company policies
- `workflow_guide.md` - detailed procedures

**Best practices**:

- Keep SKILL.md lean; move details here
- For files >10k words, include grep patterns in SKILL.md
- No duplication between SKILL.md and references

## assets/

Files used in output, not loaded into context.

**When to include**: Files copied/modified in final output
**Examples**:

- `template.pptx` - PowerPoint templates
- `logo.png` - brand assets
- `frontend-template/` - HTML/React boilerplate
- `font.ttf` - typography

**Benefits**: Separates output resources from documentation

## What NOT to Include

- README.md
- INSTALLATION_GUIDE.md
- CHANGELOG.md
- User-facing documentation
- Setup/testing procedures

Skills contain only what AI needs to do the job.
