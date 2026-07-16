#!/usr/bin/env bash
# Upgrades the case-study cluster's control plane for real, one minor
# version at a time as EKS requires (1.31 -> 1.32, matching
# demo/eks-case-study/cluster.yaml's starting version). Takes several
# minutes; eksctl blocks until it's done, then this refreshes kubeconfig
# so subsequent scripts talk to the upgraded control plane.
#
# This upgrades the control plane only -- it does not touch the managed
# node group's AMI/version. Real EKS control-plane upgrades don't drain
# worker nodes by themselves, so this step alone won't exercise the PDB
# fixtures' eviction-blocking behavior; that's a node-group-upgrade
# concern, out of scope for this case study (see the design doc's
# boundary section).
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-case-study}"
REGION="${REGION:-us-east-1}"

eksctl upgrade cluster --name "${CLUSTER_NAME}" --region "${REGION}" --approve
aws eks update-kubeconfig --name "${CLUSTER_NAME}" --region "${REGION}"

echo ""
echo "Control plane upgraded. Verify:"
echo "  aws eks describe-cluster --name ${CLUSTER_NAME} --region ${REGION} --query 'cluster.version'"
