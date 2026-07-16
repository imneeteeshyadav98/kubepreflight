#!/usr/bin/env bash
# Runs a kubepreflight scan for one phase of the case study and writes
# every artifact into demo/eks-case-study/evidence/<phase>/. The same
# --manifests demo/eks/manifests is used for every phase, so API-001 (the
# scan-only deprecated-API fixture) appears identically across before/
# after-remediation/after-upgrade -- comparisons stay apples-to-apples.
#
# Usage: ./02-scan.sh <phase-label>
#   e.g. ./02-scan.sh before
#        ./02-scan.sh after-remediation
#        ./02-scan.sh after-upgrade
#
# CLUSTER_NAME/REGION/TARGET_VERSION/KUBEPREFLIGHT_BIN can all be
# overridden via environment variable, matching demo/eks/cleanup.sh's
# existing convention.
set -euo pipefail

phase="${1:?usage: 02-scan.sh <phase-label>, e.g. before / after-remediation / after-upgrade}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
manifests_dir="${repo_root}/demo/eks/manifests"

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-case-study}"
TARGET_VERSION="${TARGET_VERSION:-1.32}"
KUBEPREFLIGHT_BIN="${KUBEPREFLIGHT_BIN:-kubepreflight}"

out_dir="${repo_root}/demo/eks-case-study/evidence/${phase}"
mkdir -p "${out_dir}"

"${KUBEPREFLIGHT_BIN}" scan \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --target-version "${TARGET_VERSION}" \
  --manifests "${manifests_dir}" \
  --output all \
  --findings-out "${out_dir}/findings.json" \
  --output-dir "${out_dir}" \
  --serve-report never

echo ""
echo "Evidence written to ${out_dir}/"
