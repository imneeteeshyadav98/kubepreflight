#!/usr/bin/env bash
# Remediates the seeded PDB risk and the fail-closed webhook -- the two
# risks that would actually block safe node eviction/upgrade -- leaving
# the pre-existing unhealthy workload (WORKLOAD-001) and the scan-only
# deprecated-API manifest (API-001) untouched. See
# docs/case-studies/eks-1.31-to-1.32.md section 4 for the real narrative
# once this has actually been run.
set -euo pipefail

# Webhook removal MUST come first. broken-webhook.yaml's rules match
# resources: ["*/*"] with operations CREATE/UPDATE in the preflight-lab
# namespace -- a `kubectl scale` (an UPDATE) below is rejected outright
# while the webhook is still live (confirmed running this case study for
# real: "no endpoints available for service \"dead-webhook\""). DELETE
# isn't in the webhook's operations list, so removing the webhook itself
# is unaffected and safe to do while it's still active.
echo "Removing the fail-closed webhook and its backend Service (fixes WH-001, WH-002)..."
kubectl delete validatingwebhookconfiguration dead-fail-closed-webhook --ignore-not-found
kubectl delete service dead-webhook -n preflight-lab --ignore-not-found

echo "Scaling critical-app to 2 replicas so its PDB (minAvailable: 1) allows a voluntary disruption (fixes PDB-001)..."
kubectl scale deployment critical-app -n preflight-lab --replicas=2

echo "Removing the overlapping PDB (fixes PDB-002)..."
kubectl delete pdb critical-app-pdb-overlap -n preflight-lab --ignore-not-found

kubectl wait --for=condition=available deployment/critical-app -n preflight-lab --timeout=120s

echo ""
echo "Remediation applied. WORKLOAD-001 (already-broken-app) and API-001 (scan-only old-api.yaml) are left as-is deliberately."
