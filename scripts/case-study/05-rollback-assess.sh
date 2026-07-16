#!/usr/bin/env bash
# Runs a real rollback readiness assessment against the post-upgrade
# cluster and writes every artifact into
# demo/eks-case-study/evidence/rollback/. Read-only -- never executes a
# rollback or mutates the cluster (see `kubepreflight rollback --help`).
#
# Run this after 02-scan.sh has already captured the after-upgrade
# findings.json -- rollback assess uses it as operational-readiness
# evidence via --findings, the same file the eventual compare step reads
# as `current`.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-case-study}"
KUBEPREFLIGHT_BIN="${KUBEPREFLIGHT_BIN:-kubepreflight}"

findings_path="${repo_root}/demo/eks-case-study/evidence/after-upgrade/findings.json"
out_dir="${repo_root}/demo/eks-case-study/evidence/rollback"
mkdir -p "${out_dir}"

if [[ ! -f "${findings_path}" ]]; then
  echo "error: ${findings_path} not found -- run 02-scan.sh after-upgrade first." >&2
  exit 1
fi

"${KUBEPREFLIGHT_BIN}" rollback assess \
  --cluster-name "${CLUSTER_NAME}" \
  --findings "${findings_path}" \
  --output all \
  --output-dir "${out_dir}" \
  --assessment-out "${out_dir}/rollback-assessment.json" \
  --terminal-output full

echo ""
echo "Rollback assessment written to ${out_dir}/"
