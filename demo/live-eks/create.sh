#!/usr/bin/env bash
# Creates the live-demo EKS cluster (cluster.yaml) with a dynamic
# ExpiresAt tag on top of the file's static tags, so a leftover cluster is
# identifiable/alarmable on even without reading this repo. Takes roughly
# 15-20 minutes (control plane + one managed node group), same as every
# other eksctl-based demo in this repo.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

EXPIRES_AT="$(date -u -d '+6 hours' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v+6H +%Y-%m-%dT%H:%M:%SZ)"

echo "Confirming target AWS account before creating anything billable..."
aws sts get-caller-identity
echo ""
echo "Confirming no cluster named kubepreflight-live-demo already exists..."
if aws eks describe-cluster --name kubepreflight-live-demo --region us-east-1 >/dev/null 2>&1; then
  echo "error: kubepreflight-live-demo already exists in this account/region -- delete it first, don't reuse." >&2
  exit 1
fi

echo ""
echo "This creates a real, billable EKS cluster (control plane + one t3.small"
echo "node) in the AWS account shown above. Run destroy.sh as soon as you're"
echo "done -- it is not optional."
if [[ "${CONFIRM:-}" != "yes" ]]; then
  read -r -p "Type 'yes' to create it: " reply
  if [[ "${reply}" != "yes" ]]; then
    echo "Aborted -- nothing was created." >&2
    exit 1
  fi
fi

echo ""
echo "Creating cluster..."
eksctl create cluster -f "${repo_root}/demo/live-eks/cluster.yaml"

echo ""
echo "Tagging cluster with ExpiresAt=${EXPIRES_AT} (eksctl's --tags can't combine with -f/--config-file, so this is applied as a separate step after creation)..."
cluster_arn="$(aws eks describe-cluster --name kubepreflight-live-demo --region us-east-1 --query 'cluster.arn' --output text)"
aws eks tag-resource --resource-arn "${cluster_arn}" --tags "ExpiresAt=${EXPIRES_AT}" --region us-east-1

echo ""
echo "Cluster created. kubeconfig updated automatically by eksctl."
