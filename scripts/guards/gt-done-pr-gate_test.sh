#!/bin/bash
# Tests for the gt done PR gate wrapper at ~/gt/bin/gt
#
# These tests verify that gt done blocks when:
# 1. No PR exists for the current branch
# 2. gh pr list fails (network error, auth error)
# 3. gh pr list returns non-numeric output
#
# And allows through when:
# 4. PR exists (gh pr list returns > 0)
# 5. --status ESCALATED is passed
# 6. --status DEFERRED is passed
# 7. --cleanup-status clean is passed
# 8. Branch is main or release

set -euo pipefail

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Create a temporary directory for test fixtures
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Create a mock gt wrapper that only runs the PR check logic
# (extracted from ~/gt/bin/gt, without exec to real gt)
cat > "$TMPDIR/gt-test" << 'WRAPPER'
#!/bin/bash
# Test harness: extracted PR gate logic from gt wrapper

case "${1:-}" in
    done)
        SKIP_PR_CHECK=false
        for arg in "$@"; do
            case "$arg" in
                --status|--status=ESCALATED|--status=DEFERRED)
                    SKIP_PR_CHECK=true
                    ;;
                --cleanup-status|--cleanup-status=clean)
                    SKIP_PR_CHECK=true
                    ;;
            esac
        done
        PREV_ARG=""
        for arg in "$@"; do
            if [[ "$PREV_ARG" == "--status" && ("$arg" == "ESCALATED" || "$arg" == "DEFERRED") ]]; then
                SKIP_PR_CHECK=true
            fi
            if [[ "$PREV_ARG" == "--cleanup-status" && "$arg" == "clean" ]]; then
                SKIP_PR_CHECK=true
            fi
            PREV_ARG="$arg"
        done

        if [[ "$SKIP_PR_CHECK" == "false" ]]; then
            CURRENT_BRANCH=$(git branch --show-current 2>/dev/null)
            if [[ -z "$CURRENT_BRANCH" ]]; then
                echo "ERROR: Could not determine current git branch." >&2
                exit 1
            fi
            if [[ "$CURRENT_BRANCH" != "main" && "$CURRENT_BRANCH" != "release" ]]; then
                PR_OUTPUT=$(gh pr list --head "$CURRENT_BRANCH" --state open --json number --jq length 2>&1)
                GH_EXIT=$?
                if [[ $GH_EXIT -ne 0 ]]; then
                    echo "ERROR: gh pr list failed (exit code $GH_EXIT)." >&2
                    exit 1
                fi
                if ! [[ "$PR_OUTPUT" =~ ^[0-9]+$ ]]; then
                    echo "ERROR: gh pr list returned unexpected output: '$PR_OUTPUT'" >&2
                    exit 1
                fi
                if [[ "$PR_OUTPUT" == "0" ]]; then
                    echo "ERROR: No open GitHub PR found." >&2
                    exit 1
                fi
            fi
        fi
        echo "PR_CHECK_PASSED"
        exit 0
        ;;
esac
WRAPPER
chmod +x "$TMPDIR/gt-test"

# Create a mock git repo
git init "$TMPDIR/repo" --initial-branch=feature-test > /dev/null 2>&1
cd "$TMPDIR/repo"
git commit --allow-empty -m "initial" > /dev/null 2>&1

# Helper to run test
run_test() {
    local name="$1"
    local expected_exit="$2"
    shift 2
    local actual_exit=0
    OUTPUT=$("$TMPDIR/gt-test" "$@" 2>&1) || actual_exit=$?
    if [[ "$actual_exit" -eq "$expected_exit" ]]; then
        echo "  PASS: $name"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $name (expected exit=$expected_exit, got exit=$actual_exit)"
        echo "        output: $OUTPUT"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== gt done PR gate tests ==="

# --- Test 1: gh pr list fails (mock gh to fail) ---
export PATH="$TMPDIR/mock-bin:$PATH"
mkdir -p "$TMPDIR/mock-bin"

# Mock gh that exits with error
cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo "HTTP 401: Bad credentials" >&2
exit 1
MOCK
chmod +x "$TMPDIR/mock-bin/gh"

run_test "gh pr list failure blocks gt done" 1 done

# --- Test 2: gh pr list returns non-numeric output ---
cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo "null"
exit 0
MOCK

run_test "gh pr list non-numeric output blocks gt done" 1 done

# --- Test 3: gh pr list returns 0 (no PR) ---
cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo "0"
exit 0
MOCK

run_test "no PR blocks gt done" 1 done

# --- Test 4: gh pr list returns 1 (PR exists) ---
cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo "1"
exit 0
MOCK

run_test "existing PR allows gt done" 0 done

# --- Test 5: --status ESCALATED skips check ---
cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo "0"
exit 0
MOCK

run_test "--status ESCALATED skips PR check" 0 done --status ESCALATED

# --- Test 6: --status DEFERRED skips check ---
run_test "--status DEFERRED skips PR check" 0 done --status DEFERRED

# --- Test 7: --cleanup-status clean skips check ---
run_test "--cleanup-status clean skips PR check" 0 done --cleanup-status clean

# --- Test 8: main branch skips check ---
cd "$TMPDIR"
git init repo-main --initial-branch=main > /dev/null 2>&1
cd repo-main
git commit --allow-empty -m "initial" > /dev/null 2>&1

cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo "0"
exit 0
MOCK

run_test "main branch skips PR check" 0 done

# --- Test 9: release branch skips check ---
git checkout -b release > /dev/null 2>&1

run_test "release branch skips PR check" 0 done

# --- Test 10: gh pr list returns empty string ---
cd "$TMPDIR/repo"
cat > "$TMPDIR/mock-bin/gh" << 'MOCK'
#!/bin/bash
echo ""
exit 0
MOCK

run_test "gh pr list empty output blocks gt done" 1 done

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ "$FAIL" -eq 0 ]]
