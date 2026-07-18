#!/usr/bin/env bash
# Redacts infrastructure identifiers from demo/live-eks/evidence/ in place,
# matching the exact redaction style already used for the committed
# EKS case-study evidence (demo/eks-case-study/evidence/, see git history
# "docs: redact EKS case study evidence identifiers"): findings, scores,
# decisions, and remediation text are never altered -- only the cluster
# ARN and node hostname strings are replaced with fixed placeholders.
#
# Run this after compare.sh and before verify-expected-output.sh (which
# checks that no unredacted identifier remains) or any screenshot/recording
# step.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
evidence_dir="${repo_root}/demo/live-eks/evidence"

cluster_arn="$(aws eks describe-cluster --name kubepreflight-live-demo --region us-east-1 --query 'cluster.arn' --output text 2>/dev/null || true)"
node_hostname="$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"

if [[ -z "${cluster_arn}" ]]; then
  echo "warning: could not resolve cluster ARN (cluster already deleted?) -- skipping ARN redaction" >&2
fi
if [[ -z "${node_hostname}" ]]; then
  echo "warning: could not resolve node hostname (cluster already deleted?) -- skipping hostname redaction" >&2
fi

count=0
while IFS= read -r -d '' file; do
  changed=0
  if [[ -n "${cluster_arn}" ]] && grep -qF "${cluster_arn}" "${file}"; then
    sed -i "s|${cluster_arn}|[redacted cluster ARN]|g" "${file}"
    changed=1
  fi
  if [[ -n "${node_hostname}" ]] && grep -qF "${node_hostname}" "${file}"; then
    sed -i "s|${node_hostname}|[redacted node hostname]|g" "${file}"
    changed=1
  fi
  if [[ "${changed}" -eq 1 ]]; then
    count=$((count + 1))
    echo "redacted: ${file#"${evidence_dir}"/}"
  fi
done < <(find "${evidence_dir}" -type f \( -name '*.json' -o -name '*.md' -o -name '*.html' \) -print0)

echo ""
echo "Redacted ${count} file(s)."
