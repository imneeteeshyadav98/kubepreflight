#!/usr/bin/env bash
# Runs a kubepreflight scan for one phase of the case study and writes
# every artifact into demo/eks-case-study/evidence/<phase>/. --manifests
# points at old-api.yaml specifically, not the whole demo/eks/manifests/
# directory -- that directory also holds pdb-lab.yaml and
# broken-webhook.yaml, which are meant to be seen live (via kubectl apply,
# see 01-seed-fixtures.sh), not manifest-scanned too. Pointing --manifests
# at the whole directory double-counts them: once as live cluster objects,
# once again as static manifests for the same real objects, producing
# redundant manifest-plane findings (confirmed against a real cluster
# while first running this case study -- see docs/case-studies/
# eks-1.31-to-1.32.md section 8). --manifests accepts a single file path
# directly, not just a directory (see relativeSourcePath's own comment in
# internal/collectors/manifest/collector.go).
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
old_api_manifest="${repo_root}/demo/eks/manifests/old-api.yaml"

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-case-study}"
TARGET_VERSION="${TARGET_VERSION:-1.32}"
KUBEPREFLIGHT_BIN="${KUBEPREFLIGHT_BIN:-kubepreflight}"

out_dir="${repo_root}/demo/eks-case-study/evidence/${phase}"
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
