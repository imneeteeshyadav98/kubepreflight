#!/usr/bin/env bash
# Tears down everything the EKS demo (see README.md) creates: the seeded
# webhook/namespace, then the EKS cluster itself. Always run this after
# you're done testing — the cluster is a real, billable AWS resource.
#
# Uses whatever AWS credentials/profile are already active in your shell
# (export AWS_PROFILE yourself beforehand if needed) — this script never
# sets or assumes a specific profile.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-demo}"
REGION="${REGION:-us-east-1}"

echo "Deleting demo webhook and namespace (ignored if already absent)..."
kubectl delete validatingwebhookconfiguration dead-fail-closed-webhook --ignore-not-found
kubectl delete ns preflight-lab --ignore-not-found

echo "Deleting EKS cluster '${CLUSTER_NAME}' in region '${REGION}' (this takes several minutes)..."
eksctl delete cluster --name "${CLUSTER_NAME}" --region "${REGION}" --wait

echo ""
echo "Verifying no clusters remain in ${REGION}:"
aws eks list-clusters --region "${REGION}"
