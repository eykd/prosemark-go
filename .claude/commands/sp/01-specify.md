---
description: Create or update the feature specification from a natural language description, using an explicit one-question-at-a-time user interview. Creates a beads epic for task tracking.
handoffs:
  - label: Build Technical Plan
    agent: sp:03-plan
    prompt: Create a plan for the spec. I am building with...
  - label: Clarify Spec Requirements
    agent: sp:02-clarify
    prompt: Clarify specification requirements
    send: true
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Outline

## Interview Protocol (One Question at a Time)

This command is **explicitly interactive**.

- If this is the **first** run for this feature, `$ARGUMENTS` is the initial feature description.
- If a spec already exists for the active feature **and** the spec contains an open interview question, then treat `$ARGUMENTS` as the user's answer to the **most recently asked** question (unless `$ARGUMENTS` is empty).
- You MUST ask **exactly one** question per run, then stop and wait for the user's answer in the next message (or next `/sp:01-specify` invocation).
- Persist the interview state **inside the spec** (so the workflow is robust across sessions):
  - Add/maintain an `## Interview` section in `spec.md` with:
    - `### Open Questions` (ordered list)
    - `### Answer Log` (Q → A pairs with dates)
    - A clear marker for the **next** question to ask (e.g. `**NEXT QUESTION:** #3`).
- The interview ends only when there are **no open questions** and there are **no** `[NEEDS CLARIFICATION]` markers in `spec.md`.

After the interview is complete, proceed to (or hand off into) `/sp:03-plan`.

## Glossary Terms Discovery

During the interview, identify any new domain terms introduced by the user:

1. **For each potential domain term** (nouns, verbs describing business concepts):
   - Check if term exists in `docs/glossary.md`
   - If new term: Ask clarifying question about its meaning (e.g., "I notice you're using the term 'campaign'. Can you clarify what that means in your domain?")
   - Add to glossary with user-provided definition

2. **Use `/glossary` skill to**:
   - Validate terminology consistency in spec
   - Identify synonyms that need consolidation
   - Ensure all domain concepts are captured

This ensures Ubiquitous Language is established during specification phase.

The text the user typed after `/sp:01-specify` in the triggering message **is** the feature description. Assume you always have it available in this conversation even if `$ARGUMENTS` appears literally below. Do not ask the user to repeat it unless they provided an empty command.

Given that feature description, do this:

1. **Initialize Beads (if needed)**:

   a. Check if beads is already initialized:

   ```bash
   test -d .beads && echo "initialized" || echo "not_initialized"
   ```

   b. If not initialized, initialize beads:

   ```bash
   npx bd init
   ```

   - This creates the `.beads/` directory for git-backed task tracking
   - If initialization fails, display error and suggest running `npm install --save-dev @beads/bd` first

   c. Verify initialization succeeded:

   ```bash
   test -d .beads && echo "beads ready" || echo "ERROR: beads initialization failed"
   ```

2. **Generate a concise short name** (2-4 words) for the branch:
   - Analyze the feature description and extract the most meaningful keywords
   - Create a 2-4 word short name that captures the essence of the feature
   - Use action-noun format when possible (e.g., "add-user-auth", "fix-payment-bug")
   - Preserve technical terms and acronyms (OAuth2, API, JWT, etc.)
   - Keep it concise but descriptive enough to understand the feature at a glance
   - Examples:
     - "I want to add user authentication" → "user-auth"
     - "Implement OAuth2 integration for the API" → "oauth2-api-integration"
     - "Create a dashboard for analytics" → "analytics-dashboard"
     - "Fix payment processing timeout bug" → "fix-payment-timeout"

3. **Run the feature creation script**:

   Run the script with the generated short-name, allowing it to auto-detect the next available number:

   ```bash
   .specify/scripts/bash/create-new-feature.sh --json --short-name "<short-name-from-step-2>" "$ARGUMENTS"
   ```

   **How it works**:
   - The script will automatically:
     - Fetch all remote branches (`git fetch --all --prune`)
     - Find the highest number across ALL branches and specs (not filtered by name)
     - Assign the next sequential number (e.g., if 010 exists, use 011)
     - Create the branch and spec directory
   - Replace `<short-name-from-step-2>` with the short name generated in step 2
   - For single quotes in args like "I'm Groot", use escape syntax: `'I'\''m Groot'` (or use double-quotes: `"I'm Groot"`)
   - The script outputs JSON with `BRANCH_NAME` and `SPEC_FILE` paths

   **IMPORTANT**:
   - You must only ever run this script once per feature
   - The JSON is provided in the terminal as output - always refer to it to get the actual content you're looking for
   - Do NOT pass `--number` manually - let the script auto-detect to avoid duplicate numbering

4. Load `.specify/templates/spec-template.md` to understand required sections.

5. Follow this execution flow:
   1. Parse user description from Input
      If empty: ERROR "No feature description provided"
   2. Extract key concepts from description
      Identify: actors, actions, data, constraints
   3. For unclear aspects:
      - Make informed guesses based on context and industry standards
      - Only mark with [NEEDS CLARIFICATION: specific question] if:
        - The choice significantly impacts feature scope or user experience
        - Multiple reasonable interpretations exist with different implications
        - No reasonable default exists
      - **LIMIT: Maximum 3 [NEEDS CLARIFICATION] markers total**
      - Prioritize clarifications by impact: scope > security/privacy > user experience > technical details
   4. Fill User Scenarios & Testing section
      If no clear user flow: ERROR "Cannot determine user scenarios"
   5. Generate Functional Requirements
      Each requirement must be testable
      Use reasonable defaults for unspecified details (document assumptions in Assumptions section)
   6. Define Success Criteria
      Create measurable, technology-agnostic outcomes
      Include both quantitative metrics (time, performance, volume) and qualitative measures (user satisfaction, task completion)
      Each criterion must be verifiable without implementation details
   7. Identify Key Entities (if data involved)
   8. Return: SUCCESS (spec ready for planning)

6. Write the specification to SPEC_FILE using the template structure, replacing placeholders with concrete details derived from the feature description (arguments) while preserving section order and headings.

7. **Create Beads Epic for Feature**:

   a. Check if an epic already exists for this feature branch:

   ```bash
   npx bd list --type epic --status open --json 2>/dev/null | grep -i "<feature-name>" || echo "no_existing_epic"
   ```

   b. If no existing epic, create one:

   ```bash
   npx bd create "Feature: <feature-name>" -t epic -p 0 --description "Spec: specs/<branch>/spec.md" --json
   ```

   - `<feature-name>`: The short name generated in step 2
   - `<branch>`: The BRANCH_NAME from step 3
   - Priority 0 = highest (epic level)
   - Type = epic

   c. Parse the epic ID from the JSON response (format: `workspace-xxxx` or similar)

   d. If epic already exists, retrieve its ID:

   ```bash
   npx bd list --type epic --status open --json | jq -r '.[] | select(.title | contains("<feature-name>")) | .id'
   ```

   e. **Store epic ID in spec.md**: Add the epic ID to the spec.md front matter:

   ```markdown
   **Feature Branch**: `<branch>`
   **Created**: <date>
   **Status**: Draft
   **Beads Epic**: `<epic-id>`
   ```

   f. If epic creation fails, display error but continue with spec creation (beads integration is enhancement, not blocker)

8. **Create Phase Tasks with Dependency Chain**:

   After creating the epic, create ALL phase tasks upfront with dependencies. This enables `/sp:next` to orchestrate the workflow.

   a. Create all nine phase tasks under the epic with structured descriptions:

   ```bash
   # Extract feature name from branch (e.g., "010-user-auth" -> "user-auth")
   FEATURE_NAME="<short-name-from-step-2>"
   BRANCH="<branch-name-from-step-3>"

   # Create phase tasks (store IDs from JSON responses)
   npx bd create "[sp:02-clarify] Clarify requirements for $FEATURE_NAME" -p 1 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md
   **Skills**: None
   **Context**: Interactive clarification - identify ambiguities and resolve through user questions
   **Acceptance**: No [NEEDS CLARIFICATION] markers remain in spec.md" --json
   # Store returned ID as CLARIFY_ID

   npx bd create "[sp:03-plan] Create implementation plan for $FEATURE_NAME" -p 1 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md
   **Skills**: /prefactoring, /latent-features, /glossary
   **Context**: Generate plan.md with technical architecture, data-model.md if needed
   **Acceptance**: All technical decisions documented, file structure defined" --json
   # Store returned ID as PLAN_ID

   npx bd create "[sp:04-red-team] Perform adversarial review for $FEATURE_NAME" -p 2 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md
   **Skills**: None
   **Context**: Adversarial review of spec and plan; enhance plan.md with security, edge cases, performance, accessibility
   **Acceptance**: plan.md enhanced with adversarial findings; red team review complete" --json
   # Store returned ID as RED_TEAM_ID

   npx bd create "[sp:05-tasks] Generate implementation tasks for $FEATURE_NAME" -p 1 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md
   **Skills**: /prefactoring, /glossary
   **Context**: Create beads tasks with skill references and acceptance criteria
   **Acceptance**: All user stories have beads tasks with descriptions" --json
   # Store returned ID as TASKS_ID

   npx bd create "[sp:06-analyze] Analyze artifacts for $FEATURE_NAME" -p 2 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md, beads tasks
   **Skills**: None (read-only analysis)
   **Context**: Validate cross-artifact consistency, coverage gaps, constitution alignment
   **Acceptance**: No CRITICAL issues; coverage report shows all requirements mapped" --json
   # Store returned ID as ANALYZE_ID

   npx bd create "[sp:07-implement] Execute implementation for $FEATURE_NAME" -p 1 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md
   **Skills**: Per-task (see task descriptions)
   **Context**: TDD implementation - strict red-green-refactor for all tasks
   **Acceptance**: All tasks closed, tests pass, 100% coverage maintained" --json
   # Store returned ID as IMPLEMENT_ID

   npx bd create "[sp:08-security-review] Security review for $FEATURE_NAME" -p 2 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md
   **Skills**: /security-review
   **Context**: Review branch diff (<base>..HEAD) for security vulnerabilities; create remediation tasks as needed
   **Acceptance**: No CRITICAL security findings; remediation tasks filed for any remaining issues" --json
   # Store returned ID as SECURITY_REVIEW_ID

   npx bd create "[sp:09-architecture-review] Architecture review for $FEATURE_NAME" -p 2 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md
   **Skills**: /clean-architecture-validator
   **Context**: Review branch diff (<base>..HEAD) for architectural compliance; create remediation tasks as needed
   **Acceptance**: No blocking architecture violations; remediation tasks filed for any remaining issues" --json
   # Store returned ID as ARCH_REVIEW_ID

   npx bd create "[sp:10-code-quality-review] Code quality review for $FEATURE_NAME" -p 2 --parent <epic-id> \
     --description "**Spec**: specs/$BRANCH/spec.md, plan.md
   **Skills**: /quality-review
   **Context**: Review branch diff (<base>..HEAD) for general code quality; create remediation tasks as needed
   **Acceptance**: No blocking quality issues; remediation tasks filed for any remaining issues" --json
   # Store returned ID as QUALITY_REVIEW_ID
   ```

   b. Create the dependency chain (each phase depends on the previous):

   ```bash
   npx bd dep add <PLAN_ID> <CLARIFY_ID>
   npx bd dep add <RED_TEAM_ID> <PLAN_ID>
   npx bd dep add <TASKS_ID> <RED_TEAM_ID>
   npx bd dep add <ANALYZE_ID> <TASKS_ID>
   npx bd dep add <IMPLEMENT_ID> <ANALYZE_ID>
   npx bd dep add <SECURITY_REVIEW_ID> <IMPLEMENT_ID>
   npx bd dep add <ARCH_REVIEW_ID> <SECURITY_REVIEW_ID>
   npx bd dep add <QUALITY_REVIEW_ID> <ARCH_REVIEW_ID>
   ```

   c. Store phase task IDs in spec.md front matter for reference:

   ```markdown
   **Beads Phase Tasks**:

   - clarify: `<CLARIFY_ID>`
   - plan: `<PLAN_ID>`
   - red-team: `<RED_TEAM_ID>`
   - tasks: `<TASKS_ID>`
   - analyze: `<ANALYZE_ID>`
   - implement: `<IMPLEMENT_ID>`
   - review: `<REVIEW_ID>`
   ```

   d. Verify the dependency chain:

   ```bash
   npx bd dep tree <epic-id>
   ```

   Expected output shows the chain: clarify → plan → red-team → tasks → analyze → implement → security-review → architecture-review → code-quality-review

   e. If phase task creation fails, log the error and continue. The workflow can still function with manual skill invocation.

9. **Specification Quality Validation**: After writing the initial spec, validate it against quality criteria:

   a. **Create Spec Quality Checklist**: Generate a checklist file at `FEATURE_DIR/checklists/requirements.md` using the checklist template structure with these validation items:

   ```markdown
   # Specification Quality Checklist: [FEATURE NAME]

   **Purpose**: Validate specification completeness and quality before proceeding to planning
   **Created**: [DATE]
   **Feature**: [Link to spec.md]

   ## Content Quality

   - [ ] No implementation details (languages, frameworks, APIs)
   - [ ] Focused on user value and business needs
   - [ ] Written for non-technical stakeholders
   - [ ] All mandatory sections completed

   ## Requirement Completeness

   - [ ] No [NEEDS CLARIFICATION] markers remain
   - [ ] Requirements are testable and unambiguous
   - [ ] Success criteria are measurable
   - [ ] Success criteria are technology-agnostic (no implementation details)
   - [ ] All acceptance scenarios are defined
   - [ ] Edge cases are identified
   - [ ] Scope is clearly bounded
   - [ ] Dependencies and assumptions identified

   ## Feature Readiness

   - [ ] All functional requirements have clear acceptance criteria
   - [ ] User scenarios cover primary flows
   - [ ] Feature meets measurable outcomes defined in Success Criteria
   - [ ] No implementation details leak into specification

   ## Notes

   - Items marked incomplete require spec updates before `/sp:02-clarify` or `/sp:03-plan`
   ```

   b. **Run Validation Check**: Review the spec against each checklist item:
   - For each item, determine if it passes or fails
   - Document specific issues found (quote relevant spec sections)

   c. **Handle Validation Results**:
   - **If all items pass**: Mark checklist complete and proceed to step 9

   - **If items fail (excluding [NEEDS CLARIFICATION])**:
     1. List the failing items and specific issues
     2. Update the spec to address each issue
     3. Re-run validation until all items pass (max 3 iterations)
     4. If still failing after 3 iterations, document remaining issues in checklist notes and warn user

   - **If [NEEDS CLARIFICATION] markers remain**:
     1. Extract all [NEEDS CLARIFICATION: ...] markers from the spec
     2. **LIMIT CHECK**: If more than 3 markers exist, keep only the 3 most critical (by scope/security/UX impact) and make informed guesses for the rest
     3. For each clarification needed (max 3), present options to user in this format:

        ```markdown
        ## Question [N]: [Topic]

        **Context**: [Quote relevant spec section]

        **What we need to know**: [Specific question from NEEDS CLARIFICATION marker]

        **Suggested Answers**:

        | Option | Answer                    | Implications                          |
        | ------ | ------------------------- | ------------------------------------- |
        | A      | [First suggested answer]  | [What this means for the feature]     |
        | B      | [Second suggested answer] | [What this means for the feature]     |
        | C      | [Third suggested answer]  | [What this means for the feature]     |
        | Custom | Provide your own answer   | [Explain how to provide custom input] |

        **Your choice**: _[Wait for user response]_
        ```

     4. **CRITICAL - Table Formatting**: Ensure markdown tables are properly formatted:
        - Use consistent spacing with pipes aligned
        - Each cell should have spaces around content: `| Content |` not `|Content|`
        - Header separator must have at least 3 dashes: `|--------|`
        - Test that the table renders correctly in markdown preview
     5. Number questions sequentially (Q1, Q2, Q3 - max 3 total)
     6. Present all questions together before waiting for responses
     7. Wait for user to respond with their choices for all questions (e.g., "Q1: A, Q2: Custom - [details], Q3: B")
     8. Update the spec by replacing each [NEEDS CLARIFICATION] marker with the user's selected or provided answer
     9. Re-run validation after all clarifications are resolved

   d. **Update Checklist**: After each validation iteration, update the checklist file with current pass/fail status

10. Report completion with:

- Branch name
- Spec file path
- **Beads epic ID** (if created successfully)
- **Phase tasks created** (list all 7 phase task IDs)
- **Dependency chain visualization** (from `bd dep tree`)
- Checklist results
- **Next step**: Run `/sp:next` to start the workflow, or `/sp:02-clarify` directly

**NOTE:** The script creates and checks out the new branch and initializes the spec file before writing.

## General Guidelines

## Quick Guidelines

- Focus on **WHAT** users need and **WHY**.
- Avoid HOW to implement (no tech stack, APIs, code structure).
- Written for business stakeholders, not developers.
- DO NOT create any checklists that are embedded in the spec. That will be a separate command.

### Section Requirements

- **Mandatory sections**: Must be completed for every feature
- **Optional sections**: Include only when relevant to the feature
- When a section doesn't apply, remove it entirely (don't leave as "N/A")

### For AI Generation

When creating this spec from a user prompt:

1. **Make informed guesses**: Use context, industry standards, and common patterns to fill gaps
2. **Document assumptions**: Record reasonable defaults in the Assumptions section
3. **Limit clarifications**: Maximum 3 [NEEDS CLARIFICATION] markers - use only for critical decisions that:
   - Significantly impact feature scope or user experience
   - Have multiple reasonable interpretations with different implications
   - Lack any reasonable default
4. **Prioritize clarifications**: scope > security/privacy > user experience > technical details
5. **Think like a tester**: Every vague requirement should fail the "testable and unambiguous" checklist item
6. **Common areas needing clarification** (only if no reasonable default exists):
   - Feature scope and boundaries (include/exclude specific use cases)
   - User types and permissions (if multiple conflicting interpretations possible)
   - Security/compliance requirements (when legally/financially significant)

**Examples of reasonable defaults** (don't ask about these):

- Data retention: Industry-standard practices for the domain
- Performance targets: Standard web/mobile app expectations unless specified
- Error handling: User-friendly messages with appropriate fallbacks
- Authentication method: Standard session-based or OAuth2 for web apps
- Integration patterns: RESTful APIs unless specified otherwise

### Success Criteria Guidelines

Success criteria must be:

1. **Measurable**: Include specific metrics (time, percentage, count, rate)
2. **Technology-agnostic**: No mention of frameworks, languages, databases, or tools
3. **User-focused**: Describe outcomes from user/business perspective, not system internals
4. **Verifiable**: Can be tested/validated without knowing implementation details

**Good examples**:

- "Users can complete checkout in under 3 minutes"
- "System supports 10,000 concurrent users"
- "95% of searches return results in under 1 second"
- "Task completion rate improves by 40%"

**Bad examples** (implementation-focused):

- "API response time is under 200ms" (too technical, use "Users see results instantly")
- "Database can handle 1000 TPS" (implementation detail, use user-facing metric)
- "React components render efficiently" (framework-specific)
- "Redis cache hit rate above 80%" (technology-specific)

## Beads Error Handling

If beads commands fail during execution:

1. **bd init fails**: Display error, suggest `npm install --save-dev @beads/bd`, continue without beads
2. **Epic creation fails**: Log warning, continue with spec creation, note in completion report
3. **Epic lookup fails**: Create new epic (may result in duplicate if lookup was false negative)

The specification workflow should complete even if beads integration encounters errors. Beads is an enhancement for task tracking, not a blocker for spec creation.
