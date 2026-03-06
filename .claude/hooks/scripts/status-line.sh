#!/usr/bin/env bash
set -euo pipefail

# Read JSON from stdin
input=$(cat)

# Extract fields using jq
pct=$(echo "$input" | jq -r '.context_window.used_percentage // 0')
total=$(echo "$input" | jq -r '.context_window.context_window_size // 0')
current_dir=$(echo "$input" | jq -r '.workspace.current_dir // ""')

# Folder name from path
folder=$(basename "$current_dir")

# Git branch
branch=$(git -C "$current_dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "???")

# Format tokens used: percentage of total, displayed as e.g. "129.8K"
tokens_used=$(awk "BEGIN { printf \"%.1fK\", ($total * $pct / 100) / 1000 }")

# ANSI colors
GREEN='\033[32m'
CYAN='\033[36m'
YELLOW='\033[33m'
RESET='\033[0m'

printf "${GREEN}%s${RESET}:${CYAN}%s${RESET} [${YELLOW}%s %s%%${RESET}]" \
  "$folder" "$branch" "$tokens_used" "$pct"
