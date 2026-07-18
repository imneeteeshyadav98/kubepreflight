#!/usr/bin/env bash
# Runs kubepreflight compare between the before/ and after/ evidence this
# demo's scan.sh already captured, writing results into
# demo/live-eks/evidence/compare/. Same shape as
# scripts/case-study/06-compare.sh.
#
# --redact-sensitive-identifiers is on here too: scan.sh already redacts
# before/after/findings.json, but compare's own output (New/Resolved/
# Unchanged entries embed the full finding, see internal/comparison's own
# comment) is redacted independently rather than assumed inherited, so
# this stays correct even if someone runs compare against evidence that
# wasn't itself produced with the flag.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
KUBEPREFLIGHT_BIN="${KUBEPREFLIGHT_BIN:-kubepreflight}"

evidence_dir="${repo_root}/demo/live-eks/evidence"
baseline_findings="${evidence_dir}/before/findings.json"
current_findings="${evidence_dir}/after/findings.json"
out_dir="${evidence_dir}/compare"

if [[ ! -f "${baseline_findings}" ]]; then
  echo "error: ${baseline_findings} not found -- run scan.sh before first." >&2
  exit 1
fi
if [[ ! -f "${current_findings}" ]]; then
  echo "error: ${current_findings} not found -- run scan.sh after first." >&2
  exit 1
fi

mkdir -p "${out_dir}"

"${KUBEPREFLIGHT_BIN}" compare \
  --baseline "${baseline_findings}" \
  --current "${current_findings}" \
  --json-out "${out_dir}/comparison.json" \
  --markdown-out "${out_dir}/comparison.md" \
  --gate-out "${out_dir}/gate.json" \
  --redact-sensitive-identifiers

echo ""
echo "Comparison (before -> after) written to ${out_dir}/"
cat "${out_dir}/gate.json"
