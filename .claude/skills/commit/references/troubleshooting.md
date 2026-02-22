# Commit Troubleshooting

## Pre-commit Hook Failures

### Formatting Errors

```bash
# Check formatting
just fmt

# gofmt will reformat files in-place, then retry commit
```

### Vet/Lint Errors

```bash
# Run go vet
just vet

# Run staticcheck
just lint

# Fix errors and retry commit
```

### Test Failures

```bash
# Run tests
just test

# Run tests with coverage to find gaps
just test-cover

# Fix failing tests and retry commit
```

## Commit Message Validation Failures

### Common Errors

**Error: `body-max-line-length`**

One or more body lines exceed 100 characters.

Fix: Manually wrap long lines at natural break points:

```bash
# Before (fails)
git commit -m "feat: add feature

This is a very long line that exceeds one hundred characters and will cause the commit to fail."

# After (passes)
git commit -m "feat: add feature

This is a very long line that exceeds one hundred characters and
will cause the commit to fail."
```

**Other validation rules:**

- Ensure commit message follows conventional commit format
- Check that type is valid (feat, fix, docs, etc.)
- Subject line should be lowercase
- No period at end of subject line
- Subject line max 100 characters

## Go Project Commit Scenarios

### New Feature with Tests

```
feat: add [feature name]

- Implement [component/function]
- Add comprehensive test coverage
- Update interfaces as needed
```

### Bug Fix

```
fix: resolve [issue description]

- Fix [specific problem]
- Add regression test
```

### Configuration Changes

```
chore: update [tool] configuration

- Adjust [setting] for [reason]
- Impact: [description]
```

### Dependency Updates

```
chore: update dependencies

- Update [package] to v[version]
- Reason: [security/feature/bugfix]
```
