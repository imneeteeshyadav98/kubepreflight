#!/usr/bin/env bash
set -euo pipefail

LIVE_EKS_WORKDIR="${LIVE_EKS_WORKDIR:-live-eks-evidence}"
LIVE_EKS_RAW_DIR="${LIVE_EKS_RAW_DIR:-${LIVE_EKS_WORKDIR}/raw}"
LIVE_EKS_SANITIZED_DIR="${LIVE_EKS_SANITIZED_DIR:-${LIVE_EKS_WORKDIR}/sanitized}"
LIVE_EKS_RELEASE_DIR="${LIVE_EKS_RELEASE_DIR:-${LIVE_EKS_WORKDIR}/release}"
LIVE_EKS_COMMAND_LOG="${LIVE_EKS_COMMAND_LOG:-${LIVE_EKS_WORKDIR}/commands.tsv}"
GH_REPO="${GH_REPO:-imneeteeshyadav98/kubepreflight}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-ghcr.io/imneeteeshyadav98/kubepreflight}"

die() {
  echo "FAIL: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    die "${name} is required"
  fi
}

require_live_eks_identity_env() {
  require_env EXPECTED_AWS_ACCOUNT_ID
  require_env EXPECTED_AWS_REGION
  require_env EXPECTED_EKS_CLUSTER
  require_env EXPECTED_KUBE_CONTEXT
}

require_release_env() {
  require_env RELEASE_TAG
  require_env EXPECTED_RELEASE_COMMIT
  require_env EXPECTED_IMAGE_DIGEST
}

require_commit_matches() {
  local observed="$1"
  local source="$2"
  if [ -z "${observed}" ]; then
    die "${source} commit is empty"
  fi
  case "${EXPECTED_RELEASE_COMMIT}" in
    "${observed}"*) ;;
    *) die "${source} commit ${observed} is not a prefix of expected ${EXPECTED_RELEASE_COMMIT}" ;;
  esac
}

require_known_build_timestamp() {
  local path="$1"
  local source="$2"
  local built
  built="$(awk '/^built: / {print $2}' "${path}")"
  if [ -z "${built}" ]; then
    die "${source} build timestamp is empty"
  fi
  if [ "${built}" = "unknown" ]; then
    die "${source} build timestamp is unknown"
  fi
}

require_live_confirmation() {
  require_live_eks_identity_env
  require_release_env
  local expected="read-only-live-eks-smoke:${EXPECTED_AWS_ACCOUNT_ID}:${EXPECTED_AWS_REGION}:${EXPECTED_EKS_CLUSTER}:${EXPECTED_KUBE_CONTEXT}:${RELEASE_TAG}"
  if [ "${SEC_TRUST_LIVE_EKS_CONFIRM:-}" != "${expected}" ]; then
    cat >&2 <<EOF
SEC_TRUST_LIVE_EKS_CONFIRM must exactly match:

${expected}

This prevents accidental execution against the wrong account, region, cluster,
kube-context, or release tag. The harness is read-only, but it still queries a
real cluster and writes evidence to ${LIVE_EKS_WORKDIR}.
EOF
    exit 1
  fi
}

mkdirs() {
  mkdir -p "${LIVE_EKS_WORKDIR}" "${LIVE_EKS_RAW_DIR}" "${LIVE_EKS_SANITIZED_DIR}" "${LIVE_EKS_RELEASE_DIR}"
}

resolve_image_digest() {
  local ref="$1"
  local manifest_json
  if ! manifest_json="$(docker buildx imagetools inspect "${ref}" --format '{{json .Manifest}}' 2>&1)"; then
    echo "${manifest_json}" >&2
    return 1
  fi
  python3 -c "import json,sys; print(json.loads(sys.argv[1])['digest'])" "${manifest_json}"
}

log_command_result() {
  local name="$1"
  local code="$2"
  shift 2
  mkdirs
  printf '%s\t%s\t%s\n' "${name}" "${code}" "$*" >>"${LIVE_EKS_COMMAND_LOG}"
}

run_evidence_command() {
  local name="$1"
  local out_dir="$2"
  shift 2
  mkdir -p "${out_dir}"
  printf '%q ' "$@" >"${out_dir}/command.txt"
  printf '\n' >>"${out_dir}/command.txt"
  set +e
  "$@" >"${out_dir}/stdout.txt" 2>"${out_dir}/stderr.txt"
  local code=$?
  set -e
  echo "${code}" >"${out_dir}/exit-code.txt"
  log_command_result "${name}" "${code}" "$@"
  if [ "${code}" -eq 4 ]; then
    die "${name} returned internal/document error exit code 4; see ${out_dir}"
  fi
}

abs_path() {
  local path="$1"
  mkdir -p "${path}"
  (cd "${path}" && pwd)
}

require_nonempty_file() {
  local path="$1"
  [ -s "${path}" ] || die "missing or empty file: ${path}"
}

require_json_file() {
  local path="$1"
  require_nonempty_file "${path}"
  python3 -c "import json,sys; json.load(open(sys.argv[1]))" "${path}" || die "invalid JSON: ${path}"
}

require_markdown_file() {
  local path="$1"
  require_nonempty_file "${path}"
  grep -q '^# ' "${path}" || die "Markdown report missing top-level heading: ${path}"
}

require_html_file() {
  local path="$1"
  require_nonempty_file "${path}"
  grep -qi '<!doctype html>' "${path}" || die "HTML report missing doctype: ${path}"
}

require_terminal_capture() {
  local dir="$1"
  require_nonempty_file "${dir}/terminal/stdout.txt"
  require_nonempty_file "${dir}/terminal/exit-code.txt"
}

write_ephemeral_eks_kubeconfig() {
  require_live_eks_identity_env
  need_cmd aws
  need_cmd jq
  mkdirs
  local cluster_json="${LIVE_EKS_RAW_DIR}/eks-cluster.json"
  local token_json="${LIVE_EKS_RAW_DIR}/eks-token.json"
  local kubeconfig="${LIVE_EKS_RAW_DIR}/ephemeral-kubeconfig.yaml"

  aws eks describe-cluster \
    --region "${EXPECTED_AWS_REGION}" \
    --name "${EXPECTED_EKS_CLUSTER}" \
    >"${cluster_json}"
  aws eks get-token \
    --region "${EXPECTED_AWS_REGION}" \
    --cluster-name "${EXPECTED_EKS_CLUSTER}" \
    >"${token_json}"

  local endpoint ca token
  endpoint="$(jq -r '.cluster.endpoint' "${cluster_json}")"
  ca="$(jq -r '.cluster.certificateAuthority.data' "${cluster_json}")"
  token="$(jq -r '.status.token' "${token_json}")"
  [ -n "${endpoint}" ] && [ "${endpoint}" != "null" ] || die "cluster endpoint missing from describe-cluster"
  [ -n "${ca}" ] && [ "${ca}" != "null" ] || die "cluster certificateAuthority.data missing from describe-cluster"
  [ -n "${token}" ] && [ "${token}" != "null" ] || die "EKS token missing from get-token"

  umask 077
  cat >"${kubeconfig}" <<EOF
apiVersion: v1
kind: Config
clusters:
- name: ${EXPECTED_EKS_CLUSTER}
  cluster:
    server: ${endpoint}
    certificate-authority-data: ${ca}
contexts:
- name: ${EXPECTED_KUBE_CONTEXT}
  context:
    cluster: ${EXPECTED_EKS_CLUSTER}
    user: live-eks-smoke-token
current-context: ${EXPECTED_KUBE_CONTEXT}
users:
- name: live-eks-smoke-token
  user:
    token: ${token}
EOF
  # Absolute, not relative -- docker run -v requires an absolute host
  # path; a relative one is parsed as a named-volume name instead and
  # rejected for containing "/". KUBECONFIG=<path> for the direct binary
  # invocations tolerates either, so this only broke the container leg.
  echo "$(cd "${LIVE_EKS_RAW_DIR}" && pwd)/ephemeral-kubeconfig.yaml"
}

sensitive_pattern() {
  echo 'arn:aws:|(^|[^0-9])[0-9]{12}([^0-9]|$)|ip-[0-9]+-[0-9]+-[0-9]+-[0-9]+\.(ec2|[a-z0-9.-]*compute)\.internal'
}

forbidden_mutation_pattern() {
  echo '(^|[[:space:]])kubectl[[:space:]]+(apply|create|delete|patch|edit|label|annotate|replace|scale|cordon|uncordon|drain|taint)|aws[[:space:]]+eks[[:space:]]+(update-cluster-version|update-nodegroup-version|start-update)|kubepreflight[[:space:]]+rollback[[:space:]]+(execute|run|apply)'
}
