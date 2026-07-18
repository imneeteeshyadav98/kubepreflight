#!/usr/bin/env bash
# Seeds the "before" fixtures onto the already-created live-demo cluster.
# Reuses the exact, already-proven fixtures from demo/eks/manifests/ and
# demo/eks-case-study/manifests/ rather than inventing new ones -- these
# are the same manifests demo/eks/README.md and the real EKS case study
# both already validated trigger PDB-001/PDB-002/WH-001/WH-002/WORKLOAD-001
# reliably. Order matches scripts/case-study/01-seed-fixtures.sh: PDB pair
# first, then the unhealthy workload, then the fail-closed webhook last
# (its namespaceSelector depends on the namespace/label pdb-lab.yaml
# creates).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
eks_manifests="${repo_root}/demo/eks/manifests"
case_study_manifests="${repo_root}/demo/eks-case-study/manifests"

echo "Seeding PDB risk (PDB-001, PDB-002)..."
kubectl apply -f "${eks_manifests}/pdb-lab.yaml"
kubectl wait --for=condition=available deployment/critical-app -n preflight-lab --timeout=120s

echo "Seeding unhealthy workload (WORKLOAD-001)..."
kubectl apply -f "${case_study_manifests}/unhealthy-workload.yaml"

echo "Seeding admission webhook risk (WH-001, WH-002) -- applied last, see broken-webhook.yaml's own warning..."
kubectl apply -f "${eks_manifests}/broken-webhook.yaml"

echo ""
echo "Fixtures seeded."
