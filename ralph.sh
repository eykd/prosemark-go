#!/usr/bin/env bash
# ralph.sh - Automated feature development loop using Claude CLI and beads
#
# Repeatedly invokes Claude with /sp:next until all tasks under the current
# feature epic are complete. Uses beads as source of truth for task state.

set -euo pipefail

# Script version
readonly VERSION="2.0.0"

# Default configuration
readonly DEFAULT_MAX_ITERATIONS=50
readonly LOCK_FILE=".ralph.lock"
readonly LOG_FILE=".ralph.log"
readonly LOG_MAX_SIZE=$((10 * 1024 * 1024))  # 10MB max log size

# Retry configuration
readonly MAX_RETRIES=10
readonly MAX_RETRY_DELAY=300  # 5 minutes cap

# Claude CLI timeout
readonly CLAUDE_TIMEOUT=1800  # 30 minutes max per invocation

# TDD cycle configuration
readonly TDD_STEP_RETRIES=3        # retries per step (semantic failures)
readonly TDD_STEP_RETRY_DELAY=10   # seconds between step retries
readonly TDD_MAX_CYCLES=5          # max R-G-R-Review cycles per task
readonly STEP_RED="RED"
readonly STEP_GREEN="GREEN"
readonly STEP_REFACTOR="REFACTOR"
readonly STEP_REVIEW="REVIEW"
readonly STEP_BIND="BIND"
readonly STEP_ACCEPTANCE="ACCEPTANCE"
readonly REVIEW_OUTPUT_FILE=".ralph-review.json"
readonly ACCEPTANCE_OUTPUT_FILE=".ralph-acceptance.json"
readonly ATDD_MAX_INNER_CYCLES=15
readonly BASELINE_FIX_MAX_ATTEMPTS=2  # max Claude attempts to fix failing baseline

# Exit codes
readonly EXIT_SUCCESS=0
readonly EXIT_FAILURE=1
readonly EXIT_LIMIT_REACHED=2
readonly EXIT_SIGINT=130

# Runtime configuration (set by argument parsing)
DRY_RUN=false
MAX_ITERATIONS="$DEFAULT_MAX_ITERATIONS"
EXPLICIT_EPIC_ID=""  # Epic ID provided via --epic argument

# Runtime state (used for signal handlers and summary)
CURRENT_ITERATION=0
START_TIME=0
CLAUDE_PID=""

##############################################################################
# Logging infrastructure
##############################################################################

# Rotate log file if it exceeds MAX_SIZE
rotate_log() {
    if [[ -f "$LOG_FILE" ]] && [[ $(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null || echo 0) -gt "$LOG_MAX_SIZE" ]]; then
        local timestamp
        timestamp=$(date +%Y%m%d-%H%M%S)
        mv "$LOG_FILE" "${LOG_FILE}.${timestamp}"
        echo "[ralph] Rotated log file to ${LOG_FILE}.${timestamp}"
    fi
}

# Initialize log file with session header
init_log() {
    rotate_log
    {
        echo "========================================================================"
        echo "Ralph Automation Session"
        echo "Started: $(date -Iseconds)"
        echo "Version: $VERSION"
        echo "Configuration: dry_run=$DRY_RUN, max_iterations=$MAX_ITERATIONS, explicit_epic=$EXPLICIT_EPIC_ID"
        echo "========================================================================"
        echo ""
    } >> "$LOG_FILE"
}

# Log a message with timestamp and level
# Usage: log LEVEL MESSAGE
# Levels: INFO, WARN, ERROR, DEBUG
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp
    timestamp=$(date -Iseconds)

    # Write to log file
    echo "[$timestamp] [$level] $message" >> "$LOG_FILE"

    # Also write to console for INFO/WARN/ERROR (always to stderr to avoid capture in command substitution)
    case "$level" in
        INFO)
            echo "[ralph] $message" >&2
            ;;
        WARN)
            echo "[ralph] WARNING: $message" >&2
            ;;
        ERROR)
            echo "[ralph] ERROR: $message" >&2
            ;;
    esac
}

# Log a separator for major sections
log_section() {
    local title="$1"
    {
        echo ""
        echo "========================================================================"
        echo "$title"
        echo "========================================================================"
        echo ""
    } >> "$LOG_FILE"
}

# Log the full content of a prompt or output
log_block() {
    local label="$1"
    local content="$2"
    {
        echo ""
        echo "-------- $label --------"
        echo "$content"
        echo "-------- End $label --------"
        echo ""
    } >> "$LOG_FILE"
}

##############################################################################
# Usage and help
##############################################################################

usage() {
    cat <<EOF
Usage: ralph.sh [OPTIONS]

Automated feature development loop using Claude CLI and beads.

Repeatedly invokes Claude with /sp:next until all tasks under the current
feature epic are complete. Uses beads as source of truth for task state.

OPTIONS:
    --dry-run           Show what would be executed without invoking Claude
    --epic <epic-id>    Explicitly specify epic ID (overrides branch-based detection)
    --max-iterations N  Maximum loop iterations (default: $DEFAULT_MAX_ITERATIONS)
    --help              Show this help message and exit
    --version           Show version and exit

EXIT CODES:
    0   All tasks completed successfully
    1   Error (prerequisites failed, Claude failures after retries, invalid epic)
    2   Maximum iterations reached
    130 Interrupted by SIGINT (Ctrl+C)

EXAMPLES:
    ralph.sh                    # Run with defaults (branch-based detection)
    ralph.sh --dry-run          # Preview what would happen
    ralph.sh --epic workspace-whatever13  # Use explicit epic ID
    ralph.sh --max-iterations 10  # Limit to 10 iterations
    ralph.sh --epic workspace-abc --dry-run  # Combine options

EOF
}

##############################################################################
# Argument parsing
##############################################################################

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --epic)
                if [[ -z "${2:-}" ]]; then
                    echo "Error: --epic requires an epic ID argument" >&2
                    exit "$EXIT_FAILURE"
                fi
                EXPLICIT_EPIC_ID="$2"
                shift 2
                ;;
            --max-iterations)
                if [[ -z "${2:-}" ]] || [[ ! "$2" =~ ^[0-9]+$ ]]; then
                    echo "Error: --max-iterations requires a positive integer" >&2
                    exit "$EXIT_FAILURE"
                fi
                MAX_ITERATIONS="$2"
                shift 2
                ;;
            --help)
                usage
                exit "$EXIT_SUCCESS"
                ;;
            --version)
                echo "ralph.sh version $VERSION"
                exit "$EXIT_SUCCESS"
                ;;
            -*)
                echo "Error: Unknown option: $1" >&2
                usage >&2
                exit "$EXIT_FAILURE"
                ;;
            *)
                echo "Error: Unexpected argument: $1" >&2
                usage >&2
                exit "$EXIT_FAILURE"
                ;;
        esac
    done
}

##############################################################################
# Prerequisite validation
##############################################################################

# Check if Claude CLI is available
check_claude_cli() {
    log DEBUG "Checking for Claude CLI..."
    if ! command -v claude &>/dev/null; then
        log ERROR "Claude CLI not found. Please install and authenticate claude CLI."
        return 1
    fi
    log DEBUG "Claude CLI found"
    return 0
}

# Check if beads is initialized
check_beads_init() {
    log DEBUG "Checking for beads initialization..."
    if [[ ! -d ".beads" ]]; then
        log ERROR "Beads not initialized. Run 'npx bd init' to initialize beads."
        return 1
    fi
    log DEBUG "Beads initialized"
    return 0
}

# Detect whether epic uses spec-kit workflow or generic task workflow
# Returns "spec-kit" if any [sp:NN-*] phase tasks exist, "generic" otherwise
detect_task_source() {
    local epic_id="$1"
    local all_tasks phase_tasks phase_count

    log DEBUG "Detecting task source mode for epic $epic_id..."

    # Query all tasks under the epic
    all_tasks=$(npx bd list --parent "$epic_id" --json 2>/dev/null) || {
        log DEBUG "Failed to query tasks, defaulting to generic mode"
        echo "generic"
        return 0
    }

    # Search for sp:NN- pattern in task titles
    phase_tasks=$(echo "$all_tasks" | jq '[.[] | select(.title | test("\\[sp:[0-9]{2}-"))]' 2>/dev/null) || {
        log DEBUG "Failed to parse tasks, defaulting to generic mode"
        echo "generic"
        return 0
    }

    phase_count=$(echo "$phase_tasks" | jq 'length' 2>/dev/null || echo "0")

    if [[ "$phase_count" -gt 0 ]]; then
        log INFO "Detected spec-kit workflow mode ($phase_count phase task(s) found)"
        echo "spec-kit"
    else
        log INFO "Detected generic task workflow mode (no sp:* phase tasks found)"
        echo "generic"
    fi

    return 0
}

# Check if clarify phase is complete for the epic
check_clarify_complete() {
    local epic_id="$1"
    local clarify_task status

    log DEBUG "Checking clarify phase completion for epic $epic_id..."

    # Find the clarify task for this epic (title contains [sp:02-clarify])
    # Must use --status closed since we're checking for completed phase tasks
    clarify_task=$(npx bd list --parent "$epic_id" --status closed --json 2>/dev/null | \
        jq -c 'first(.[] | select(.title | contains("[sp:02-clarify]"))) | {id, status}')

    if [[ -z "$clarify_task" ]]; then
        log DEBUG "No clarify task found for epic $epic_id"
        return 2
    fi

    status=$(echo "$clarify_task" | jq -r '.status')
    log DEBUG "Clarify task status: $status"

    if [[ "$status" != "closed" ]]; then
        log ERROR "Clarify phase not complete (status: $status)"
        echo "" >&2
        echo "Ralph automates phases 03-09 only. Before running ralph.sh:" >&2
        echo "  1. Run '/sp:01-specify' to create the feature specification" >&2
        echo "  2. Run '/sp:02-clarify' to clarify requirements" >&2
        echo "" >&2
        echo "Once clarify is complete, ralph.sh can automate the rest." >&2
        return 1
    fi

    log DEBUG "Clarify phase complete"
    return 0
}

# Check if task suite has been generated for the epic
check_tasks_generated() {
    local epic_id="$1"
    local tasks_task status

    log DEBUG "Checking task generation for epic $epic_id..."

    # Find the tasks phase task for this epic (title contains [sp:05-tasks])
    # Must use --status closed since we're checking for completed phase tasks
    tasks_task=$(npx bd list --parent "$epic_id" --status closed --json 2>/dev/null | \
        jq -c 'first(.[] | select(.title | contains("[sp:05-tasks]"))) | {id, status}')

    if [[ -z "$tasks_task" ]]; then
        log DEBUG "No tasks phase found for epic $epic_id"
        return 2
    fi

    status=$(echo "$tasks_task" | jq -r '.status')
    log DEBUG "Tasks phase status: $status"

    if [[ "$status" != "closed" ]]; then
        log ERROR "Task generation not complete (status: $status)"
        echo "" >&2
        echo "Run '/sp:05-tasks' to generate the task suite before running ralph." >&2
        return 1
    fi

    log DEBUG "Task generation complete"
    return 0
}

# Run all prerequisite checks
validate_prerequisites() {
    local epic_id="$1"
    local task_source clarify_result tasks_result all_tasks task_count

    log INFO "Validating prerequisites..."
    log_section "PREREQUISITE VALIDATION"

    # Always check infrastructure
    check_claude_cli || return 1
    check_beads_init || return 1

    # Detect task source mode
    task_source=$(detect_task_source "$epic_id")

    if [[ "$task_source" == "spec-kit" ]]; then
        log INFO "Validating spec-kit workflow prerequisites..."

        # Check clarify phase
        clarify_result=0
        check_clarify_complete "$epic_id" || clarify_result=$?

        if [[ "$clarify_result" -eq 1 ]]; then
            log ERROR "Clarify phase not complete"
            echo "" >&2
            echo "Ralph automates phases 03-09 only. Before running ralph.sh:" >&2
            echo "  1. Run '/sp:01-specify' to create the feature specification" >&2
            echo "  2. Run '/sp:02-clarify' to clarify requirements" >&2
            echo "" >&2
            echo "Once clarify is complete, ralph.sh can automate the rest." >&2
            return 1
        fi

        # Check tasks generation phase
        tasks_result=0
        check_tasks_generated "$epic_id" || tasks_result=$?

        if [[ "$tasks_result" -eq 1 ]]; then
            log ERROR "Task generation not complete"
            echo "" >&2
            echo "The task suite has not been generated yet. Run these phases first:" >&2
            echo "  1. '/sp:03-plan' to create the implementation plan" >&2
            echo "  2. '/sp:04-checklist' to generate the checklist" >&2
            echo "  3. '/sp:05-tasks' to generate beads tasks" >&2
            echo "" >&2
            echo "Once tasks are generated, ralph.sh can automate implementation." >&2
            return 1
        fi

        log INFO "Spec-kit workflow prerequisites satisfied"
    else
        log INFO "Validating generic task workflow prerequisites..."

        # For generic mode, just verify epic has at least one task
        all_tasks=$(npx bd list --parent "$epic_id" --json 2>/dev/null) || {
            log ERROR "Failed to query tasks for epic $epic_id"
            return 1
        }

        # Exclude the epic itself and event tasks
        task_count=$(echo "$all_tasks" | jq '[.[] | select(.id != "'"$epic_id"'" and .issue_type != "event")] | length' 2>/dev/null || echo "0")

        if [[ "$task_count" -eq 0 ]]; then
            log ERROR "The epic has no tasks to process"
            echo "" >&2
            echo "[ralph] ERROR: Epic $epic_id has no tasks." >&2
            echo "" >&2
            echo "Please create tasks under this epic before running ralph." >&2
            return 1
        fi

        log INFO "Generic task workflow prerequisites satisfied ($task_count task(s) found)"
    fi

    log INFO "All prerequisites satisfied"
    return 0
}

##############################################################################
# Lock file management
##############################################################################

# Check if a process with given PID is a running ralph.sh instance
# Validates both PID existence and process identity to handle PID reuse
is_ralph_running() {
    local pid="$1"
    local cmd

    # First check if the process exists
    if ! kill -0 "$pid" 2>/dev/null; then
        return 1
    fi

    # Verify it's actually a ralph.sh process (handles PID reuse)
    cmd=$(ps -p "$pid" -o comm= 2>/dev/null) || return 1
    [[ "$cmd" == "ralph.sh" || "$cmd" == "bash" ]]
}

# Acquire exclusive lock for this branch using atomic file creation
acquire_lock() {
    local branch="$1"
    local lock_content existing_pid existing_branch

    log DEBUG "Attempting to acquire lock for branch: $branch"

    # Prepare lock content: PID, timestamp, branch
    lock_content="$$
$(date -Iseconds)
$branch"

    # Try atomic lock creation first (prevents TOCTOU race condition)
    if ( set -o noclobber; echo "$lock_content" > "$LOCK_FILE" ) 2>/dev/null; then
        log INFO "Acquired lock (PID: $$)"
        return 0
    fi

    # Lock file exists - check if it's stale
    existing_pid=$(head -n1 "$LOCK_FILE" 2>/dev/null || echo "")
    existing_branch=$(sed -n '3p' "$LOCK_FILE" 2>/dev/null || echo "")

    log DEBUG "Lock file exists: PID=$existing_pid, branch=$existing_branch"

    if [[ -n "$existing_pid" ]] && is_ralph_running "$existing_pid"; then
        log ERROR "ralph.sh is already running on branch '$existing_branch' (PID: $existing_pid)"
        echo "If this is stale, remove $LOCK_FILE manually." >&2
        return 1
    fi

    # Stale lock file - remove and retry atomically
    log INFO "Removing stale lock file (PID $existing_pid not running)"
    rm -f "$LOCK_FILE"

    if ( set -o noclobber; echo "$lock_content" > "$LOCK_FILE" ) 2>/dev/null; then
        log INFO "Acquired lock (PID: $$)"
        return 0
    fi

    # Another process acquired the lock between rm and create
    log ERROR "Another ralph.sh instance acquired the lock"
    return 1
}

# Release the lock file
release_lock() {
    if [[ -f "$LOCK_FILE" ]]; then
        local lock_pid
        lock_pid=$(head -n1 "$LOCK_FILE" 2>/dev/null || echo "")

        # Only release if we own the lock
        if [[ "$lock_pid" == "$$" ]]; then
            rm -f "$LOCK_FILE"
            log INFO "Released lock"
        fi
    fi
}

##############################################################################
# Epic detection
##############################################################################

# Get the current git branch name
get_current_branch() {
    git branch --show-current 2>/dev/null || {
        echo "Error: Not in a git repository or HEAD is detached" >&2
        return 1
    }
}

# Extract feature name from branch (strip numeric prefix like "001-")
extract_feature_name() {
    local branch="$1"
    # Remove leading digits and hyphen (e.g., "001-ralph-automation" -> "ralph-automation")
    echo "$branch" | sed 's/^[0-9]*-//'
}

# Find the beads epic ID matching the feature name
find_epic_id() {
    local feature_name="$1"
    local epics_json

    epics_json=$(npx bd list --type feature --status open --json 2>/dev/null) || {
        echo "Error: Failed to query beads for epics" >&2
        return 1
    }

    # Find epic where title contains the feature name
    # Normalize hyphens to spaces so "linemark-mvp" matches "Linemark MVP"
    local epic_id
    epic_id=$(echo "$epics_json" | jq -r --arg name "$feature_name" \
        '.[] | select(.title | ascii_downcase | gsub("-"; " ") | contains($name | ascii_downcase | gsub("-"; " "))) | .id' | head -n1)

    if [[ -z "$epic_id" ]]; then
        echo "Error: No epic found matching feature '$feature_name'" >&2
        return 1
    fi

    echo "$epic_id"
}

# Validate that an epic exists and is open
# Arguments: epic_id
# Returns: 0 if valid and open, 1 if not found/closed
validate_epic_exists() {
    local epic_id="$1"
    local epic_data status

    log DEBUG "Validating epic: $epic_id"

    # Query beads for this epic ID
    epic_data=$(npx bd list --type feature --json 2>/dev/null | \
        jq -r --arg id "$epic_id" '.[] | select(.id == $id)') || {
        log ERROR "Failed to query beads for epics"
        return 1
    }

    if [[ -z "$epic_data" ]]; then
        log ERROR "Epic not found: $epic_id"
        echo "Error: Epic '$epic_id' not found in beads" >&2
        return 1
    fi

    # Check if epic is open
    status=$(echo "$epic_data" | jq -r '.status // "unknown"')

    if [[ "$status" != "open" ]]; then
        log ERROR "Epic is not open: $epic_id (status: $status)"
        echo "Error: Epic '$epic_id' is not open (status: $status)" >&2
        return 1
    fi

    log DEBUG "Epic validated successfully: $epic_id (status: $status)"
    return 0
}

# Detect epic from explicit argument or current branch
# If EXPLICIT_EPIC_ID is set, validate and return it
# Otherwise, fall back to branch-based detection
detect_epic() {
    local epic_id

    # If --epic argument was provided, use and validate it
    if [[ -n "$EXPLICIT_EPIC_ID" ]]; then
        log DEBUG "Using explicit epic ID: $EXPLICIT_EPIC_ID"

        if ! validate_epic_exists "$EXPLICIT_EPIC_ID"; then
            return 1
        fi

        echo "$EXPLICIT_EPIC_ID"
        return 0
    fi

    # Fall back to branch-based detection
    log DEBUG "Detecting epic from current branch..."

    local branch feature_name

    branch=$(get_current_branch) || return 1

    if [[ -z "$branch" ]]; then
        log ERROR "Could not determine current branch"
        return 1
    fi

    feature_name=$(extract_feature_name "$branch")
    log INFO "Branch: $branch, Feature: $feature_name"

    epic_id=$(find_epic_id "$feature_name") || return 1
    log INFO "Epic ID: $epic_id"

    echo "$epic_id"
}

##############################################################################
# Task checking
##############################################################################

# Get in-progress tasks for the epic (returns JSON array)
get_in_progress_tasks() {
    local epic_id="$1"
    local in_progress_json

    in_progress_json=$(npx bd list --status in-progress --parent "$epic_id" --json 2>/dev/null) || {
        echo "Error: Failed to query beads for in-progress tasks" >&2
        return 1
    }

    # Filter out the epic itself and event tasks
    # (bd list --parent already returns only child tasks)
    echo "$in_progress_json" | jq --arg epic "$epic_id" \
        '[.[] | select(.id != $epic and .issue_type != "event")]'
}

# Check if there are in-progress tasks
has_in_progress_tasks() {
    local epic_id="$1"
    local in_progress_tasks count

    in_progress_tasks=$(get_in_progress_tasks "$epic_id") || return 1
    count=$(echo "$in_progress_tasks" | jq 'length')

    [[ "$count" -gt 0 ]]
}

# Get the first in-progress task info
get_in_progress_task() {
    local epic_id="$1"
    local in_progress_tasks

    in_progress_tasks=$(get_in_progress_tasks "$epic_id") || return 1
    echo "$in_progress_tasks" | jq -r '.[0] // empty'
}

# Check if a task has children (is a parent container)
task_has_children() {
    local task_id="$1"
    local child_count
    child_count=$(npx bd list --parent "$task_id" --limit 1 --json 2>/dev/null | jq 'length' 2>/dev/null) || return 1
    [[ "$child_count" -gt 0 ]]
}

# Check if all direct children of a task are closed
all_children_closed() {
    local task_id="$1"
    local open_count
    open_count=$(npx bd list --parent "$task_id" --status open --json 2>/dev/null | jq 'length' 2>/dev/null) || return 1
    [[ "$open_count" -eq 0 ]]
}

# Auto-close parent container tasks when all their children are completed.
# Walks up from the given task ID, closing ancestors bottom-up.
auto_close_completed_parents() {
    local task_id="$1"
    local epic_id="$2"
    local current_id="$task_id"

    while true; do
        # Remove last ID segment to get parent
        local parent_id="${current_id%.*}"

        # Stop if we can't go higher or reached the epic
        [[ "$parent_id" == "$current_id" ]] && break
        [[ "$parent_id" == "$epic_id" ]] && break

        # Check if parent has children and all are closed
        if task_has_children "$parent_id" && all_children_closed "$parent_id"; then
            log INFO "Auto-closing completed parent task: $parent_id"
            npx bd close "$parent_id" 2>/dev/null || {
                log WARN "Failed to auto-close parent task: $parent_id"
                break
            }
        else
            break  # If this parent can't close, no ancestor can either
        fi

        current_id="$parent_id"
    done
}

# Get ready tasks for the epic (returns JSON array)
get_ready_tasks() {
    local epic_id="$1"
    local ready_json

    ready_json=$(npx bd ready --parent "$epic_id" --limit 1000 --json 2>/dev/null) || {
        echo "Error: Failed to query beads for ready tasks" >&2
        return 1
    }

    # Filter out the epic itself and event tasks
    local filtered
    filtered=$(echo "$ready_json" | jq --arg epic "$epic_id" \
        '[.[] | select(.id != $epic and .issue_type != "event")]')

    # Filter out parent container tasks (tasks that have children).
    # Parent tasks are not work items; ralph processes their children individually.
    local leaf_ids=()
    local task_id
    while IFS= read -r task_id; do
        [[ -z "$task_id" ]] && continue
        if task_has_children "$task_id"; then
            log DEBUG "Skipping parent container task: $task_id"
        else
            leaf_ids+=("$task_id")
        fi
    done < <(echo "$filtered" | jq -r '.[].id')

    if [[ ${#leaf_ids[@]} -eq 0 ]]; then
        echo "[]"
        return 0
    fi

    # Rebuild JSON array with only leaf tasks
    local id_array
    id_array=$(printf '"%s",' "${leaf_ids[@]}")
    id_array="[${id_array%,}]"

    echo "$filtered" | jq --argjson ids "$id_array" \
        '[.[] | select(.id | IN($ids[]))]'
}

# Check if there are ready tasks remaining
has_ready_tasks() {
    local epic_id="$1"
    local ready_tasks count

    ready_tasks=$(get_ready_tasks "$epic_id") || return 1
    count=$(echo "$ready_tasks" | jq 'length')

    [[ "$count" -gt 0 ]]
}

# Get the first ready task info
get_next_task() {
    local epic_id="$1"
    local ready_tasks

    ready_tasks=$(get_ready_tasks "$epic_id") || return 1
    echo "$ready_tasks" | jq -r '.[0] // empty'
}

# Get all open tasks for the epic (returns JSON array)
get_open_tasks() {
    local epic_id="$1"
    local open_json

    open_json=$(npx bd list --status open --parent "$epic_id" --json 2>/dev/null) || {
        echo "Error: Failed to query beads for open tasks" >&2
        return 1
    }

    # Filter out the epic itself and event tasks
    # (bd list --parent already returns only child tasks)
    echo "$open_json" | jq --arg epic "$epic_id" \
        '[.[] | select(.id != $epic and .issue_type != "event")]'
}

# Check if there are any open tasks remaining
has_open_tasks() {
    local epic_id="$1"
    local open_tasks count

    open_tasks=$(get_open_tasks "$epic_id") || return 1
    count=$(echo "$open_tasks" | jq 'length')

    [[ "$count" -gt 0 ]]
}

##############################################################################
# Task type detection and prompt generation
##############################################################################

# Generate focused prompt based on task type
generate_focused_prompt() {
    local task_json="$1"
    local task_title task_id task_description task_details comments_json comments_text task_status

    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')
    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_description=$(echo "$task_json" | jq -r '.description // ""')
    task_status=$(echo "$task_json" | jq -r '.status // "unknown"')

    # Fetch full task details including comments
    task_details=$(npx bd show "$task_id" --json 2>/dev/null) || task_details=""

    # Extract and format comments if they exist
    if [[ -n "$task_details" ]]; then
        # Normalize to object (handle both array and object responses from bd show)
        comments_json=$(echo "$task_details" | jq -r '(if type == "array" then .[0] else . end) | .comments // []')
        comments_text=$(echo "$comments_json" | jq -r '.[] | "- \(.timestamp // "unknown"): \(.text // "")"' 2>/dev/null)
    else
        comments_text=""
    fi

    # Start with base prompt (no /sp:next — ralph already resolved the task)
    cat <<EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.
Communicate status ONLY through beads task management.

## Task Details
Title: $task_title
ID: $task_id
Status: $task_status
EOF

    # Add resumption notice if task is in-progress
    if [[ "$task_status" == "in-progress" ]]; then
        cat <<EOF

**RESUMING INTERRUPTED TASK**
This task was previously started but not completed.
Review previous comments below for context on what was done.
Continue from where you left off.
EOF
    fi

    cat <<EOF

Description:
$task_description
EOF

    # Add comments section if there are any
    if [[ -n "$comments_text" ]]; then
        cat <<EOF

Previous Comments:
$comments_text
EOF
    fi

    cat <<EOF

## Task Focus
Complete ONLY this task described above.
Do NOT explore unrelated code or work on other tasks.

## Bead Lifecycle Management (REQUIRED)
1. Start task: npx bd start $task_id
2. Track progress: npx bd comment $task_id "status update message"
3. Complete task: npx bd close $task_id
4. If blocked: npx bd comment $task_id "BLOCKED: reason" (do NOT close)

CRITICAL: You MUST close the bead when the task is complete.
If you do not close it, ralph will run this task again.
EOF

    # Add testing instructions for Go TDD
    cat <<EOF

## TDD Practice (Go)
Apply strict red-green-refactor:
1. RED: Write failing test first
2. GREEN: Minimal code to pass
3. REFACTOR: Improve while green

Run tests:
- All tests: just test
- With coverage: just test-cover
- Coverage check: just test-cover-check
- Single test: go test -run TestName ./path/to/package

100% coverage is required for non-Impl functions.
Use table-driven tests where appropriate.
EOF

    # Always add commit instructions
    cat <<EOF

## After Task Completion - COMMITTING IS MANDATORY

YOU MUST COMMIT YOUR WORK. This is NON-NEGOTIABLE.

### Why Committing Is Critical

Without a successful commit:
- Your work will be LOST if the next task modifies the same files
- Ralph will repeat this task thinking it wasn't completed
- The .beads state won't be saved, causing task tracking failures
- Pre-commit hooks won't validate your changes

### Exact Commit Sequence (REQUIRED)

1. **Close bead FIRST**: \`npx bd close $task_id\`
   - This updates .beads state which MUST be included in the commit
   - Marks task as complete in beads tracking

2. **Commit ALL changes**: Run \`/commit\` skill
   - Stages ALL modified files (.beads state + your code changes)
   - Creates conventional commit message
   - Runs pre-commit hooks (gofmt, go vet, staticcheck, test-coverage)
   - **PRE-COMMIT HOOKS MUST PASS - NO EXCEPTIONS**

3. **If pre-commit hooks FAIL**:
   - READ the error message carefully
   - FIX the issues (formatting, linting, type errors, test failures)
   - Run \`/commit\` again
   - REPEAT until commit succeeds

4. **Verify commit succeeded**:
   - You should see "committed successfully" message
   - If not, the task is NOT complete - keep fixing and retrying

### Critical Rules

✅ REQUIRED:
- Create a commit for EVERY completed task
- Fix ALL pre-commit hook failures
- Include .beads state changes in commit
- Verify commit succeeded before moving on

❌ FORBIDDEN:
- Skipping commit after completing task
- Using --no-verify, --no-hooks, or similar flags
- Leaving task closed but changes uncommitted
- Moving to next iteration without successful commit

### Success Criteria

The task is ONLY complete when:
1. ✓ Bead is closed (\`npx bd close $task_id\`)
2. ✓ All changes are committed (including .beads/)
3. ✓ Pre-commit hooks passed (gofmt, go vet, staticcheck, 100% coverage)
4. ✓ Commit succeeded (you saw success message)

If ANY of these are false, the task is INCOMPLETE - keep working until all pass.

Ralph will create a series of commits across iterations.
User will push all commits manually when the feature is ready.
EOF
}

##############################################################################
# TDD helper functions
##############################################################################

# Load skill content from .claude/skills/<name>/SKILL.md
# Returns content on stdout. Falls back gracefully if file missing.
load_skill_content() {
    local skill_name="$1"
    local skill_path=".claude/skills/${skill_name}/SKILL.md"

    if [[ -f "$skill_path" ]]; then
        cat "$skill_path"
    else
        log WARN "Skill file not found: $skill_path"
        echo "(Skill $skill_name not available)"
    fi
}

# Check that tests pass before starting RED step.
# If tests are already failing, mark task BLOCKED and skip.
# Allows "no test files" as valid baseline (go test exits 0 with "no test files").
check_baseline_tests() {
    local test_output exit_code

    log INFO "Checking baseline tests before RED step..."

    test_output=$(go test ./... 2>&1) && exit_code=0 || exit_code=$?

    if [[ "$exit_code" -ne 0 ]]; then
        log ERROR "Baseline tests are failing - cannot start RED step"
        log_block "Baseline Test Output" "$test_output"
        return 1
    fi

    log INFO "Baseline tests pass (or no test files) - ready for RED"
    return 0
}

# Attempt to auto-fix failing baseline tests by invoking Claude.
# Loops up to BASELINE_FIX_MAX_ATTEMPTS times.
# Returns 0 if tests are fixed, 1 if still failing.
attempt_baseline_fix() {
    local attempt=0
    local test_output exit_code prompt

    log INFO "Attempting baseline auto-fix (max $BASELINE_FIX_MAX_ATTEMPTS attempts)"

    while (( attempt < BASELINE_FIX_MAX_ATTEMPTS )); do
        attempt=$((attempt + 1))
        log INFO "Baseline fix attempt $attempt/$BASELINE_FIX_MAX_ATTEMPTS"

        # Capture current failing output
        test_output=$(go test ./... 2>&1) && exit_code=0 || exit_code=$?

        if [[ "$exit_code" -eq 0 ]]; then
            log INFO "Baseline tests now pass (fixed before invoking Claude)"
            return 0
        fi

        # Generate fix prompt and invoke Claude
        prompt=$(generate_baseline_fix_prompt "$test_output")
        if ! invoke_claude_with_retry "$prompt"; then
            log WARN "Baseline fix attempt $attempt: Claude invocation failed"
            continue
        fi

        # Re-check tests after Claude's fix
        test_output=$(go test ./... 2>&1) && exit_code=0 || exit_code=$?

        if [[ "$exit_code" -eq 0 ]]; then
            log INFO "Baseline tests fixed on attempt $attempt"
            return 0
        fi

        log WARN "Baseline fix attempt $attempt: tests still failing"
        log_block "Post-Fix Test Output (attempt $attempt)" "$test_output"
    done

    log ERROR "Baseline auto-fix exhausted $BASELINE_FIX_MAX_ATTEMPTS attempts"
    return 1
}

# Ralph's spot-check gate between TDD steps.
# RED: expects test failure (exit != 0)
# GREEN/REFACTOR: expects test success (exit == 0)
run_verification() {
    local step="$1"
    local test_output exit_code

    log INFO "Running verification for $step step..."

    test_output=$(go test ./... 2>&1) && exit_code=0 || exit_code=$?

    case "$step" in
        "$STEP_RED")
            if [[ "$exit_code" -ne 0 ]]; then
                log INFO "RED verification passed: tests fail as expected"
                return 0
            else
                log ERROR "RED verification failed: tests should fail but pass"
                log_block "Test Output ($step)" "$test_output"
                return 1
            fi
            ;;
        "$STEP_GREEN"|"$STEP_REFACTOR")
            if [[ "$exit_code" -eq 0 ]]; then
                log INFO "$step verification passed: tests pass"
                return 0
            else
                log ERROR "$step verification failed: tests should pass but fail"
                log_block "Test Output ($step)" "$test_output"
                return 1
            fi
            ;;
        *)
            log ERROR "Unknown step for verification: $step"
            return 1
            ;;
    esac
}

# Detect if REFACTOR made changes. No changes -> skip verification and commit.
check_for_changes() {
    if git diff --quiet HEAD 2>/dev/null; then
        log INFO "No changes detected after REFACTOR"
        return 1  # no changes
    else
        log INFO "Changes detected after REFACTOR"
        return 0  # has changes
    fi
}

##############################################################################
# TDD prompt generators
##############################################################################

# Generate prompt to fix failing baseline tests.
# Arguments: test_output (raw failing test output)
generate_baseline_fix_prompt() {
    local test_output="$1"
    local go_tdd_skill commit_skill

    go_tdd_skill=$(load_skill_content "go-tdd")
    commit_skill=$(load_skill_content "commit")

    cat <<PROMPT_EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.

## Mission: Fix Failing Tests

The test suite is currently failing. Your ONLY job is to diagnose and fix
whatever is causing the failures. Do NOT add new features or make unrelated
changes — fix only what is needed to make \`go test ./...\` pass.

### Failing Test Output
\`\`\`
$test_output
\`\`\`

### Rules
1. Diagnose the root cause from the test output above
2. Fix ONLY what is needed to make tests pass
3. Do NOT add new features or refactor unrelated code
4. Do NOT delete or skip tests — fix the code or fix the test expectations
5. Run \`go test ./...\` to verify your fix before committing
6. Commit the fix with a message like: "fix: repair failing baseline tests"
7. If the fix involves stale imports, renamed functions, or unbound acceptance
   test stubs, those are the most common causes — check for those first

### Reference: Go TDD Skill
<go-tdd-skill>
$go_tdd_skill
</go-tdd-skill>

### Commit Reference
$commit_skill
PROMPT_EOF
}

# Generate RED step prompt: write failing test ONLY
# Arguments: task_json, cycle_number, [remaining_items], [retry_context]
generate_red_prompt() {
    local task_json="$1"
    local cycle="$2"
    local remaining_items="${3:-}"
    local retry_context="${4:-}"
    local task_title task_id task_description
    local go_tdd_skill

    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')
    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_description=$(echo "$task_json" | jq -r '.description // ""')

    go_tdd_skill=$(load_skill_content "go-tdd")

    cat <<PROMPT_EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.
Communicate status ONLY through beads task management.

## TDD Step: RED (Cycle $cycle of $TDD_MAX_CYCLES)

### Task Details
Title: $task_title
ID: $task_id

Description:
$task_description

### Your Mission: Write FAILING Tests ONLY

You MUST write tests that FAIL. Do NOT write any implementation code.

**Rules:**
1. Write test(s) that cover the task requirements
2. Tests MUST fail when run (compile errors count as failing)
3. Do NOT write any production/implementation code
4. Do NOT modify existing passing tests
5. Use table-driven tests where appropriate
6. Follow Go testing conventions

PROMPT_EOF

    # bd start only on first cycle
    if [[ "$cycle" -eq 1 ]]; then
        cat <<PROMPT_EOF

### Bead Management
1. Start task: \`npx bd start $task_id\`
2. Comment progress: \`npx bd comment $task_id "RED: wrote failing tests for ..."\`
PROMPT_EOF
    else
        cat <<PROMPT_EOF

### Bead Management
- Comment progress: \`npx bd comment $task_id "RED cycle $cycle: wrote failing tests for ..."\`
- Do NOT start or close the bead (already started)
PROMPT_EOF
    fi

    # Feed remaining items from REVIEW if this is cycle > 1
    if [[ -n "$remaining_items" ]]; then
        cat <<PROMPT_EOF

### Focus Areas (from previous REVIEW)
The following items were identified as incomplete or missing:
$remaining_items

Write tests targeting these specific gaps.
PROMPT_EOF
    fi

    # Add retry context if retrying after failed verification
    if [[ -n "$retry_context" ]]; then
        cat <<PROMPT_EOF

### Retry Context (Previous Attempt Failed Verification)
The previous RED attempt failed verification. The tests were expected to fail
but they passed instead. This means you need to write tests for behavior that
is NOT yet implemented.

Previous test output:
$retry_context
PROMPT_EOF
    fi

    cat <<PROMPT_EOF

### Do NOT Commit
Ralph handles commits. Do NOT run git commit.

### Go TDD Reference
$go_tdd_skill
PROMPT_EOF
}

# Generate GREEN step prompt: write minimum code to pass
# Arguments: task_json, cycle_number, [retry_context]
generate_green_prompt() {
    local task_json="$1"
    local cycle="$2"
    local retry_context="${3:-}"
    local task_title task_id task_description
    local go_tdd_skill prefactoring_skill go_cli_ddd_skill commit_skill

    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')
    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_description=$(echo "$task_json" | jq -r '.description // ""')

    go_tdd_skill=$(load_skill_content "go-tdd")
    prefactoring_skill=$(load_skill_content "prefactoring")
    go_cli_ddd_skill=$(load_skill_content "go-cli-ddd")
    commit_skill=$(load_skill_content "commit")

    cat <<PROMPT_EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.
Communicate status ONLY through beads task management.

## TDD Step: GREEN (Cycle $cycle of $TDD_MAX_CYCLES)

### Task Details
Title: $task_title
ID: $task_id

Description:
$task_description

### Your Mission: Write MINIMUM Code to Pass Tests

Make all failing tests pass with the simplest possible implementation.

**Rules:**
1. Write ONLY enough production code to make tests pass
2. Do NOT add features beyond what tests require
3. Do NOT refactor (that's the next step)
4. Do NOT modify test files
5. All tests must pass: \`go test ./...\`
6. Handle all errors explicitly (no \`_\` for errors)
7. Follow Go conventions and project structure

### Bead Management
- Comment progress: \`npx bd comment $task_id "GREEN: implemented ..."\`
- Do NOT close the bead (REVIEW step decides completion)

### Commit Your Work (REQUIRED)
After making tests pass, you MUST commit:
1. Stage all changed files (specific files, never \`git add -A\`)
2. Create a conventional commit
3. Pre-commit hooks MUST pass (gofmt, go vet, staticcheck, coverage)
4. If hooks fail, fix issues and retry until commit succeeds
PROMPT_EOF

    # Add retry context if retrying after failed verification
    if [[ -n "$retry_context" ]]; then
        cat <<PROMPT_EOF

### Retry Context (Previous Attempt Failed Verification)
The previous GREEN attempt failed verification. Tests are still failing.
Fix the implementation to make ALL tests pass.

Previous test output:
$retry_context
PROMPT_EOF
    fi

    cat <<PROMPT_EOF

### Go TDD Reference
$go_tdd_skill

### Prefactoring Reference
$prefactoring_skill

### Go CLI DDD Reference
$go_cli_ddd_skill

### Commit Reference
$commit_skill
PROMPT_EOF
}

# Generate REFACTOR step prompt: improve code without changing behavior
# Arguments: task_json, cycle_number, [retry_context]
generate_refactor_prompt() {
    local task_json="$1"
    local cycle="$2"
    local retry_context="${3:-}"
    local task_title task_id task_description
    local go_tdd_skill refactoring_skill

    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')
    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_description=$(echo "$task_json" | jq -r '.description // ""')

    go_tdd_skill=$(load_skill_content "go-tdd")
    refactoring_skill=$(load_skill_content "refactoring")

    cat <<PROMPT_EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.
Communicate status ONLY through beads task management.

## TDD Step: REFACTOR (Cycle $cycle of $TDD_MAX_CYCLES)

### Task Details
Title: $task_title
ID: $task_id

Description:
$task_description

### Your Mission: Improve Code Quality Without Changing Behavior

All tests are passing. Improve the code while keeping them green.

**Rules:**
1. Do NOT change behavior (tests must stay green)
2. Improve naming, structure, readability
3. Eliminate duplication
4. Apply SOLID principles where appropriate
5. Run tests after each change: \`go test ./...\`
6. If no improvements needed, do nothing (that's fine)

### Bead Management
- Comment progress: \`npx bd comment $task_id "REFACTOR: improved ..."\`
- Do NOT close the bead

### Commit If Changes Made
If you made changes:
1. Stage all changed files (specific files, never \`git add -A\`)
2. Create a conventional commit (type: refactor)
3. Pre-commit hooks MUST pass
4. If hooks fail, fix issues and retry

If no changes needed, skip commit entirely.
PROMPT_EOF

    # Add retry context if retrying after failed verification
    if [[ -n "$retry_context" ]]; then
        cat <<PROMPT_EOF

### Retry Context (Previous Attempt Failed Verification)
The previous REFACTOR attempt broke tests. The changes were reverted.
Be more careful this time - make smaller changes and test after each one.

Previous test output:
$retry_context
PROMPT_EOF
    fi

    cat <<PROMPT_EOF

### Go TDD Reference
$go_tdd_skill

### Refactoring Reference
$refactoring_skill
PROMPT_EOF
}

# Generate REVIEW step prompt: evaluate completeness and test quality
# Arguments: task_json, cycle_number
generate_review_prompt() {
    local task_json="$1"
    local cycle="$2"
    local task_title task_id task_description

    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')
    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_description=$(echo "$task_json" | jq -r '.description // ""')

    cat <<PROMPT_EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.

## TDD Step: REVIEW (Cycle $cycle of $TDD_MAX_CYCLES)

### Task Details
Title: $task_title
ID: $task_id

Description:
$task_description

### Your Mission: Evaluate Completeness and Test Quality

Review the current implementation and tests against the task requirements.

**Evaluate:**
1. **Functional completeness**: Does the implementation satisfy ALL requirements in the task description?
2. **Test quality**: Are there meaningful assertions? Edge cases? Error paths?
3. **Coverage**: Are all non-Impl functions tested? (100% coverage required)

### Output: Write JSON to $REVIEW_OUTPUT_FILE

You MUST write a JSON file to \`$REVIEW_OUTPUT_FILE\` with this exact schema:

\`\`\`json
{
  "complete": true,
  "reason": "Brief explanation of verdict",
  "remaining_items": [],
  "test_gaps": []
}
\`\`\`

**Fields:**
- \`complete\`: \`true\` if task is fully implemented and well-tested, \`false\` otherwise
- \`reason\`: One-sentence explanation of your verdict
- \`remaining_items\`: Array of strings describing unimplemented requirements (empty if complete)
- \`test_gaps\`: Array of strings describing missing test coverage (empty if complete)

**Examples:**

Complete:
\`\`\`json
{
  "complete": true,
  "reason": "All requirements implemented with comprehensive test coverage",
  "remaining_items": [],
  "test_gaps": []
}
\`\`\`

Incomplete:
\`\`\`json
{
  "complete": false,
  "reason": "Missing error handling for invalid input",
  "remaining_items": ["validate empty string input", "handle file not found error"],
  "test_gaps": ["no test for empty input", "no test for concurrent access"]
}
\`\`\`

### Critical Rules
1. Write the JSON file using: echo '...' > $REVIEW_OUTPUT_FILE
2. The file MUST be valid JSON (parseable by jq)
3. Do NOT commit anything
4. Do NOT close or modify the bead
5. Be honest - if something is missing, say so
6. Consider: would you be confident shipping this?
PROMPT_EOF
}

# Generate BIND step prompt: write acceptance test implementations from spec
# Arguments: task_json, cycle_number, spec_file, [retry_context]
generate_bind_prompt() {
    local task_json="$1"
    local cycle="$2"
    local spec_file="$3"
    local retry_context="${4:-}"
    local task_title task_id task_description
    local acceptance_skill

    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')
    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_description=$(echo "$task_json" | jq -r '.description // ""')

    acceptance_skill=$(load_skill_content "acceptance-tests")

    cat <<PROMPT_EOF
## Non-Interactive Mode
You are running in ralph's automation loop (non-interactive).
You CANNOT ask questions or wait for user input.
Communicate status ONLY through beads task management.

## TDD Step: BIND (Cycle $cycle)

### Task Details
Title: $task_title
ID: $task_id

Description:
$task_description

### Your Mission: Write Acceptance Test Implementations

Read the spec file and the generated acceptance test stubs. For each stub
containing \`t.Fatal("acceptance test not yet bound")\`, replace the sentinel
with a real test implementation.

**Spec file:** \`$spec_file\`
**Generated tests:** \`generated-acceptance-tests/\`

### Binding Pattern

For each test function:
1. Each GIVEN step → file system setup using \`t.TempDir()\`
2. Each WHEN step → CLI invocation via \`exec.Command("go", "run", ".", subcommand, args...)\`
3. Each THEN step → output assertion using \`strings.Contains\` or similar

Example:
\`\`\`go
func Test_Some_scenario(t *testing.T) {
    dir := t.TempDir()
    // GIVEN: set up files
    // ...

    cmd := exec.Command("go", "run", ".", "subcommand")
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("command failed: %v\n%s", err, output)
    }

    // THEN: assert on output
    if !strings.Contains(string(output), "expected") {
        t.Errorf("expected output, got: %s", output)
    }
}
\`\`\`

### Rules
1. Edit the generated test files directly in \`generated-acceptance-tests/\`
2. Replace ONLY \`t.Fatal("acceptance test not yet bound")\` with real code
3. Add necessary imports (\`"os"\`, \`"os/exec"\`, \`"path/filepath"\`, \`"strings"\`)
4. The pipeline preserves bound implementations across regeneration
5. Do NOT modify the spec file
6. These tests are expected to FAIL until the feature is built — that's OK

### Bead Management
- Comment progress: \`npx bd comment $task_id "BIND: wrote acceptance test implementations for ..."\`
- Do NOT close the bead (acceptance check validates later)

### Do NOT Commit
Ralph handles commits after BIND. Do NOT run git commit.
PROMPT_EOF

    if [[ -n "$retry_context" ]]; then
        cat <<PROMPT_EOF

### Retry Context (Previous Attempt Had Issues)
$retry_context
PROMPT_EOF
    fi

    cat <<PROMPT_EOF

### Acceptance Tests Reference
$acceptance_skill
PROMPT_EOF
}

##############################################################################
# TDD cycle orchestration
##############################################################################

# Execute one TDD step with up to TDD_STEP_RETRIES semantic retries.
# Arguments: step_name, task_json, cycle_number, [remaining_items_for_red], [spec_file]
# Returns 0 on success, 1 on failure after all retries exhausted.
execute_tdd_step() {
    local step="$1"
    local task_json="$2"
    local cycle="$3"
    local remaining_items="${4:-}"
    local spec_file="${5:-}"
    local attempt=0
    local prompt retry_context=""
    local test_output

    log INFO "Executing TDD step: $step (cycle $cycle)"

    while (( attempt < TDD_STEP_RETRIES )); do
        attempt=$((attempt + 1))
        log DEBUG "$step attempt $attempt/$TDD_STEP_RETRIES"

        # Generate prompt based on step
        case "$step" in
            "$STEP_RED")
                prompt=$(generate_red_prompt "$task_json" "$cycle" "$remaining_items" "$retry_context")
                ;;
            "$STEP_GREEN")
                prompt=$(generate_green_prompt "$task_json" "$cycle" "$retry_context")
                ;;
            "$STEP_REFACTOR")
                prompt=$(generate_refactor_prompt "$task_json" "$cycle" "$retry_context")
                ;;
            "$STEP_REVIEW")
                prompt=$(generate_review_prompt "$task_json" "$cycle")
                ;;
            "$STEP_BIND")
                prompt=$(generate_bind_prompt "$task_json" "$cycle" "$spec_file" "$retry_context")
                ;;
            *)
                log ERROR "Unknown TDD step: $step"
                return 1
                ;;
        esac

        if [[ "$DRY_RUN" == "true" ]]; then
            log INFO "DRY RUN: Would invoke Claude for $step step"
            log_block "Dry Run $step Prompt" "$prompt"
            echo "--- $step PROMPT (cycle $cycle) ---"
            echo "$prompt"
            echo "--- END $step PROMPT ---"
            return 0
        fi

        # Invoke Claude (handles transient API failures internally)
        if ! invoke_claude_with_retry "$prompt"; then
            log ERROR "$step: Claude invocation failed after API retries"
            return 1
        fi

        # REVIEW step: no test verification, uses JSON output instead
        if [[ "$step" == "$STEP_REVIEW" ]]; then
            log INFO "REVIEW step complete (verification via JSON output)"
            return 0
        fi

        # BIND step: no test verification (tests are expected to fail until feature is built)
        if [[ "$step" == "$STEP_BIND" ]]; then
            log INFO "BIND step complete (acceptance check validates bindings)"
            return 0
        fi

        # REFACTOR: check if changes were made
        if [[ "$step" == "$STEP_REFACTOR" ]]; then
            if ! check_for_changes; then
                log INFO "REFACTOR made no changes - skipping verification"
                return 0
            fi
        fi

        # Run verification (spot-check)
        if run_verification "$step"; then
            if (( attempt > 1 )); then
                log INFO "$step succeeded after $attempt attempts"
            fi
            return 0
        fi

        # Verification failed - capture test output for retry context
        if (( attempt < TDD_STEP_RETRIES )); then
            test_output=$(go test ./... 2>&1) || true
            retry_context="$test_output"

            # REFACTOR failure: revert changes to preserve GREEN commit
            if [[ "$step" == "$STEP_REFACTOR" ]]; then
                log WARN "REFACTOR broke tests - reverting changes"
                git checkout . 2>/dev/null || true
            fi

            log WARN "$step verification failed (attempt $attempt/$TDD_STEP_RETRIES), retrying in ${TDD_STEP_RETRY_DELAY}s..."
            sleep "$TDD_STEP_RETRY_DELAY"
        fi
    done

    log ERROR "$step failed after $TDD_STEP_RETRIES attempts"

    # Final REFACTOR failure: revert to preserve GREEN state
    if [[ "$step" == "$STEP_REFACTOR" ]]; then
        log WARN "REFACTOR exhausted retries - reverting to preserve GREEN state"
        git checkout . 2>/dev/null || true
        # Return success since GREEN state is preserved
        return 0
    fi

    return 1
}

# Parse .ralph-review.json and return results
# Sets global variables: REVIEW_COMPLETE, REVIEW_REASON, REVIEW_REMAINING
parse_review_result() {
    local review_file="$REVIEW_OUTPUT_FILE"

    # Reset globals
    REVIEW_COMPLETE="false"
    REVIEW_REASON=""
    REVIEW_REMAINING=""

    if [[ ! -f "$review_file" ]]; then
        log WARN "Review output file not found: $review_file"
        return 1
    fi

    # Validate JSON structure
    if ! jq empty "$review_file" 2>/dev/null; then
        log WARN "Review output is not valid JSON"
        log_block "Invalid Review JSON" "$(cat "$review_file")"
        return 1
    fi

    # Extract fields
    REVIEW_COMPLETE=$(jq -r '.complete // false' "$review_file")
    REVIEW_REASON=$(jq -r '.reason // "no reason provided"' "$review_file")

    # Combine remaining_items and test_gaps into a single list for next RED
    local remaining_items test_gaps
    remaining_items=$(jq -r '.remaining_items // [] | .[]' "$review_file" 2>/dev/null)
    test_gaps=$(jq -r '.test_gaps // [] | .[]' "$review_file" 2>/dev/null)

    REVIEW_REMAINING=""
    if [[ -n "$remaining_items" ]]; then
        REVIEW_REMAINING="Remaining items:
$(echo "$remaining_items" | sed 's/^/- /')"
    fi
    if [[ -n "$test_gaps" ]]; then
        if [[ -n "$REVIEW_REMAINING" ]]; then
            REVIEW_REMAINING="$REVIEW_REMAINING

"
        fi
        REVIEW_REMAINING="${REVIEW_REMAINING}Test gaps:
$(echo "$test_gaps" | sed 's/^/- /')"
    fi

    log_block "Review Result" "Complete: $REVIEW_COMPLETE
Reason: $REVIEW_REASON
Remaining: $REVIEW_REMAINING"

    return 0
}

# Execute full TDD cycle (R-G-R-Review) for one bead task.
# Arguments: task_json, epic_id
# Returns 0 on success (task complete), 1 on failure (BLOCKED)
execute_unit_tdd_cycle() {
    local task_json="$1"
    local epic_id="$2"
    local task_id task_title
    local cycle=0
    local remaining_items=""
    local head_before head_after

    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')

    log_section "TDD CYCLE for $task_id: $task_title"

    while (( cycle < TDD_MAX_CYCLES )); do
        cycle=$((cycle + 1))
        log INFO "=== TDD Cycle $cycle/$TDD_MAX_CYCLES for $task_id ==="

        # DRY RUN: generate all four prompts and return
        if [[ "$DRY_RUN" == "true" ]]; then
            execute_tdd_step "$STEP_RED" "$task_json" "$cycle" "$remaining_items"
            execute_tdd_step "$STEP_GREEN" "$task_json" "$cycle"
            execute_tdd_step "$STEP_REFACTOR" "$task_json" "$cycle"
            execute_tdd_step "$STEP_REVIEW" "$task_json" "$cycle"
            return 0
        fi

        # Check baseline tests before RED
        if ! check_baseline_tests; then
            log WARN "Baseline tests failing - attempting auto-fix"
            if ! attempt_baseline_fix; then
                log ERROR "Baseline auto-fix failed - marking task BLOCKED"
                npx bd comment "$task_id" "BLOCKED: baseline tests failing (auto-fix failed after $BASELINE_FIX_MAX_ATTEMPTS attempts)" 2>/dev/null || true
                return 1
            fi
            log INFO "Baseline tests fixed - continuing to RED step"
            npx bd comment "$task_id" "Baseline tests were failing but auto-fixed" 2>/dev/null || true
        fi

        # --- RED: Write failing tests ---
        if ! execute_tdd_step "$STEP_RED" "$task_json" "$cycle" "$remaining_items"; then
            log ERROR "RED step failed for $task_id"
            npx bd comment "$task_id" "BLOCKED: RED step failed after $TDD_STEP_RETRIES retries" 2>/dev/null || true
            return 1
        fi

        # --- GREEN: Write minimum code to pass ---
        head_before=$(git rev-parse HEAD 2>/dev/null)

        if ! execute_tdd_step "$STEP_GREEN" "$task_json" "$cycle"; then
            log ERROR "GREEN step failed for $task_id"
            npx bd comment "$task_id" "BLOCKED: GREEN step failed after $TDD_STEP_RETRIES retries" 2>/dev/null || true
            return 1
        fi

        # Check if GREEN committed (Claude should have committed)
        head_after=$(git rev-parse HEAD 2>/dev/null)
        if [[ "$head_before" == "$head_after" ]]; then
            log WARN "GREEN step did not commit - creating fallback commit"
            git add -A 2>/dev/null || true
            git commit -m "feat: implement $task_title (GREEN step - fallback commit)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>" 2>/dev/null || {
                log WARN "Fallback commit failed (possibly no changes)"
            }
        fi

        # --- REFACTOR: Improve code quality ---
        if ! execute_tdd_step "$STEP_REFACTOR" "$task_json" "$cycle"; then
            log WARN "REFACTOR step failed - continuing with GREEN state"
            # Non-fatal: REFACTOR failure preserves GREEN commit
        fi

        # --- REVIEW: Evaluate completeness ---
        # Clean up previous review file
        rm -f "$REVIEW_OUTPUT_FILE"

        if ! execute_tdd_step "$STEP_REVIEW" "$task_json" "$cycle"; then
            log WARN "REVIEW step failed - treating as incomplete"
            # Continue to next cycle
            remaining_items="(REVIEW step failed - please evaluate and implement any missing requirements)"
            continue
        fi

        # Parse review result
        if ! parse_review_result; then
            log WARN "Failed to parse review result - treating as incomplete"
            remaining_items="(Review JSON was invalid - please evaluate and implement any missing requirements)"
            continue
        fi

        # Check if complete
        if [[ "$REVIEW_COMPLETE" == "true" ]]; then
            log INFO "REVIEW says task is COMPLETE: $REVIEW_REASON"

            # Close the bead
            npx bd close "$task_id" 2>/dev/null || {
                log WARN "Failed to close bead $task_id"
            }

            # Final commit to capture .beads/ state
            git add -A 2>/dev/null || true
            git commit -m "chore: close bead $task_id

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>" 2>/dev/null || {
                log DEBUG "No additional changes to commit after bead close"
            }

            # Clean up review file
            rm -f "$REVIEW_OUTPUT_FILE"

            return 0
        fi

        # Incomplete - feed remaining items to next cycle's RED
        remaining_items="$REVIEW_REMAINING"
        log INFO "Cycle $cycle incomplete: $REVIEW_REASON"

        # Clean up review file between cycles
        rm -f "$REVIEW_OUTPUT_FILE"
    done

    # Exhausted all cycles
    log ERROR "Task $task_id exhausted $TDD_MAX_CYCLES TDD cycles - marking BLOCKED"
    npx bd comment "$task_id" "BLOCKED: exhausted $TDD_MAX_CYCLES TDD cycles without completion. Last review: $REVIEW_REASON" 2>/dev/null || true

    return 1
}

##############################################################################
# ATDD outer loop
##############################################################################

# Find the GWT spec file for a task by extracting US<N> from the task title.
# Arguments: task_json
# Returns: spec file path on stdout, exit 0 if found, exit 1 if not found
find_spec_for_task() {
    local task_json="$1"
    local task_title
    task_title=$(echo "$task_json" | jq -r '.title // ""')

    # Extract US<N> pattern from task title
    local us_id
    us_id=$(echo "$task_title" | grep -oP 'US\d+' | head -1 || true)

    if [[ -z "$us_id" ]]; then
        return 1
    fi

    # Find matching spec file
    local spec_file
    spec_file=$(find specs/ -name "${us_id}-*.txt" 2>/dev/null | head -1 || true)

    if [[ -z "$spec_file" || ! -f "$spec_file" ]]; then
        return 1
    fi

    echo "$spec_file"
    return 0
}

# Run acceptance check for a spec file.
# Arguments: spec_file
# Returns 0 if acceptance tests PASS, 1 if they fail or pipeline errors
run_acceptance_check() {
    local spec_file="$1"

    log INFO "Running acceptance check for $spec_file"

    # Clean IR artifacts (generated tests are preserved for bound implementations)
    rm -rf acceptance-pipeline/ir/

    # Run the pipeline
    if ! go run ./acceptance/cmd/pipeline -action=run 2>&1; then
        log DEBUG "Acceptance tests failing (expected during development)"
        return 1
    fi

    log INFO "Acceptance tests PASSING"
    return 0
}

# Execute ATDD cycle: outer acceptance loop wrapping inner TDD cycles.
# Tasks with matching spec files get the full ATDD treatment.
# Tasks without specs fall back to execute_unit_tdd_cycle.
# Arguments: task_json, epic_id
# Returns 0 on success (task complete), 1 on failure (BLOCKED)
execute_atdd_cycle() {
    local task_json="$1"
    local epic_id="$2"
    local task_id task_title
    local spec_file

    task_id=$(echo "$task_json" | jq -r '.id // "unknown"')
    task_title=$(echo "$task_json" | jq -r '.title // "unknown"')

    # Try to find a matching spec file
    if ! spec_file=$(find_spec_for_task "$task_json"); then
        log INFO "No spec file found for $task_id - falling back to unit TDD cycle"
        execute_unit_tdd_cycle "$task_json" "$epic_id"
        return $?
    fi

    log_section "ATDD CYCLE for $task_id: $task_title (spec: $spec_file)"

    # DRY RUN: show prompts and return
    if [[ "$DRY_RUN" == "true" ]]; then
        log INFO "[DRY RUN] ATDD cycle for $task_id with spec $spec_file"
        log INFO "[DRY RUN] Would run acceptance check, then BIND, then inner TDD cycles"
        execute_tdd_step "$STEP_BIND" "$task_json" 0 "" "$spec_file"
        execute_unit_tdd_cycle "$task_json" "$epic_id"
        return 0
    fi

    # Initial acceptance check - if tests already pass, task is done
    if run_acceptance_check "$spec_file"; then
        log INFO "Acceptance tests already passing for $task_id - closing bead"
        npx bd close "$task_id" 2>/dev/null || {
            log WARN "Failed to close bead $task_id"
        }
        git add -A 2>/dev/null || true
        git commit -m "chore: close bead $task_id (acceptance tests already passing)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>" 2>/dev/null || true
        return 0
    fi

    # --- BIND: Write acceptance test implementations ---
    log INFO "=== BIND: Writing acceptance test implementations ==="
    if ! execute_tdd_step "$STEP_BIND" "$task_json" 0 "" "$spec_file"; then
        log WARN "BIND step failed - continuing with stubs"
    fi

    # Inner TDD loop with acceptance check after each cycle
    local cycle=0
    local remaining_items=""

    while (( cycle < ATDD_MAX_INNER_CYCLES )); do
        cycle=$((cycle + 1))
        log INFO "=== ATDD Inner Cycle $cycle/$ATDD_MAX_INNER_CYCLES for $task_id ==="

        # Check baseline tests before RED
        if ! check_baseline_tests; then
            log WARN "Baseline tests failing - attempting auto-fix"
            if ! attempt_baseline_fix; then
                log ERROR "Baseline auto-fix failed - marking task BLOCKED"
                npx bd comment "$task_id" "BLOCKED: baseline tests failing (auto-fix failed after $BASELINE_FIX_MAX_ATTEMPTS attempts)" 2>/dev/null || true
                return 1
            fi
            log INFO "Baseline tests fixed - continuing to RED step"
            npx bd comment "$task_id" "Baseline tests were failing but auto-fixed" 2>/dev/null || true
        fi

        # --- RED: Write smallest possible failing unit test ---
        if ! execute_tdd_step "$STEP_RED" "$task_json" "$cycle" "$remaining_items"; then
            log ERROR "RED step failed for $task_id"
            npx bd comment "$task_id" "BLOCKED: RED step failed after $TDD_STEP_RETRIES retries" 2>/dev/null || true
            return 1
        fi

        # --- GREEN: Write minimal code to pass that one test ---
        local head_before head_after
        head_before=$(git rev-parse HEAD 2>/dev/null)

        if ! execute_tdd_step "$STEP_GREEN" "$task_json" "$cycle"; then
            log ERROR "GREEN step failed for $task_id"
            npx bd comment "$task_id" "BLOCKED: GREEN step failed after $TDD_STEP_RETRIES retries" 2>/dev/null || true
            return 1
        fi

        # Check if GREEN committed
        head_after=$(git rev-parse HEAD 2>/dev/null)
        if [[ "$head_before" == "$head_after" ]]; then
            log WARN "GREEN step did not commit - creating fallback commit"
            git add -A 2>/dev/null || true
            git commit -m "feat: implement $task_title (GREEN step - fallback commit)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>" 2>/dev/null || {
                log WARN "Fallback commit failed (possibly no changes)"
            }
        fi

        # --- REFACTOR: Improve code quality ---
        if ! execute_tdd_step "$STEP_REFACTOR" "$task_json" "$cycle"; then
            log WARN "REFACTOR step failed - continuing with GREEN state"
        fi

        # --- ACCEPTANCE CHECK after each inner cycle ---
        if run_acceptance_check "$spec_file"; then
            log INFO "Acceptance tests PASSING after cycle $cycle - task complete"
            npx bd close "$task_id" 2>/dev/null || {
                log WARN "Failed to close bead $task_id"
            }
            git add -A 2>/dev/null || true
            git commit -m "chore: close bead $task_id (acceptance tests passing)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>" 2>/dev/null || true
            rm -f "$ACCEPTANCE_OUTPUT_FILE"
            return 0
        fi

        log INFO "Acceptance tests still failing after cycle $cycle - continuing"
        remaining_items="Acceptance tests still failing. Continue implementing toward passing acceptance criteria in $spec_file."
    done

    # Exhausted all inner cycles
    log ERROR "Task $task_id exhausted $ATDD_MAX_INNER_CYCLES ATDD inner cycles - marking BLOCKED"
    npx bd comment "$task_id" "BLOCKED: exhausted $ATDD_MAX_INNER_CYCLES ATDD inner cycles without acceptance tests passing" 2>/dev/null || true
    rm -f "$ACCEPTANCE_OUTPUT_FILE"
    return 1
}

##############################################################################
# Claude CLI invocation
##############################################################################

# Invoke Claude CLI with focused prompt
# Returns the exit code from Claude
invoke_claude() {
    local prompt="$1"
    local exit_code
    local claude_output

    log INFO "Invoking Claude with focused prompt"
    log_section "CLAUDE INVOCATION - $(date -Iseconds)"
    log_block "Prompt" "$prompt"

    # Capture Claude output to a temp file for logging
    local temp_output
    temp_output=$(mktemp)

    # Write prompt to a temp file to avoid shell quoting issues with script -c
    local prompt_file
    prompt_file=$(mktemp)
    printf '%s' "$prompt" > "$prompt_file"

    # Use script(1) to provide a PTY — claude -p hangs when stdout is not a
    # terminal because its tool-execution framework requires a TTY.  Wrapping
    # with `script -qc` gives claude a pseudo-terminal while still capturing
    # output to a file.  We strip ANSI escape sequences afterwards.
    #
    # timeout -k 10: send SIGKILL 10s after SIGTERM if process ignores it.
    timeout -k 10 "$CLAUDE_TIMEOUT" \
        script -qc "claude -p \"\$(cat '$prompt_file')\" --output-format text" "$temp_output" &
    CLAUDE_PID=$!

    # Clean up prompt file when no longer needed (after claude reads it)
    ( sleep 5; rm -f "$prompt_file" ) &

    # Wait for the background process to complete
    if wait "$CLAUDE_PID"; then
        exit_code=0
        log INFO "Claude completed successfully"
    else
        exit_code=$?
        if [[ "$exit_code" -eq 124 ]]; then
            log ERROR "Claude timed out after ${CLAUDE_TIMEOUT}s"
        elif [[ "$exit_code" -eq 130 ]]; then
            log INFO "Claude interrupted by SIGINT"
            # Don't log output on interrupt - user is stopping the process
            rm -f "$temp_output"
            CLAUDE_PID=""
            return "$exit_code"
        else
            log ERROR "Claude failed with exit code: $exit_code"
        fi
    fi

    CLAUDE_PID=""

    # Log the output (strip ANSI/terminal escapes and script(1) header/footer)
    claude_output=$(sed -e 's/\x1b\[[0-9;?]*[a-zA-Z]//g' \
                        -e 's/\x1b\][^\x1b]*\x07//g' \
                        -e 's/\x1b\][0-9;]*[^\a]*//g' \
                        -e 's/\x1b(B//g' \
                        -e 's/\r//g' \
                        -e '/^Script started on/d' \
                        -e '/^Script done on/d' \
                        -e '/^\[COMMAND/d' \
                        "$temp_output")
    log_block "Claude Output" "$claude_output"
    rm -f "$temp_output"

    return "$exit_code"
}

##############################################################################
# Retry logic with exponential backoff
##############################################################################

# Calculate delay for a given retry attempt (exponential backoff capped at MAX_RETRY_DELAY)
# Delays: 1s, 2s, 4s, 8s, 16s, 32s, 64s, 128s, 256s, 300s (capped)
calculate_delay() {
    local attempt="$1"
    local delay

    # 2^(attempt-1) gives: 1, 2, 4, 8, 16, 32, 64, 128, 256, 512...
    delay=$((1 << (attempt - 1)))

    # Cap at MAX_RETRY_DELAY (300 seconds = 5 minutes)
    if (( delay > MAX_RETRY_DELAY )); then
        delay="$MAX_RETRY_DELAY"
    fi

    echo "$delay"
}

# Invoke Claude with exponential backoff retry
# Returns 0 on success, 1 if all retries exhausted
invoke_claude_with_retry() {
    local prompt="$1"
    local attempt=0
    local delay

    while (( attempt < MAX_RETRIES )); do
        attempt=$((attempt + 1))
        log DEBUG "Claude invocation attempt $attempt/$MAX_RETRIES"

        if invoke_claude "$prompt"; then
            if (( attempt > 1 )); then
                log INFO "Claude succeeded after $attempt attempts"
            fi
            return 0
        fi

        if (( attempt >= MAX_RETRIES )); then
            log ERROR "All $MAX_RETRIES retry attempts exhausted"
            return 1
        fi

        delay=$(calculate_delay "$attempt")
        log WARN "Retry $attempt/$MAX_RETRIES failed. Waiting ${delay}s before next attempt..."
        sleep "$delay"
    done

    return 1
}

##############################################################################
# Main loop
##############################################################################

run_loop() {
    local epic_id="$1"
    local iteration=0
    local next_task task_id task_title task_description task_type
    local is_resuming=false
    local task_source

    log INFO "Starting automation loop (max $MAX_ITERATIONS iterations)"
    log_section "AUTOMATION LOOP START"

    # Detect and log operational mode
    task_source=$(detect_task_source "$epic_id")
    if [[ "$task_source" == "spec-kit" ]]; then
        log INFO "Operating in spec-kit workflow mode"
        log INFO "Ralph will process tasks from the sp:* workflow phases"
    else
        log INFO "Operating in generic task workflow mode"
        log INFO "Ralph will process all tasks under the epic (no phase prerequisites)"
    fi

    while (( iteration < MAX_ITERATIONS )); do
        iteration=$((iteration + 1))
        CURRENT_ITERATION="$iteration"  # Update global for SIGINT handler

        log_section "ITERATION $iteration/$MAX_ITERATIONS"

        # Check for in-progress tasks first (resume interrupted work)
        log DEBUG "Checking for in-progress tasks..."
        if has_in_progress_tasks "$epic_id"; then
            is_resuming=true
            next_task=$(get_in_progress_task "$epic_id")
            log INFO "Resuming interrupted task"
        else
            # No in-progress tasks, check for ready tasks
            is_resuming=false
            log DEBUG "Checking for ready tasks..."
            if ! has_ready_tasks "$epic_id"; then
                # No ready tasks, but check if there are still open tasks (e.g., P3 tasks)
                log DEBUG "No ready tasks found. Checking for remaining open tasks..."
                if has_open_tasks "$epic_id"; then
                    local open_tasks open_count
                    open_tasks=$(get_open_tasks "$epic_id")
                    open_count=$(echo "$open_tasks" | jq 'length')
                    log WARN "No ready tasks, but $open_count open task(s) remain (possibly P3 or blocked tasks)"
                    log_block "Remaining Open Tasks" "$(echo "$open_tasks" | jq -r '.[] | "\(.id): \(.title) [priority: \(.priority // "none")]"')"
                    log ERROR "Cannot complete epic with open tasks remaining"
                    echo "" >&2
                    echo "[ralph] ERROR: Epic has $open_count open task(s) that are not ready:" >&2
                    echo "$open_tasks" | jq -r '.[] | "  - \(.id): \(.title) [priority: \(.priority // "none"), status: \(.status)]"' >&2
                    echo "" >&2
                    echo "These tasks may be:" >&2
                    echo "  - Low priority (P3) tasks waiting to be started" >&2
                    echo "  - Tasks blocked by dependencies" >&2
                    echo "  - Tasks that need manual intervention" >&2
                    echo "" >&2
                    echo "Please review these tasks and either:" >&2
                    echo "  - Close them if they're no longer needed" >&2
                    echo "  - Unblock them and let ralph continue" >&2
                    echo "  - Complete them manually" >&2
                    return "$EXIT_FAILURE"
                fi
                log INFO "No more ready tasks and no open tasks. Feature complete!"
                return "$EXIT_SUCCESS"
            fi
            next_task=$(get_next_task "$epic_id")
        fi

        # Extract task info for logging
        task_id=$(echo "$next_task" | jq -r '.id // "unknown"')
        task_title=$(echo "$next_task" | jq -r '.title // "unknown"')
        task_description=$(echo "$next_task" | jq -r '.description // "no description"')

        # Task type for logging (single-language Go project)
        task_type="Go"

        if [[ "$is_resuming" == "true" ]]; then
            log INFO "Iteration $iteration/$MAX_ITERATIONS"
            log INFO "Epic: $epic_id | Task: $task_id | [RESUMING] $task_title"
        else
            log INFO "Iteration $iteration/$MAX_ITERATIONS"
            log INFO "Epic: $epic_id | Task: $task_id | $task_title"
        fi

        log_block "Task Details" "ID: $task_id
Title: $task_title
Type: $task_type
Status: $(if [[ "$is_resuming" == "true" ]]; then echo "RESUMING IN-PROGRESS"; else echo "STARTING NEW"; fi)

Description:
$task_description"

        # In dry-run mode, show all four TDD prompts for this task and exit
        if [[ "$DRY_RUN" == "true" ]]; then
            execute_atdd_cycle "$next_task" "$epic_id"
            return "$EXIT_SUCCESS"
        fi

        # Execute ATDD cycle (acceptance-driven TDD) for this task
        if execute_atdd_cycle "$next_task" "$epic_id"; then
            log INFO "Task $task_id completed via ATDD cycle"

            # Auto-close parent container tasks if all children are now completed.
            # Walks up from the processed task, closing each ancestor whose
            # children are all closed. This unblocks dependents of the parent.
            auto_close_completed_parents "$task_id" "$epic_id"
        else
            log WARN "Task $task_id marked BLOCKED - continuing to next task"
            # Continue to next task instead of returning EXIT_FAILURE
        fi
    done

    if (( iteration >= MAX_ITERATIONS )); then
        log WARN "Maximum iterations ($MAX_ITERATIONS) reached"
        return "$EXIT_LIMIT_REACHED"
    fi

    return "$EXIT_SUCCESS"
}

##############################################################################
# Summary and reporting
##############################################################################

# Format duration in human-readable form
format_duration() {
    local seconds="$1"
    local minutes=$((seconds / 60))
    local remaining_seconds=$((seconds % 60))

    if (( minutes > 0 )); then
        echo "${minutes}m ${remaining_seconds}s"
    else
        echo "${remaining_seconds}s"
    fi
}

# Display completion summary
show_summary() {
    local exit_reason="$1"
    local end_time elapsed_seconds elapsed_formatted

    if (( START_TIME > 0 )); then
        end_time=$(date +%s)
        elapsed_seconds=$((end_time - START_TIME))
        elapsed_formatted=$(format_duration "$elapsed_seconds")
    else
        elapsed_formatted="N/A"
    fi

    # Log to file
    log_section "SESSION SUMMARY"
    {
        echo "Exit reason: $exit_reason"
        echo "Iterations completed: $CURRENT_ITERATION"
        echo "Elapsed time: $elapsed_formatted"
        echo "Ended: $(date -Iseconds)"
    } >> "$LOG_FILE"

    # Display to console
    echo ""
    echo "[ralph] ========================================="
    echo "[ralph] Summary: $exit_reason"
    echo "[ralph] Iterations: $CURRENT_ITERATION"
    echo "[ralph] Elapsed time: $elapsed_formatted"
    echo "[ralph] Log file: $LOG_FILE"
    echo "[ralph] ========================================="
}

##############################################################################
# Signal handlers
##############################################################################

# Handler for SIGINT (Ctrl+C)
handle_sigint() {
    echo ""
    log INFO "Received SIGINT, cleaning up..."

    # Kill the entire process group rooted at CLAUDE_PID (script -> claude -> node)
    if [[ -n "$CLAUDE_PID" ]] && kill -0 "$CLAUDE_PID" 2>/dev/null; then
        log INFO "Terminating Claude subprocess (PID: $CLAUDE_PID)"
        # Kill the process group (negative PID) to catch script + all children
        kill -TERM -- -"$CLAUDE_PID" 2>/dev/null || kill -TERM "$CLAUDE_PID" 2>/dev/null || true
        # Give it a moment to terminate gracefully
        sleep 1
        # Force kill if still running
        if kill -0 "$CLAUDE_PID" 2>/dev/null; then
            log WARN "Force killing Claude subprocess"
            kill -KILL -- -"$CLAUDE_PID" 2>/dev/null || kill -KILL "$CLAUDE_PID" 2>/dev/null || true
        fi
    fi

    show_summary "Interrupted by user"
    # EXIT trap will handle lock release
    exit "$EXIT_SIGINT"
}

# Cleanup handler (runs on EXIT)
cleanup() {
    release_lock
}

##############################################################################
# Main entry point
##############################################################################

main() {
    parse_args "$@"

    # Initialize logging infrastructure
    init_log

    log INFO "Configuration: dry_run=$DRY_RUN, max_iterations=$MAX_ITERATIONS, explicit_epic=$EXPLICIT_EPIC_ID"
    log INFO "Log file: $LOG_FILE"

    # Record start time for summary
    START_TIME=$(date +%s)

    # Detect the epic for this branch
    local epic_id branch
    branch=$(get_current_branch) || exit "$EXIT_FAILURE"
    epic_id=$(detect_epic) || exit "$EXIT_FAILURE"

    log INFO "Working on epic: $epic_id"

    # Skip lock and prerequisites in dry-run mode for testing
    if [[ "$DRY_RUN" != "true" ]]; then
        # Acquire lock to prevent concurrent runs
        acquire_lock "$branch" || exit "$EXIT_FAILURE"

        # Set up traps
        trap cleanup EXIT
        trap handle_sigint SIGINT

        # Validate prerequisites
        validate_prerequisites "$epic_id" || exit "$EXIT_FAILURE"
    fi

    # Run the main loop
    local result
    if run_loop "$epic_id"; then
        result=$?
    else
        result=$?
    fi

    # Show summary based on exit reason
    case "$result" in
        "$EXIT_SUCCESS")
            show_summary "All tasks completed successfully"
            ;;
        "$EXIT_LIMIT_REACHED")
            show_summary "Maximum iterations reached"
            ;;
        "$EXIT_FAILURE")
            show_summary "Execution failed"
            ;;
    esac

    exit "$result"
}

main "$@"
