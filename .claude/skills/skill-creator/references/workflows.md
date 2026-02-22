# Workflow Patterns

## Sequential Workflows

For complex tasks, break operations into clear steps with overview:

```markdown
Filling a PDF form involves:

1. Analyze form (run analyze_form.py)
2. Create field mapping (edit fields.json)
3. Validate mapping (run validate_fields.py)
4. Fill form (run fill_form.py)
5. Verify output (run verify_output.py)
```

## Conditional Workflows

Guide through decision points:

```markdown
1. Determine modification type:
   **Creating new?** → Follow "Creation workflow"
   **Editing existing?** → Follow "Editing workflow"

2. Creation workflow: [steps]
3. Editing workflow: [steps]
```

## Workflow Decision Trees

For skills with multiple paths, use decision trees at the top:

```markdown
## Workflow Decision Tree

**What do you need to do?**
├── Read/analyze document → § Reading Documents
├── Create new document → § Creating Documents
├── Edit existing document → § Editing Documents
└── Convert format → § Format Conversion
```
