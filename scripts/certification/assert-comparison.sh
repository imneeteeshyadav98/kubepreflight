#!/usr/bin/env bash
# Proves PR 5's core invariant (internal/comparison/compare.go's
# ruleProvenEvaluated) against a real comparison.json + the current
# findings.json it was built from: a baseline finding whose responsible
# rule is not proven "applicable and evaluated" in `current` must appear in
# comparison.json's "not_re_evaluated" bucket, and NEVER in "resolved".
#
# This is the automated form of
# docs/certification/v1.3.0/expected-results.md's "Comparison proof"
# section. Safe to run against any local pair of files -- no network
# access, no AWS/cluster access.
#
# Usage: assert-comparison.sh --current <findings.json> --comparison <comparison.json> [--min-not-re-evaluated N]
#
# --min-not-re-evaluated N (optional, default 0): fail if
#   comparison.json's "not_re_evaluated" array has fewer than N entries.
#   Added during Stage 2 independent review -- without this, the two
#   structural PASS checks below are satisfied trivially by an EMPTY
#   not_re_evaluated array, which would make the primary certification
#   proof (docs/certification/v1.3.0/expected-results.md's "Comparison
#   proof" section) pass vacuously if the real cluster this certification
#   runs against happens to produce zero findings from any rule outside
#   {API-001, API-002} -- i.e. the script would report OK even though it
#   never actually observed the not_re_evaluated bucket firing on real
#   data. Pass --min-not-re-evaluated 1 (or higher) for any comparison
#   this certification treats as its non-vacuous proof; leave it at the
#   default 0 for a comparison where an empty bucket is a legitimately
#   valid outcome (there is no such case in this certification's own
#   Phase 5 today, but the flag stays optional so this script remains
#   reusable for a future comparison with different expectations).
#
# Exit 0: invariant holds for every entry in both "resolved" and
#         "not_re_evaluated", AND not_re_evaluated has >= N entries.
# Exit 1: at least one entry violates the invariant (each printed), or
#         not_re_evaluated has fewer than N entries.
# Exit 2: usage error, missing file, or missing jq.
set -euo pipefail

current_path=""
comparison_path=""
min_not_re_evaluated=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --current) current_path="$2"; shift 2 ;;
    --comparison) comparison_path="$2"; shift 2 ;;
    --min-not-re-evaluated) min_not_re_evaluated="$2"; shift 2 ;;
    *) echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$current_path" || -z "$comparison_path" ]]; then
  echo "usage: $(basename "$0") --current <findings.json> --comparison <comparison.json>" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required" >&2
  exit 2
fi
for f in "$current_path" "$comparison_path"; do
  [[ -f "$f" ]] || { echo "error: not a file: $f" >&2; exit 2; }
done

# Build a ruleId -> "applicable+evaluated" (true/false) lookup from
# current's own ruleExecutions -- mirrors
# internal/comparison/compare.go's indexRuleExecutions +
# ruleProvenEvaluated exactly: a rule ID with no record at all is treated
# as NOT proven (false), same as a real lookup miss would be.
proven_map=$(jq -c '
  [.ruleExecutions[] | {key: .ruleId, value: (.applicability == "applicable" and .state == "evaluated")}] | from_entries
' "$current_path")

failures=0

# Every "resolved" entry's ruleId MUST be proven true.
bad_resolved=$(jq -c --argjson proven "$proven_map" '
  [.resolved[] | select(($proven[.ruleId] // false) | not) | {ruleId, fingerprint: (.fingerprint // "unknown")}]
' "$comparison_path")
bad_resolved_count=$(echo "$bad_resolved" | jq 'length')
if [[ "$bad_resolved_count" == "0" ]]; then
  echo "PASS: every \"resolved\" entry's rule is proven applicable+evaluated in current"
else
  echo "FAIL: $bad_resolved_count \"resolved\" entr(y/ies) whose rule is NOT proven evaluated in current -- this is exactly the false-resolution bug PR 5 exists to prevent:"
  echo "$bad_resolved" | jq -c '.[]' | sed 's/^/  /'
  failures=$((failures + bad_resolved_count))
fi

# Every "not_re_evaluated" entry's ruleId MUST be proven false (i.e. must
# NOT be applicable+evaluated) -- a rule that genuinely is proven evaluated
# has no business being in this bucket; it should have been "resolved".
bad_not_re_evaluated=$(jq -c --argjson proven "$proven_map" '
  [.not_re_evaluated[] | select($proven[.ruleId] // false) | {ruleId, fingerprint: (.fingerprint // "unknown")}]
' "$comparison_path")
bad_nre_count=$(echo "$bad_not_re_evaluated" | jq 'length')
if [[ "$bad_nre_count" == "0" ]]; then
  echo "PASS: no \"not_re_evaluated\" entry's rule is actually proven evaluated in current (bucket is conservative, as designed)"
else
  echo "FAIL: $bad_nre_count \"not_re_evaluated\" entr(y/ies) whose rule IS proven evaluated in current -- these should have been classified \"resolved\" instead:"
  echo "$bad_not_re_evaluated" | jq -c '.[]' | sed 's/^/  /'
  failures=$((failures + bad_nre_count))
fi

resolved_count=$(jq '.resolved | length' "$comparison_path")
nre_count=$(jq '.not_re_evaluated | length' "$comparison_path")
echo ""
echo "Summary: resolved=$resolved_count not_re_evaluated=$nre_count"

if [[ "$nre_count" -lt "$min_not_re_evaluated" ]]; then
  echo "FAIL: not_re_evaluated has $nre_count entr(y/ies), want >= $min_not_re_evaluated -- an empty (or too-small) not_re_evaluated bucket means this comparison never actually exercised the invariant it exists to prove, even though no entry violated it. If the real cluster genuinely produced no findings from any rule outside the current mode's applicable set, that is itself a certification-planning problem (see expected-results.md's 'Comparison proof' section) -- it means this particular baseline/current pair cannot serve as this certification's central not_re_evaluated proof and a different baseline (or a cluster with more real findings) is needed, not that the check should be loosened."
  failures=$((failures + 1))
fi

if [[ $failures -gt 0 ]]; then
  echo "assert-comparison.sh: FAILED -- $failures invariant violation(s)" >&2
  exit 1
fi
echo "assert-comparison.sh: OK -- not_re_evaluated invariant holds"
exit 0
