---
description: Perform adversarial review of spec and plan to identify security, edge cases, performance, and accessibility concerns. Enhances plan.md with findings before task generation.
---

## Red Team Purpose: Adversarial Plan Enhancement

**CRITICAL CONCEPT**: This phase performs **ADVERSARIAL REVIEW** of requirements and design to strengthen the plan BEFORE implementation tasks are generated.

**What Red Team Does**:

- Reviews spec.md and plan.md from an attacker/critic perspective
- Identifies gaps, implicit assumptions, and missing considerations
- Looks for what's NOT in the plan but should be
- Enhances plan.md with security, edge case, performance, and accessibility strategies

**What Red Team is NOT**:

- ❌ NOT implementation review (that's sp:08-security-review)
- ❌ NOT code review (happens after implementation)
- ❌ NOT artifact consistency check (that's sp:06-analyze)
- ❌ NOT requirement quality validation (integrated into spec creation)

**Key Distinction from sp:08-security-review**:

- **sp:04-red-team**: Reviews REQUIREMENTS and DESIGN (spec.md + plan.md)
  - Happens BEFORE implementation
  - Enhances plan.md with adversarial considerations
  - Prevents problems by strengthening design
  - Output: Improved plan with security/edge case/performance/A11y sections
- **sp:08-security-review**: Reviews IMPLEMENTATION CODE (git diff)
  - Happens AFTER implementation
  - Creates beads tasks for code-level security issues
  - Catches implementation vulnerabilities
  - Output: List of code-level security issues to fix

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Execution Steps

### 1. Setup

Run `.specify/scripts/bash/check-prerequisites.sh --json` from repo root and parse JSON for FEATURE_DIR.

- All file paths must be absolute.
- For single quotes in args like "I'm Groot", use escape syntax: `'I'\''m Groot'` (or double-quote: `"I'm Groot"`)

### 2. Load Context

Read from FEATURE_DIR:

- `spec.md`: Feature requirements and scope
- `plan.md`: Technical design and implementation approach

If plan.md doesn't exist:

- ERROR: "No plan.md found. Run `/sp:03-plan` first."

### 3. Check Prior Learnings for Known Vulnerability Patterns

Before performing adversarial analysis, search `.specify/solutions/` for previously documented issues. If the directory does not exist, skip this step silently.

1. Search `.specify/solutions/security/` and `.specify/solutions/clean-architecture/` for solutions matching the feature's domain.
2. For each match, check if plan.md already addresses the concern.
3. If the plan does NOT address a previously documented pattern, include it as a **HIGH severity** finding in Step 4, referencing the original solution document.

This ensures the team does not repeat previously solved problems.

### 4. Perform Adversarial Analysis

Analyze the spec and plan from an adversarial perspective across these categories:

#### Security/Privacy Analysis

Think like an attacker:

- What authentication/authorization gaps exist?
- What data exposure risks are present?
- What injection vulnerabilities could occur (SQL, XSS, CSRF, command injection)?
- How could malicious users abuse this feature?
- What sensitive data needs protection?
- Are there rate limiting or brute force concerns?
- What session hijacking or token theft risks exist?

#### Edge Case Analysis

Think about boundary conditions:

- What happens with null/empty/missing inputs?
- How does the system handle max/min limits?
- What about race conditions or concurrent access?
- Are there timing issues or state conflicts?
- What happens when dependencies fail?
- How does partial failure affect the system?
- What about data consistency during errors?

#### Error Scenario Analysis

Think about failure modes:

- What error paths exist?
- How does the system recover from failures?
- What happens when external services are down?
- How are timeouts handled?
- What about partial data updates?
- How does the system degrade gracefully?
- Are error messages secure (no information leakage)?

#### Performance Analysis

Think about stress conditions:

- What bottlenecks exist under load?
- Are there N+1 query problems?
- What about memory leaks or resource exhaustion?
- How does caching strategy affect performance?
- What happens with large datasets?
- Are there unbounded operations?
- What about database query optimization?

#### Accessibility Analysis

Think about barriers:

- Is keyboard navigation fully supported?
- Are screen readers properly supported?
- What about color contrast and readability?
- Are ARIA attributes needed?
- How does focus management work?
- What about alternative text for images?
- Are forms properly labeled?

#### Misuse/Abuse Analysis

Think like someone trying to break it:

- How could users game the system?
- What resource exhaustion attacks are possible?
- How could automation/bots abuse this?
- What happens with invalid or malicious input?
- Are there DoS attack vectors?
- How could users bypass intended workflows?

### 5. Generate Structured Findings

For each concern identified, create a structured finding:

```typescript
interface Finding {
  category: 'Security' | 'EdgeCase' | 'ErrorHandling' | 'Performance' | 'Accessibility' | 'Misuse';
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  title: string; // Brief description (50 chars max)
  description: string; // Detailed explanation of the concern
  planEnhancement: string; // Specific content to add to plan.md
  location: string; // Where in plan.md ("## Security Considerations" or "new section")
}
```

**Severity Guidelines**:

- **Critical**: Security vulnerabilities, data loss risks, system-breaking issues
- **High**: Significant edge cases, major performance problems, accessibility barriers
- **Medium**: Important considerations that should be addressed
- **Low**: Nice-to-have improvements, documentation needs

**Quality Standards**:

- Be specific and actionable (not generic/boilerplate)
- Reference specific parts of the spec/plan
- Provide concrete mitigation strategies
- Focus on what's MISSING or WEAK in the current plan
- For simple features: may find minimal issues (that's OK!)
- For complex features: should find substantial concerns

### 6. Enhance plan.md

For each category with findings, add or enhance sections in plan.md:

#### Security Findings → "## Security Considerations"

```markdown
## Security Considerations

### Authentication & Authorization

- [Specific concern and mitigation strategy from findings]

### Input Validation

- [Validation requirements to prevent injection]

### Data Protection

- [Encryption, access control strategies]

### Rate Limiting

- [Protection against abuse/DoS]
```

#### Edge Case Findings → "## Edge Cases & Error Handling"

```markdown
## Edge Cases & Error Handling

### Boundary Conditions

- Null/empty inputs: [handling strategy]
- Max limits: [graceful degradation approach]

### Race Conditions

- [Concurrency handling approach]

### Dependency Failures

- [Fallback strategies when external services fail]
```

#### Performance Findings → "## Performance Considerations"

```markdown
## Performance Considerations

### Query Optimization

- [Database query strategies]
- [Pagination/limit strategies]

### Caching Strategy

- [What to cache, TTL, invalidation]

### Resource Limits

- [Max sizes, timeouts, rate limits]
```

#### Accessibility Findings → "## Accessibility Requirements"

```markdown
## Accessibility Requirements

### Keyboard Navigation

- [Focus management, shortcuts, tab order]

### Screen Reader Support

- [ARIA labels, semantic HTML, announcements]

### Visual Design

- [Color contrast ratios, font sizing, spacing]
```

**Enhancement Strategy**:

- Use Edit tool to add new sections if they don't exist
- Enhance existing sections with findings if they already exist
- Maintain plan structure and readability
- Keep content specific and actionable
- Group related findings together
- Preserve existing plan content

### 7. Close Phase Task

After enhancing plan.md, close the red team phase task:

a. Read the epic ID from spec.md front matter:

```bash
grep "Beads Epic" $FEATURE_DIR/spec.md | grep -oE 'workspace-[a-z0-9]+|bd-[a-z0-9]+'
```

b. Find the red team phase task:

```bash
npx bd list --parent <epic-id> --status open --json | jq -r '.[] | select(.title | contains("[sp:04-red-team]")) | .id'
```

c. Close the task with summary:

```bash
npx bd close <red-team-task-id> --reason "Red team review complete: Enhanced plan with <N> findings (<Critical>C/<High>H/<Medium>M/<Low>L) across <X> categories"
```

### 8. Report Completion

Output a comprehensive summary:

```markdown
## Red Team Review Complete

**Plan Enhanced With:**

- Security considerations: X findings
- Edge case handling: Y findings
- Performance optimizations: Z findings
- Accessibility requirements: W findings
- Error handling strategies: V findings

**Findings by Severity:**

- Critical: A findings
- High: B findings
- Medium: C findings
- Low: D findings

**Sections Added/Enhanced in plan.md:**

- [List of sections modified with brief description]

**Analysis Summary:**
[1-2 paragraph summary of the most critical findings and how they strengthen the plan]

**Next Steps:**

- Review updated plan.md in `$FEATURE_DIR/plan.md`
- Run `/sp:next` to proceed to sp:05-tasks
- Implementation tasks will include adversarial considerations from enhanced plan
```

## Adversarial Analysis Guidelines

### Think Like an Attacker

- How would you break this system?
- What assumptions can you exploit?
- What did the designer forget to consider?
- Where are the trust boundaries?
- What happens when trust is violated?

### Think Like a Critic

- What's vague or unclear?
- What edge cases are missing?
- What happens under stress?
- What could go wrong?
- What's the worst case scenario?

### Think Like a User

- How might I misuse this?
- What if I provide garbage input?
- What if I do things out of order?
- What if I hit the system really hard?
- What if I try to bypass the intended flow?

### Think Like a Tester

- What aren't they testing for?
- What scenarios are missing?
- What boundary conditions exist?
- What failure modes are possible?
- How could this fail silently?

## Quality Standards

### DO Generate Findings For:

- ✅ Specific security vulnerabilities with concrete attack vectors
- ✅ Edge cases with concrete scenarios and handling strategies
- ✅ Performance bottlenecks with specific optimization approaches
- ✅ Accessibility barriers with specific WCAG criteria
- ✅ Missing error handling for specific failure modes
- ✅ Concrete race conditions or concurrency issues

### DON'T Generate Findings For:

- ❌ Generic security advice ("use strong passwords")
- ❌ Boilerplate recommendations ("follow best practices")
- ❌ Vague concerns ("might have performance issues")
- ❌ Already covered in plan ("plan already addresses authentication")
- ❌ Out of scope items (not relevant to this feature)
- ❌ Implementation details (language-specific patterns)

## Examples

### Simple Feature: "Add a footer link"

**Findings**: Minimal (2-3 findings)

- Edge case: What if link target is missing/invalid?
- Accessibility: Ensure link has descriptive text for screen readers
- Performance: N/A (no performance concerns for static link)

**Plan Enhancement**: Small additions

- Add edge case handling section
- Add accessibility requirement to existing UI section

### Complex Feature: "User authentication system"

**Findings**: Extensive (15-20 findings)

- Security: Password hashing, session management, CSRF, timing attacks, brute force
- Edge cases: Concurrent logins, session expiry, invalid tokens, missing sessions
- Performance: Rate limiting, query optimization, session store scaling
- Accessibility: Keyboard nav, screen reader announcements, error messages
- Error handling: Failed login, expired session, invalid credentials

**Plan Enhancement**: Major additions

- New Security Considerations section with 5+ subsections
- Comprehensive Edge Cases section
- Performance section with caching/rate limiting
- Enhanced accessibility requirements

### UI Feature: "Interactive dashboard"

**Findings**: Moderate (8-12 findings)

- Security: XSS in user content, CSRF for actions
- Edge cases: Empty states, loading states, no data, max data
- Performance: Lazy loading, pagination, debouncing
- Accessibility: Keyboard navigation, focus management, ARIA labels
- Error handling: Network errors, stale data, partial updates

**Plan Enhancement**: Multiple new sections

- Security section for XSS/CSRF mitigation
- Edge cases for states and boundaries
- Performance section for optimization strategies
- Accessibility section for interaction patterns

## Skip Scenario

If user skips red team generation:

```bash
npx bd close <red-team-task-id> --reason "Skipped: Red team review deferred"
```

Note: Skipping red team means the plan will not be enhanced with adversarial considerations. Implementation tasks will be generated from the original plan, which may miss security, edge case, performance, or accessibility concerns.
