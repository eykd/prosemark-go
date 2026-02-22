---
name: quality-review
description: Review code for quality, correctness, and test quality. Use when reviewing PR changes for code quality without security or architecture concerns (those have dedicated skills).
---

# Quality Review Skill

Review pull requests for code quality, correctness, and test quality while excluding security and architecture concerns (handled by dedicated skills).

## Review Process

1. **Get PR diff**: Use `gh pr diff <number>` to get branch changes
2. **Check known issues**: Read `known-issues.json` if present to avoid re-reporting tracked beads tasks
3. **Analyze changes**: Evaluate across 5 focus areas
4. **Organize findings**: Categorize by priority level (Must Fix, Should Fix, Consider)
5. **Post review**: Use `gh pr comment` to post structured review

## Focus Areas

### 1. What Changed

Provide a plain English summary of the changes for non-technical stakeholders:

- **What functionality was added/changed/removed?**
- **Why would someone care about this change?**
- **What user-visible behavior changed?**

Avoid implementation details - focus on the "what" and "why", not the "how".

### 2. Does It Work

Evaluate correctness and production-readiness:

- **Correctness**: Does the code do what it's supposed to do?
- **Edge cases**: Are error conditions and boundary cases handled?
- **Testing**: Are there tests that demonstrate it works?
- **Production-ready**: Will this work in production environments?

**Critical checks**:

- All paths tested
- Error handling present
- No obvious bugs
- Follows expected behavior

### 3. Test Quality

See [references/test-desiderata.md](references/test-desiderata.md) for detailed guidance.

Evaluate using Kent Beck's Test Desiderata:

- **Isolated**: Can tests run independently?
- **Composable**: Can tests run in any combination?
- **Fast**: Quick feedback?
- **Deterministic**: Same result every run?
- **Readable**: Clear what's being tested?
- **Behavioral**: Tests outcomes, not implementation?
- **Specific**: Clear failure messages?
- **Predictive**: Catches real bugs?

Check for anti-patterns:

- Missing assertions
- Shared mutable state
- Excessive mocking
- Logic in tests

**Coverage verification**:

- Run `just test-cover-check` for coverage
- Project requires 100% coverage of non-Impl functions
- Flag any coverage below 100% as HIGH severity

### 4. Simplicity

Evaluate maintainability and code clarity:

- **Simple vs complex**: Is this the simplest solution that works?
- **Readable**: Can another developer understand this quickly?
- **Maintainable**: Will this be easy to modify later?
- **Appropriate abstractions**: Right level of abstraction for the problem?

**Watch for**:

- Over-engineering (unnecessary abstractions, premature optimization)
- Under-engineering (copy-paste code, missing obvious patterns)
- Clever code (hard to understand, shows off rather than solves)

### 5. Code Quality

Check compliance with CLAUDE.md project standards:

- **go vet compliance**: Zero warnings
- **staticcheck compliance**: Zero warnings, no deprecated APIs
- **Naming conventions**: Exported/unexported, idiomatic Go naming
- **GoDoc**: All exported functions/methods/types documented
- **Import order**: Standard library, then third-party, then local
- **Formatting**: gofmt compliance

**Project-specific rules**:

- 100% test coverage threshold (non-Impl functions)
- Conventional commit format
- Zero warnings policy
- All errors handled explicitly (no `_` for errors)

## Known Issues Handling

See [references/known-issues.md](references/known-issues.md) for details.

Before reporting findings:

1. Check if `known-issues.json` exists in workspace
2. Parse JSON for tasks with status "open" or "ready"
3. If finding relates to a known issue, reference it: "Related to beads task workspace-123"
4. Only report NEW findings not already tracked

## Priority Levels

See [references/priority-levels.md](references/priority-levels.md) for definitions.

### Must Fix (Blocks Merge)

- Correctness issues or bugs
- Failing tests or missing test coverage
- Project standard violations (CLAUDE.md)
- Breaking changes without justification

### Should Fix (Important but Not Blocking)

- Test quality improvements
- Simplicity enhancements
- Missing edge case handling
- Maintainability concerns

### Consider (Optional Suggestions)

- Style preferences beyond standards
- Future enhancement ideas
- Alternative approaches
- Minor optimizations

## Audience: Non-Technical Managers

**CRITICAL**: Write the ENTIRE review for non-technical managers, not developers.

- Technical jargon is OK when explained briefly and concisely
- Always follow technical terms with plain English explanation (e.g., "mocking - simulating external systems in tests")
- Explain impacts in business terms (risk, cost, user experience)
- Focus on "what" and "why", not implementation details
- Keep explanations concise - don't over-explain
- Keep all sections accessible to non-technical readers

## Output Format

Post as PR comment with this structure:

```markdown
## Code Quality Review

### What Changed

[Plain English summary - 2-3 sentences for non-technical readers]

### Does It Work

[Assessment of correctness and production-readiness]

### Test Quality

[Evaluation against Test Desiderata, coverage check, anti-patterns]

### Simplicity

[Assessment of code clarity and maintainability]

### Code Quality

[CLAUDE.md standards compliance check]

---

## Findings

### Must Fix

1. **[Issue Title]** - [File:Line]
   - **Problem**: [Description]
   - **Impact**: [Why this matters]
   - **Fix**: [How to resolve]

### Should Fix

[Same format as Must Fix]

### Consider

[Same format as Must Fix]

---

## Summary

[Overall assessment - ready to merge, needs fixes, etc.]

---

## Copy-Paste Prompt for Claude Code

**REQUIRED when findings exist**: Provide a ready-to-use prompt in a code block.
```

[Specific, actionable prompt with file paths and line numbers that addresses all Must Fix and Should Fix items]

```
```

## Integration Notes

### Relationship to Other Review Skills

- **security-review**: Handles OWASP vulnerabilities, auth, data security
- **clean-architecture-validator**: Handles dependency violations, layer boundaries
- **quality-review** (this skill): Handles correctness, test quality, simplicity, code standards

**DO NOT overlap** - if a finding is about security or architecture, don't report it here.

### GitHub Actions Context

This skill is designed for GitHub Actions workflows with:

- `gh` CLI available for PR operations
- `known-issues.json` generated from beads tasks
- CLAUDE.md project standards in repository
- Go test framework with coverage reporting

### Local Usage

Can also be used locally for PR review:

```bash
# Review a specific PR
/quality-review

# Skill will prompt for PR number if not in GitHub Actions context
```

## Edge Cases

- **No changes**: Report "No changes detected" and exit gracefully
- **No known-issues.json**: Skip known issues check, report all findings
- **Very large PRs**: Focus on highest-impact changes, note review limitations
- **No tests changed**: Skip test quality section, focus on other areas
