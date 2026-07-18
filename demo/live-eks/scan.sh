#!/usr/bin/env bash
# Runs a kubepreflight scan for one phase of the live demo and writes
# every artifact into demo/live-eks/evidence/<phase>/. --manifests points
# at old-api.yaml specifically (scan-only, never applied to the cluster --
# see that file's own warning comment), matching
# scripts/case-study/02-scan.sh's exact reasoning for not scanning the
# whole demo/eks/manifests/ directory.
#
# Usage: ./scan.sh <phase-label>
#   e.g. ./scan.sh before
#        ./scan.sh after
#
# CLUSTER_NAME/REGION/TARGET_VERSION/KUBEPREFLIGHT_BIN can all be
# overridden via environment variable.
set -euo pipefail

phase="${1:?usage: scan.sh <phase-label>, e.g. before / after}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
old_api_manifest="${repo_root}/demo/eks/manifests/old-api.yaml"

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-live-demo}"
TARGET_VERSION="${TARGET_VERSION:-1.36}"
KUBEPREFLIGHT_BIN="${KUBEPREFLIGHT_BIN:-kubepreflight}"

out_dir="${repo_root}/demo/live-eks/evidence/${phase}"
mkdir -p "${out_dir}"

"${KUBEPREFLIGHT_BIN}" scan \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --target-version "${TARGET_VERSION}" \
  --manifests "${old_api_manifest}" \
  --output all \
  --findings-out "${out_dir}/findings.json" \
  --output-dir "${out_dir}" \
  --serve-report never

echo ""
echo "Evidence written to ${out_dir}/"
