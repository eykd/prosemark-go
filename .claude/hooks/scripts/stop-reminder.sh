#!/usr/bin/env bash
# Stop hook: reminds to run both test streams if specs exist.

set -euo pipefail

# Check if specs directory exists and has .txt files
if [[ -d "specs" ]] && [[ -n "$(find specs -name '*.txt' 2>/dev/null | head -1)" ]]; then
    echo "Remember to run both test streams before pushing:"
    echo "  1. Unit tests:       just test"
    echo "  2. Acceptance tests: just acceptance"
fi

exit 0
