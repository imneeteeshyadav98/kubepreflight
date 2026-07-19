#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/live-eks/lib.sh
source "${script_dir}/lib.sh"

need_cmd aws
need_cmd jq
need_cmd kubectl
need_cmd python3
require_live_confirmation
mkdirs

account_json="${LIVE_EKS_RAW_DIR}/aws-account.json"
cluster_json="${LIVE_EKS_RAW_DIR}/eks-cluster.json"
kube_context_file="${LIVE_EKS_RAW_DIR}/kube-context.txt"
kubectl_auth_file="${LIVE_EKS_RAW_DIR}/kubectl-auth-can-i-list.txt"
kubectl_version_file="${LIVE_EKS_RAW_DIR}/kubectl-version.txt"

aws sts get-caller-identity >"${account_json}"
actual_account="$(jq -r '.Account' "${account_json}")"
[ "${actual_account}" = "${EXPECTED_AWS_ACCOUNT_ID}" ] || die "AWS account ${actual_account} != expected ${EXPECTED_AWS_ACCOUNT_ID}"

aws eks describe-cluster \
  --region "${EXPECTED_AWS_REGION}" \
  --name "${EXPECTED_EKS_CLUSTER}" \
  >"${cluster_json}"

actual_cluster="$(jq -r '.cluster.name' "${cluster_json}")"
actual_region="$(jq -r '.cluster.arn | split(":")[3]' "${cluster_json}")"
actual_cluster_account="$(jq -r '.cluster.arn | split(":")[4]' "${cluster_json}")"
actual_status="$(jq -r '.cluster.status' "${cluster_json}")"
[ "${actual_cluster}" = "${EXPECTED_EKS_CLUSTER}" ] || die "EKS cluster ${actual_cluster} != expected ${EXPECTED_EKS_CLUSTER}"
[ "${actual_region}" = "${EXPECTED_AWS_REGION}" ] || die "EKS region ${actual_region} != expected ${EXPECTED_AWS_REGION}"
[ "${actual_cluster_account}" = "${EXPECTED_AWS_ACCOUNT_ID}" ] || die "EKS cluster account ${actual_cluster_account} != expected ${EXPECTED_AWS_ACCOUNT_ID}"
[ "${actual_status}" = "ACTIVE" ] || die "EKS cluster status ${actual_status} is not ACTIVE"

kubectl config current-context >"${kube_context_file}"
actual_context="$(tr -d '\n' <"${kube_context_file}")"
[ "${actual_context}" = "${EXPECTED_KUBE_CONTEXT}" ] || die "kube-context ${actual_context} != expected ${EXPECTED_KUBE_CONTEXT}"

kubectl auth can-i --list >"${kubectl_auth_file}"
kubectl version --short >"${kubectl_version_file}" 2>&1 || kubectl version >"${kubectl_version_file}" 2>&1

"${script_dir}/verify-read-only.sh"

cat >"${LIVE_EKS_WORKDIR}/preflight-summary.md" <<EOF
# Live EKS Smoke Preflight

- AWS account: ${EXPECTED_AWS_ACCOUNT_ID}
- AWS region: ${EXPECTED_AWS_REGION}
- EKS cluster: ${EXPECTED_EKS_CLUSTER}
- kube-context: ${EXPECTED_KUBE_CONTEXT}
- release tag: ${RELEASE_TAG}
- expected release commit: ${EXPECTED_RELEASE_COMMIT}
- expected image digest: ${EXPECTED_IMAGE_DIGEST}
- read-only command inventory: passed
EOF

echo "OK: live EKS preflight passed; raw evidence is in ${LIVE_EKS_RAW_DIR}"
