#!/usr/bin/env bash
# Tears down everything this demo creates, in the safe order: any
# LoadBalancer/Ingress-backed Services first (none exist in this demo's
# own fixtures, but this covers anything added ad hoc while recording --
# AWS's own guidance is to remove these before cluster deletion, since
# their backing ELBs otherwise outlive the cluster), then the seeded
# webhook/namespaces, then the cluster itself. Always run this
# immediately after recording -- the cluster is real and billable for
# every minute it stays up.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kubepreflight-live-demo}"
REGION="${REGION:-us-east-1}"

echo "Checking for any LoadBalancer-type Services (none expected from this demo's own fixtures)..."
kubectl get svc -A -o json 2>/dev/null | python3 -c "
import json, sys
data = json.load(sys.stdin)
lbs = [(i['metadata']['namespace'], i['metadata']['name']) for i in data.get('items', []) if i.get('spec', {}).get('type') == 'LoadBalancer']
for ns, name in lbs:
    print(f'{ns}/{name}')
" | while read -r svc; do
  ns="${svc%%/*}"
  name="${svc##*/}"
  echo "Deleting LoadBalancer Service ${ns}/${name} before cluster deletion..."
  kubectl delete svc "${name}" -n "${ns}" --ignore-not-found
done

echo "Deleting demo webhook, Service, and namespaces (ignored if already absent)..."
kubectl delete validatingwebhookconfiguration dead-fail-closed-webhook --ignore-not-found
kubectl delete ns preflight-lab --ignore-not-found
kubectl delete ns preflight-case-study --ignore-not-found

echo "Deleting EKS cluster '${CLUSTER_NAME}' in region '${REGION}' (this takes several minutes)..."
eksctl delete cluster --name "${CLUSTER_NAME}" --region "${REGION}" --wait

echo ""
echo "Verifying the cluster is gone (expect ResourceNotFoundException):"
if aws eks describe-cluster --name "${CLUSTER_NAME}" --region "${REGION}" >/dev/null 2>&1; then
  echo "error: ${CLUSTER_NAME} still exists after delete --wait returned. Investigate before walking away." >&2
  exit 1
fi
echo "Confirmed: ${CLUSTER_NAME} no longer exists."

echo ""
echo "Remaining clusters in ${REGION} (should be empty unless you have unrelated ones):"
aws eks list-clusters --region "${REGION}"

echo ""
echo "Checking for leftover CloudFormation stacks..."
leftover_stacks="$(aws cloudformation list-stacks --region "${REGION}" \
  --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE DELETE_FAILED \
  --query "StackSummaries[?contains(StackName, '${CLUSTER_NAME}')].StackName" \
  --output text)"
if [[ -n "${leftover_stacks}" ]]; then
  echo "error: leftover CloudFormation stack(s) still exist -- teardown is incomplete:" >&2
  echo "${leftover_stacks}" >&2
  exit 1
fi
echo "Confirmed: no leftover CloudFormation stacks for ${CLUSTER_NAME}."
