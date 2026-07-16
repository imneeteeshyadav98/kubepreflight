#!/usr/bin/env bash
# Applies every case-study fixture to the already-created cluster (see
# demo/eks-case-study/cluster.yaml and docs/case-studies/eks-1.31-to-1.32.md)
# in the order the fixtures' own comments require. Safe to re-run --
# every object here is idempotent to re-apply.
#
# demo/eks/manifests/old-api.yaml (API-001) is deliberately NOT applied by
# this script -- it's scan-only, picked up via `--manifests` in
# 02-scan.sh instead. A real 1.31+ control plane rejects it outright if
# you ever try (see that file's own warning comment).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
manifests_dir="${repo_root}/demo/eks/manifests"

echo "Seeding PDB risk (PDB-001, PDB-002)..."
kubectl apply -f "${manifests_dir}/pdb-lab.yaml"
kubectl wait --for=condition=available deployment/critical-app -n preflight-lab --timeout=120s

echo "Seeding unhealthy workload (WORKLOAD-001)..."
kubectl apply -f "${repo_root}/demo/eks-case-study/manifests/unhealthy-workload.yaml"

echo "Seeding admission webhook risk (WH-001, WH-002) -- applied last, see broken-webhook.yaml's own warning..."
kubectl apply -f "${manifests_dir}/broken-webhook.yaml"

echo ""
echo "Fixtures seeded."
