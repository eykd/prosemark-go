#!/usr/bin/env bash
# Pre-tool-use hook: blocks dangerous git flags in Bash commands.
#
# Input: JSON on stdin with structure { "tool_input": { "command": "..." } }
# Exit codes:
#   0 = allow command
#   1 = hook error
#   2 = command blocked

set -euo pipefail

input=$(cat)

command=$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null) || {
  echo "Hook error: failed to parse JSON input" >&2
  exit 1
}

# Check for prohibited git flags
if printf '%s' "$command" | grep -qE 'git.*(--no-verify|--no-gpg-sign)|git\s+push.*(--force([^-]|$)|-f\s|--force-with-lease)'; then
  cat >&2 <<'MSG'
BLOCKED: Hook bypass or force flags detected.

Prohibited flags: --no-verify, --no-gpg-sign, --force, -f, --force-with-lease

Instead of bypassing safety checks:
- If pre-commit hook fails: Fix the linting/formatting/type errors it found
- If commit-msg fails: Write a proper conventional commit message
- If pre-push fails: Fix the issues preventing push
- If force push needed: This usually indicates a workflow problem

Fix the root problem rather than bypassing the safety mechanism.
Only use these flags when explicitly requested by the user.
MSG
  exit 2
fi

exit 0
