#!/usr/bin/env bash
# Tears down everything the real EKS case study creates (see
# docs/case-studies/eks-1.31-to-1.32.md and this directory's README): the
# seeded webhook and namespaces, then the EKS cluster itself. Always run
# this when you're done -- the cluster is a real, billable AWS resource.
#
# Uses whatever AWS credentials/profile are already active in your shell
# (export AWS_PROFILE yourself beforehand if needed) -- mirrors
# demo/eks/cleanup.sh's own convention. Safe to re-run: every delete is
# --ignore-not-found or already idempotent.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-case-study}"
REGION="${REGION:-us-east-1}"

echo "Deleting seeded webhook and namespaces (ignored if already absent)..."
kubectl delete validatingwebhookconfiguration dead-fail-closed-webhook --ignore-not-found
kubectl delete ns preflight-lab --ignore-not-found
kubectl delete ns preflight-case-study --ignore-not-found

echo "Deleting EKS cluster '${CLUSTER_NAME}' in region '${REGION}' (this takes several minutes)..."
eksctl delete cluster --name "${CLUSTER_NAME}" --region "${REGION}" --wait

echo ""
echo "Verifying no clusters remain in ${REGION}:"
aws eks list-clusters --region "${REGION}"
