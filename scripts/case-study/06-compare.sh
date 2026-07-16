#!/usr/bin/env bash
# Runs `kubepreflight compare` (comparison + gate) between any two
# evidence phases 02-scan.sh already captured, writing results into
# demo/eks-case-study/evidence/compare/<label>/.
#
# Usage: ./06-compare.sh <baseline-phase> <current-phase> <output-label>
#   e.g. ./06-compare.sh before after-remediation resolved-findings
#        ./06-compare.sh after-remediation after-upgrade upgrade-regression
set -euo pipefail

baseline_phase="${1:?usage: 06-compare.sh <baseline-phase> <current-phase> <output-label>}"
current_phase="${2:?usage: 06-compare.sh <baseline-phase> <current-phase> <output-label>}"
label="${3:?usage: 06-compare.sh <baseline-phase> <current-phase> <output-label>}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
KUBEPREFLIGHT_BIN="${KUBEPREFLIGHT_BIN:-kubepreflight}"

evidence_dir="${repo_root}/demo/eks-case-study/evidence"
baseline_findings="${evidence_dir}/${baseline_phase}/findings.json"
current_findings="${evidence_dir}/${current_phase}/findings.json"
out_dir="${evidence_dir}/compare/${label}"

if [[ ! -f "${baseline_findings}" ]]; then
  echo "error: ${baseline_findings} not found -- run 02-scan.sh ${baseline_phase} first." >&2
  exit 1
fi
if [[ ! -f "${current_findings}" ]]; then
  echo "error: ${current_findings} not found -- run 02-scan.sh ${current_phase} first." >&2
  exit 1
fi

mkdir -p "${out_dir}"

"${KUBEPREFLIGHT_BIN}" compare \
  --baseline "${baseline_findings}" \
  --current "${current_findings}" \
  --json-out "${out_dir}/comparison.json" \
  --markdown-out "${out_dir}/comparison.md" \
  --gate-out "${out_dir}/gate.json"

echo ""
echo "Comparison (${baseline_phase} -> ${current_phase}) written to ${out_dir}/"
cat "${out_dir}/gate.json"
