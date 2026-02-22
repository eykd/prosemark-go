# Output Patterns

## Template Pattern

**Strict (API responses, data formats):**

```markdown
## Report structure

ALWAYS use this exact template:

# [Title]

## Executive summary

[One paragraph overview]

## Key findings

- Finding 1 with data
- Finding 2 with data

## Recommendations

1. Actionable item
2. Actionable item
```

**Flexible (when adaptation useful):**

```markdown
## Report structure

Sensible default—use judgment:

# [Title]

## Executive summary

[Overview]

## Key findings

[Adapt to discoveries]

## Recommendations

[Tailor to context]
```

## Examples Pattern

For quality-dependent output, provide input/output pairs:

```markdown
## Commit messages

**Example 1:**
Input: Added user authentication with JWT tokens
Output: feat(auth): implement JWT-based authentication

**Example 2:**
Input: Fixed bug where dates displayed incorrectly
Output: fix(reports): correct date formatting in timezone conversion

Follow: type(scope): brief description
```

Examples communicate style better than descriptions.
