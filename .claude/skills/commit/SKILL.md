---
name: commit
description: 'Use when: (1) committing session changes, (2) creating conventional commits, (3) handling pre-commit hook failures. Go projects with lefthook.'
allowed-tools: [Bash, Read]
---

# Commit Changes

Create git commits following conventional commit format.

## Process

### 1. Review Changes

```bash
git status
git diff
```

### 2. Stage and Commit

```bash
# Stage specific files (never use -A or .)
git add file1.go file2.go internal/domain/

# Commit with proper format
git commit -m "feat: add new feature

- Implement core functionality
- Add comprehensive test coverage"

git log --oneline -n 3
```

## Commit Message Format

```
<type>: <subject>

<body>
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`

## Line Length Rules (CRITICAL)

Commit messages enforce **100-character maximum** for ALL lines:

- **Subject line**: Max 100 characters (including type and colon)
- **Body lines**: Max 100 characters **per line**

**Common mistake**: Writing long sentences that exceed 100 characters.

**Solution**: Wrap lines manually at natural break points.

### Examples

Bad (line too long):

```bash
git commit -m "docs: amend constitution to v1.2.1 (strengthen 100% coverage enforcement)

Add comprehensive guidance for achieving 100% test coverage across multiple documentation layers."
# Error: body-max-line-length
```

Good (lines wrapped):

```bash
git commit -m "docs: amend constitution to v1.2.1 (strengthen coverage)

Add comprehensive guidance for achieving 100% test coverage across
multiple documentation layers."
```

### Formatting Lists

When including lists or multiple points, keep each line under 100 chars:

```bash
git commit -m "$(cat <<'EOF'
feat: add authentication system

Implement user authentication with the following features:
- JWT token generation and validation
- Secure password hashing with Argon2id
- Session management
- Rate limiting for login attempts

All features include comprehensive test coverage.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

### Tips for Staying Under 100 Characters

1. **Break at conjunctions**: "and", "but", "or", "with"
2. **Break after punctuation**: Periods, commas, colons
3. **Use HEREDOC** for complex messages (see example above)
4. **Check line length** if commit fails with `body-max-line-length`

## Important Rules

**Stage files explicitly:**

- Never use `git add -A` or `git add .`
- Always specify files: `git add file1.go internal/domain/`

**Message quality:**

- Use imperative mood ("add feature" not "added feature")
- Subject line: max 100 characters
- Body: explain WHY, not just WHAT

## Pre-commit Hooks

This project uses lefthook for automatic validation:

- gofmt formatting check
- go vet (zero warnings)
- staticcheck (zero warnings)
- go test with 100% coverage (non-Impl functions)

Hooks run automatically. If commit fails, fix issues and retry.

## Reference

- **[troubleshooting.md](references/troubleshooting.md)** -- Hook failures, message validation, commit scenarios
