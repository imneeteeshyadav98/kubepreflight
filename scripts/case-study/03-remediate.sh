#!/usr/bin/env bash
# Remediates the seeded PDB risk and the fail-closed webhook -- the two
# risks that would actually block safe node eviction/upgrade -- leaving
# the pre-existing unhealthy workload (WORKLOAD-001) and the scan-only
# deprecated-API manifest (API-001) untouched. See
# docs/case-studies/eks-1.31-to-1.32.md section 4 for the real narrative
# once this has actually been run.
set -euo pipefail

echo "Scaling critical-app to 2 replicas so its PDB (minAvailable: 1) allows a voluntary disruption (fixes PDB-001)..."
kubectl scale deployment critical-app -n preflight-lab --replicas=2

echo "Removing the overlapping PDB (fixes PDB-002)..."
kubectl delete pdb critical-app-pdb-overlap -n preflight-lab --ignore-not-found

echo "Removing the fail-closed webhook and its backend Service (fixes WH-001, WH-002)..."
kubectl delete validatingwebhookconfiguration dead-fail-closed-webhook --ignore-not-found
kubectl delete service dead-webhook -n preflight-lab --ignore-not-found

kubectl wait --for=condition=available deployment/critical-app -n preflight-lab --timeout=120s

echo ""
echo "Remediation applied. WORKLOAD-001 (already-broken-app) and API-001 (scan-only old-api.yaml) are left as-is deliberately."
