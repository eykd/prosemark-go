---
description: Capture a solved problem as a compound learning for future reference. Run after fixing bugs, resolving review findings, or discovering patterns.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Instructions

Use the `/compound` skill (`.claude/skills/compound/SKILL.md`) to capture a learning.

Follow the skill's workflow steps 1-7 in order.

If the user provides a problem description in their input, use it as the starting point for Step 1 instead of asking.

If no input is provided and no recent context is available (recent commits, closed tasks), ask:

> "What problem did you just solve? Describe the symptoms or error you encountered."
