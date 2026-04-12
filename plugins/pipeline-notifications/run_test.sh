#!/usr/bin/env bash
# POPANDPEEK-FORK BEGIN: hq-kkata deacon BEAD_STATUS parser tests.
# Tests for the title-parsing logic in run.sh. The parser runs inside a
# nohup bash -c subshell, so we can not source run.sh directly — the logic
# is duplicated below and must stay in sync with the deacon case block in
# run.sh. If you change the parser in run.sh, update this file too.
#
# What is tested:
#   - BEAD_ID / NEW_STATUS / PR_NUM extraction across 8 realistic titles
#   - The "BEAD_STATUS: <id> to <status>" case pattern matches only on
#     valid deacon inputs and not on adjacent ntfy titles (PR merged, etc).
set -euo pipefail

PASS=0
FAIL=0

# parse_deacon_fields echoes "<id>|<status>|<pr>" from an input title.
# Keeps the same grep/sed pipeline as run.sh.
parse_deacon_fields() {
  local title="$1"
  local bead_id new_status pr_num
  bead_id=$(echo "$title" | grep -oE "\b[a-z]+-[a-z0-9]+" | head -1 || true)
  new_status=$(echo "$title" | sed -n "s/.* to \([a-z_]*\).*/\1/p" || true)
  pr_num=$(echo "$title" | grep -oE "PR #[0-9]+" | grep -oE "[0-9]+" || true)
  echo "${bead_id}|${new_status}|${pr_num}"
}

# matches_deacon_case returns "yes" if the title matches the case glob used
# in run.sh (the deacon auto-flip branch).
matches_deacon_case() {
  local title="$1"
  case "$title" in
    "BEAD_STATUS: "*" to "*) echo "yes" ;;
    *) echo "no" ;;
  esac
}

check_fields() {
  local name="$1" title="$2" want_id="$3" want_status="$4" want_pr="$5"
  local got want
  got=$(parse_deacon_fields "$title")
  want="${want_id}|${want_status}|${want_pr}"
  if [ "$got" = "$want" ]; then
    PASS=$((PASS+1))
    printf "  PASS  %s\n" "$name"
  else
    FAIL=$((FAIL+1))
    printf "  FAIL  %s — got %q want %q\n" "$name" "$got" "$want"
  fi
}

check_case() {
  local name="$1" title="$2" want="$3"
  local got
  got=$(matches_deacon_case "$title")
  if [ "$got" = "$want" ]; then
    PASS=$((PASS+1))
    printf "  PASS  %s\n" "$name"
  else
    FAIL=$((FAIL+1))
    printf "  FAIL  %s — got %q want %q\n" "$name" "$got" "$want"
  fi
}

echo "Deacon BEAD_STATUS parser tests"
echo

# Happy-path cases — titles observed in the live watcher log today.
check_fields "reviewing + PR"     "BEAD_STATUS: be-amhql to reviewing | PR #710"  "be-amhql" "reviewing" "710"
check_fields "closed + PR"        "BEAD_STATUS: be-fa0dm to closed | PR #696"     "be-fa0dm" "closed"    "696"
check_fields "deploying + PR"     "BEAD_STATUS: be-u06ok to deploying | PR #681"  "be-u06ok" "deploying" "681"
check_fields "hq bead"            "BEAD_STATUS: hq-m5sjf to reviewing | PR #13"   "hq-m5sjf" "reviewing" "13"
check_fields "no PR suffix"       "BEAD_STATUS: be-xyzab to closed"               "be-xyzab" "closed"    ""
check_fields "alphanumeric id"    "BEAD_STATUS: be-abc123 to working | PR #1"     "be-abc123" "working"  "1"
check_fields "custom status"      "BEAD_STATUS: be-abc to planning | PR #2"       "be-abc"   "planning"  "2"
check_fields "open (rework)"      "BEAD_STATUS: be-tdf4n to open | PR #694"       "be-tdf4n" "open"      "694"

# Case-pattern tests — verify the deacon case glob only matches intended inputs.
echo
check_case "match BEAD_STATUS"     "BEAD_STATUS: be-amhql to reviewing | PR #710"  "yes"
check_case "no-match PR merged"    "PR merged: #709"                               "no"
check_case "no-match PR Validation" "PR Validation | Passed | PR #709"             "no"
check_case "no-match Released"     "Released v0.0.362"                             "no"
check_case "no-match REVERT"       "REVERT: Released v0.0.362"                     "no"
check_case "no-match empty"        ""                                              "no"
check_case "no-match garbage"      "CI event with no bead"                         "no"

echo
echo "Summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
# POPANDPEEK-FORK END
