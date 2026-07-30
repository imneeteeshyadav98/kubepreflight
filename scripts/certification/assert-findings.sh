#!/usr/bin/env bash
# Asserts the invariants docs/certification/v1.3.0/expected-results.md
# documents for a given findings.json, per evidence mode. Safe to run
# against any findings.json, including synthetic/local fixtures with no
# AWS access at all (a --manifests-only scan output, for example) -- this
# script never touches the network itself, it only reads a file already on
# disk.
#
# Usage: assert-findings.sh <mode> <findings.json path>
#   mode: full-access | reduced-iam | manifests-only
#
# Exit 0: every assertion for the given mode passed.
# Exit 1: at least one assertion failed (each failure is printed).
# Exit 2: usage error, missing file, or missing jq.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $(basename "$0") <full-access|reduced-iam|manifests-only> <findings.json path>" >&2
  exit 2
fi
mode="$1"
path="$2"

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required" >&2
  exit 2
fi
if [[ ! -f "$path" ]]; then
  echo "error: not a file: $path" >&2
  exit 2
fi
case "$mode" in
  full-access|reduced-iam|manifests-only) ;;
  *) echo "error: mode must be one of full-access, reduced-iam, manifests-only (got: $mode)" >&2; exit 2 ;;
esac

# The canonical 31-rule universe, in rules.AllRuleIDs() registration order
# (internal/rules/defaults.go's NewDefaultRegistry) -- confirmed by reading
# every rule's ID() method directly, not assumed from naming convention
# (e.g. EKS-NG-001, not EKSNG-001; EKS-INSIGHT-001, not EKSINSIGHT-001).
expected_rule_ids='["API-001","API-002","WH-001","WH-002","WH-004","WH-005","DRAIN-001","DRAIN-002","DRAIN-003","DRAIN-004","DRAIN-005","PDB-001","PDB-002","NODE-001","NODE-002","NODE-003","NET-002","WORKLOAD-001","ADDON-001","ADDON-002","EKS-NG-001","EKS-NG-002","EKS-NG-003","EKS-NG-004","EKS-INSIGHT-001","EKS-INSIGHT-002","EKS-INSIGHT-003","COREDNS-001","CRD-001","CRD-002","APISERVICE-001"]'
manifest_applicable_rule_ids='["API-001","API-002"]'
# Every registered rule now reports per-rule evidence dependency state.
# A reduced Kubernetes or AWS evidence plane may therefore produce
# "insufficient_evidence" on any rule in the registered universe; the
# assertion below still rejects impossible/unknown rule IDs.
insufficient_evidence_capable_rule_ids="$expected_rule_ids"
reduced_iam_expected_rule_ids='["NODE-002","NET-002","ADDON-001","ADDON-002","EKS-NG-001","EKS-NG-002","EKS-NG-003","EKS-NG-004","EKS-INSIGHT-001","EKS-INSIGHT-002","EKS-INSIGHT-003"]'

failures=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; failures=$((failures + 1)); }

check_eq() {
  local desc="$1" got="$2" want="$3"
  if [[ "$got" == "$want" ]]; then pass "$desc (got $got)"; else fail "$desc (got $got, want $want)"; fi
}

# --- Universal assertions (every mode) ---

schema_version=$(jq -r '.schemaVersion' "$path")
check_eq "schemaVersion == \"1.1\"" "$schema_version" "1.1"

rule_count=$(jq '.ruleExecutions | length' "$path")
check_eq "ruleExecutions count == 31" "$rule_count" "31"

normalized=$(jq -r '.ruleExecutionsNormalized // false' "$path")
check_eq "ruleExecutionsNormalized == false (native report)" "$normalized" "false"

id_set_match=$(jq --argjson expected "$expected_rule_ids" '
  ([.ruleExecutions[].ruleId] | sort) == ($expected | sort)
' "$path")
check_eq "ruleExecutions rule-ID set matches the 31-rule universe exactly" "$id_set_match" "true"

dup_count=$(jq '[.ruleExecutions[].ruleId] | group_by(.) | map(select(length > 1)) | length' "$path")
check_eq "no duplicate ruleId entries in ruleExecutions" "$dup_count" "0"

# insufficient_evidence must never appear for a rule outside the registered
# universe.
bad_insufficient=$(jq --argjson allowed "$insufficient_evidence_capable_rule_ids" '
  [.ruleExecutions[] | select(.state == "insufficient_evidence") | select(.ruleId as $id | ($allowed | index($id)) | not) | .ruleId]
' "$path")
bad_insufficient_count=$(echo "$bad_insufficient" | jq 'length')
if [[ "$bad_insufficient_count" == "0" ]]; then
  pass "no rule outside the registered rule universe reports state insufficient_evidence"
else
  fail "unexpected insufficient_evidence on untracked rule(s): $(echo "$bad_insufficient" | jq -c .)"
fi

# --- Mode-specific assertions ---

case "$mode" in
  manifests-only)
    applicable_ok=$(jq --argjson ids "$manifest_applicable_rule_ids" '
      [.ruleExecutions[] | select(.ruleId as $id | ($ids | index($id)))] |
      all(.applicability == "applicable" and .state == "evaluated")
    ' "$path")
    check_eq "API-001/API-002 are applicable+evaluated" "$applicable_ok" "true"

    excluded_ok=$(jq --argjson ids "$manifest_applicable_rule_ids" '
      [.ruleExecutions[] | select(.ruleId as $id | ($ids | index($id)) | not)] |
      all(.applicability == "not_applicable" and .state == "not_evaluated")
    ' "$path")
    check_eq "remaining 29 rules are not_applicable+not_evaluated" "$excluded_ok" "true"

    excluded_count=$(jq --argjson ids "$manifest_applicable_rule_ids" '
      [.ruleExecutions[] | select(.ruleId as $id | ($ids | index($id)) | not)] | length
    ' "$path")
    check_eq "excluded-rule count == 29" "$excluded_count" "29"

    k8s_status=$(jq -r '.coverage.kubernetes.status' "$path")
    check_eq "coverage.kubernetes.status == \"skipped\"" "$k8s_status" "skipped"

    aws_status=$(jq -r '.coverage.aws.status' "$path")
    check_eq "coverage.aws.status == \"skipped\"" "$aws_status" "skipped"
    ;;

  full-access)
    applicable_count=$(jq '[.ruleExecutions[] | select(.applicability == "not_applicable")] | length' "$path")
    check_eq "no rule is not_applicable in full-access mode" "$applicable_count" "0"
    ;;

  reduced-iam)
    aws_status=$(jq -r '.coverage.aws.status' "$path")
    if [[ "$aws_status" == "partial" ]]; then
      pass "coverage.aws.status == \"partial\" (IAM restriction visible at the plane level, as expected)"
    else
      fail "coverage.aws.status expected \"partial\" under a reduced IAM policy, got \"$aws_status\" -- either the policy wasn't actually reduced, or the AWS collector's error surfacing changed"
    fi

    unexpected_k8s_insufficient=$(jq --argjson ids "$reduced_iam_expected_rule_ids" '
      [.ruleExecutions[] | select(.state == "insufficient_evidence") | select(.ruleId as $id | ($ids | index($id)) | not) | .ruleId]
    ' "$path")
    unexpected_k8s_insufficient_count=$(echo "$unexpected_k8s_insufficient" | jq 'length')
    if [[ "$unexpected_k8s_insufficient_count" == "0" ]]; then
      pass "reduced-IAM insufficient_evidence is confined to AWS-dependent rules"
    else
      fail "reduced-IAM produced insufficient_evidence outside expected AWS-dependent rules: $(echo "$unexpected_k8s_insufficient" | jq -c .)"
    fi
    ;;
esac

echo ""
if [[ $failures -gt 0 ]]; then
  echo "assert-findings.sh: FAILED -- $failures assertion(s) failed for mode=$mode, file=$path" >&2
  exit 1
fi
echo "assert-findings.sh: OK -- all assertions passed for mode=$mode, file=$path"
exit 0
