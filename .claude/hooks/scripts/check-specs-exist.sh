#!/usr/bin/env bash
# Pre-edit hook: reminds to write GWT specs before editing production Go files.
# Fires before Edit on .go files (not _test.go files).

set -euo pipefail

# Read the tool input from stdin
INPUT=$(cat)

# Extract the file path from the Edit tool input
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')

# Only check .go files that are not test files
if [[ "$FILE_PATH" != *.go ]] || [[ "$FILE_PATH" == *_test.go ]]; then
    exit 0
fi

# Skip acceptance pipeline files (generated/internal tooling)
if [[ "$FILE_PATH" == */acceptance/* ]]; then
    exit 0
fi

# Check if specs directory exists and has .txt files
if [[ ! -d "specs" ]] || [[ -z "$(find specs -name '*.txt' 2>/dev/null | head -1)" ]]; then
    echo "Consider writing GWT acceptance specs before implementation."
    echo "Run /spec-check to audit existing specs."
fi

exit 0
