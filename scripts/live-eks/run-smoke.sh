#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/live-eks/lib.sh
source "${script_dir}/lib.sh"

need_cmd aws
need_cmd docker
need_cmd jq
need_cmd kubectl
need_cmd python3
require_live_confirmation
require_env TARGET_VERSION
require_env PLAN_TO_VERSION
mkdirs

release_bin="${RELEASE_BINARY:-}"
if [ -z "${release_bin}" ] && [ -s "${LIVE_EKS_RELEASE_DIR}/binary.path" ]; then
  release_bin="$(cat "${LIVE_EKS_RELEASE_DIR}/binary.path")"
fi
[ -x "${release_bin}" ] || die "release binary not executable; run download-release.sh first or set RELEASE_BINARY"

release_image="${RELEASE_IMAGE:-}"
if [ -z "${release_image}" ] && [ -s "${LIVE_EKS_RELEASE_DIR}/image.ref" ]; then
  release_image="$(cat "${LIVE_EKS_RELEASE_DIR}/image.ref")"
fi
[ -n "${release_image}" ] || die "release image missing; run download-release.sh first or set RELEASE_IMAGE"

kubeconfig="$(write_ephemeral_eks_kubeconfig)"
chmod 0644 "${kubeconfig}"

scan_bin_dir="${LIVE_EKS_WORKDIR}/binary/scan"
plan_bin_dir="${LIVE_EKS_WORKDIR}/binary/plan"
rollback_plan_bin_dir="${LIVE_EKS_WORKDIR}/binary/rollback-plan"
rollback_assess_bin_dir="${LIVE_EKS_WORKDIR}/binary/rollback-assess"
scan_container_dir="${LIVE_EKS_WORKDIR}/container/scan"
compare_dir="${LIVE_EKS_WORKDIR}/compare"

mkdir -p "${scan_bin_dir}" "${plan_bin_dir}" "${rollback_plan_bin_dir}" "${rollback_assess_bin_dir}" "${scan_container_dir}" "${compare_dir}"
scan_container_abs="$(abs_path "${scan_container_dir}")"
# 65532 (the release image's fixed nonroot UID) has no write access to a
# host output dir it doesn't own, so container-scan failed writing
# findings.json until --user matched it to the uid:gid that actually
# owns every bind-mounted path here -- the same uid the binary already
# runs as natively.
docker_user=(--user "$(id -u):$(id -g)")
docker_env=(
  -e "AWS_REGION=${EXPECTED_AWS_REGION}"
  -e "AWS_DEFAULT_REGION=${EXPECTED_AWS_REGION}"
)
# aws-sdk-go-v2's shared config/credentials file loading needs a
# passwd-resolvable identity for the container's --user override, which
# a CGO_ENABLED=0 static build inside a shell-less distroless image (no
# /etc/passwd entry for an arbitrary host UID) can't provide. Confirmed
# empirically: the exact same ~/.aws/credentials file, verified readable
# inside an equivalent container, still produced "no AWS credentials
# found" via aws-sdk-go-v2's LoadDefaultConfig, while the identical
# resolved credentials passed as plain env vars worked immediately.
# `aws configure export-credentials` resolves whatever the host's
# active credential source actually is (static keys, SSO, an assumed
# role, ...) into plain env vars, bypassing in-container file-based
# loading entirely. If the host has no credentials configured at all,
# this cleanly no-ops and container-scan runs cluster-only, same as the
# binary would.
#
# Values are exported into this script's own environment and passed to
# docker as bare `-e KEY` (no `=value`) so docker resolves each value
# from the environment at invocation time -- run_evidence_command logs
# "$@" verbatim to command.txt, which lands in live-eks-evidence/container
# (not raw/ or release/, the only two directories sanitize-evidence.sh
# skips), so an embedded `-e KEY=<secret>` would otherwise put real AWS
# credentials in evidence its own redaction patterns don't know about.
if aws_creds="$(aws configure export-credentials --format env-no-export 2>/dev/null)"; then
  while IFS='=' read -r key value; do
    [ -n "${key}" ] || continue
    export "${key}=${value}"
    docker_env+=(-e "${key}")
  done <<<"${aws_creds}"
fi

run_evidence_command "binary-scan" "${scan_bin_dir}/terminal" \
  env KUBECONFIG="${kubeconfig}" AWS_REGION="${EXPECTED_AWS_REGION}" AWS_DEFAULT_REGION="${EXPECTED_AWS_REGION}" \
  "${release_bin}" scan \
  --provider eks \
  --cluster-name "${EXPECTED_EKS_CLUSTER}" \
  --target-version "${TARGET_VERSION}" \
  --context "${EXPECTED_KUBE_CONTEXT}" \
  --output all \
  --output-dir "${scan_bin_dir}" \
  --findings-out "${scan_bin_dir}/findings.json" \
  --redact-sensitive-identifiers \
  --serve-report never \
  --terminal-output full
require_terminal_capture "${scan_bin_dir}"
require_json_file "${scan_bin_dir}/findings.json"
require_markdown_file "${scan_bin_dir}/report.md"
require_html_file "${scan_bin_dir}/report.html"

run_evidence_command "binary-plan" "${plan_bin_dir}/terminal" \
  env KUBECONFIG="${kubeconfig}" AWS_REGION="${EXPECTED_AWS_REGION}" AWS_DEFAULT_REGION="${EXPECTED_AWS_REGION}" \
  "${release_bin}" plan \
  --provider eks \
  --cluster-name "${EXPECTED_EKS_CLUSTER}" \
  --to-version "${PLAN_TO_VERSION}" \
  --context "${EXPECTED_KUBE_CONTEXT}" \
  --output all \
  --output-dir "${plan_bin_dir}" \
  --findings-out "${plan_bin_dir}/findings.json" \
  --redact-sensitive-identifiers \
  --serve-report never \
  --terminal-output full
require_terminal_capture "${plan_bin_dir}"
require_json_file "${plan_bin_dir}/findings.json"
require_json_file "${plan_bin_dir}/upgrade-plan.json"
require_markdown_file "${plan_bin_dir}/report.md"
require_html_file "${plan_bin_dir}/report.html"

run_evidence_command "binary-rollback-plan" "${rollback_plan_bin_dir}/terminal" \
  env KUBECONFIG="${kubeconfig}" AWS_REGION="${EXPECTED_AWS_REGION}" AWS_DEFAULT_REGION="${EXPECTED_AWS_REGION}" \
  "${release_bin}" rollback plan \
  --provider eks \
  --cluster-name "${EXPECTED_EKS_CLUSTER}" \
  --findings "${scan_bin_dir}/findings.json" \
  --output all \
  --output-dir "${rollback_plan_bin_dir}" \
  --assessment-out "${rollback_plan_bin_dir}/rollback-assessment.json" \
  --redact-sensitive-identifiers \
  --terminal-output full
require_terminal_capture "${rollback_plan_bin_dir}"
require_json_file "${rollback_plan_bin_dir}/rollback-assessment.json"
require_markdown_file "${rollback_plan_bin_dir}/rollback-report.md"
require_html_file "${rollback_plan_bin_dir}/rollback-report.html"

run_evidence_command "binary-rollback-assess" "${rollback_assess_bin_dir}/terminal" \
  env KUBECONFIG="${kubeconfig}" AWS_REGION="${EXPECTED_AWS_REGION}" AWS_DEFAULT_REGION="${EXPECTED_AWS_REGION}" \
  "${release_bin}" rollback assess \
  --provider eks \
  --cluster-name "${EXPECTED_EKS_CLUSTER}" \
  --findings "${scan_bin_dir}/findings.json" \
  --output all \
  --output-dir "${rollback_assess_bin_dir}" \
  --assessment-out "${rollback_assess_bin_dir}/rollback-assessment.json" \
  --redact-sensitive-identifiers \
  --terminal-output full
require_terminal_capture "${rollback_assess_bin_dir}"
require_json_file "${rollback_assess_bin_dir}/rollback-assessment.json"
require_markdown_file "${rollback_assess_bin_dir}/rollback-report.md"
require_html_file "${rollback_assess_bin_dir}/rollback-report.html"

run_evidence_command "container-scan" "${scan_container_dir}/terminal" \
  docker run --rm \
  --network host \
  "${docker_user[@]}" \
  "${docker_env[@]}" \
  -v "${kubeconfig}:/work/kubeconfig:ro" \
  -v "${scan_container_abs}:/work/out" \
  "${release_image}" scan \
  --provider eks \
  --cluster-name "${EXPECTED_EKS_CLUSTER}" \
  --target-version "${TARGET_VERSION}" \
  --context "${EXPECTED_KUBE_CONTEXT}" \
  --kubeconfig /work/kubeconfig \
  --output all \
  --output-dir /work/out \
  --findings-out /work/out/findings.json \
  --redact-sensitive-identifiers \
  --serve-report never \
  --terminal-output full
require_terminal_capture "${scan_container_dir}"
require_json_file "${scan_container_dir}/findings.json"
require_markdown_file "${scan_container_dir}/report.md"
require_html_file "${scan_container_dir}/report.html"

run_evidence_command "binary-container-compare" "${compare_dir}/terminal" \
  "${release_bin}" compare \
  --baseline "${scan_bin_dir}/findings.json" \
  --current "${scan_container_dir}/findings.json" \
  --json-out "${compare_dir}/comparison.json" \
  --markdown-out "${compare_dir}/comparison.md" \
  --gate-out "${compare_dir}/gate.json" \
  --redact-sensitive-identifiers
require_terminal_capture "${compare_dir}"
require_json_file "${compare_dir}/comparison.json"
require_json_file "${compare_dir}/gate.json"
require_markdown_file "${compare_dir}/comparison.md"

# findings.json has no "tool" field -- version/commit come from the
# version banners download-release.sh already captured. (A prior version
# of this check read left["tool"]["version"] / ["commit"], fields that
# don't exist in the findings schema, so binary_version/container_version
# /binary_commit/container_commit were always silently null here -- this
# parity-summary.json claimed to prove "same version, commit" per
# docs/live-eks-release-smoke.md but never actually checked it.)
python3 - "${scan_bin_dir}/findings.json" "${scan_container_dir}/findings.json" "${compare_dir}/parity-summary.json" "${LIVE_EKS_RELEASE_DIR}/binary-version.txt" "${LIVE_EKS_RELEASE_DIR}/container-version.txt" <<'PY'
import json
import re
import sys

left = json.load(open(sys.argv[1]))
right = json.load(open(sys.argv[2]))

def parse_version_banner(path):
    text = open(path).read()
    lines = text.splitlines()
    version = lines[0][len("KubePreflight "):].strip() if lines else None
    commit_match = re.search(r"^commit: (\S+)$", text, re.MULTILINE)
    return version, commit_match.group(1) if commit_match else None

binary_version, binary_commit = parse_version_banner(sys.argv[4])
container_version, container_commit = parse_version_banner(sys.argv[5])

def finding_key(f):
    return (
        f.get("id"),
        f.get("fingerprint"),
        f.get("severity"),
        f.get("resource", {}).get("kind"),
        f.get("resource", {}).get("namespace"),
        f.get("resource", {}).get("name"),
    )

summary = {
    "binary_version": binary_version,
    "container_version": container_version,
    "binary_commit": binary_commit,
    "container_commit": container_commit,
    "binary_verdict": left.get("upgradeReadiness", {}).get("verdict"),
    "container_verdict": right.get("upgradeReadiness", {}).get("verdict"),
    "binary_findings": len(left.get("findings", [])),
    "container_findings": len(right.get("findings", [])),
    "same_finding_keys": sorted(map(finding_key, left.get("findings", []))) == sorted(map(finding_key, right.get("findings", []))),
}
json.dump(summary, open(sys.argv[3], "w"), indent=2, sort_keys=True)
print(json.dumps(summary, indent=2, sort_keys=True))
if not summary["same_finding_keys"]:
    raise SystemExit("binary/container finding keys differ")
if summary["binary_verdict"] != summary["container_verdict"]:
    raise SystemExit("binary/container verdicts differ")
if binary_version != container_version:
    raise SystemExit("binary/container versions differ")
if binary_commit != container_commit:
    raise SystemExit("binary/container commits differ")
PY

"${script_dir}/check-redaction.sh" \
  "${scan_bin_dir}/terminal/stdout.txt" \
  "${scan_bin_dir}/findings.json" \
  "${scan_bin_dir}/report.md" \
  "${scan_bin_dir}/report.html" \
  "${plan_bin_dir}/terminal/stdout.txt" \
  "${plan_bin_dir}/findings.json" \
  "${plan_bin_dir}/upgrade-plan.json" \
  "${plan_bin_dir}/report.md" \
  "${plan_bin_dir}/report.html" \
  "${rollback_plan_bin_dir}/terminal/stdout.txt" \
  "${rollback_plan_bin_dir}/rollback-assessment.json" \
  "${rollback_plan_bin_dir}/rollback-report.md" \
  "${rollback_plan_bin_dir}/rollback-report.html" \
  "${rollback_assess_bin_dir}/terminal/stdout.txt" \
  "${rollback_assess_bin_dir}/rollback-assessment.json" \
  "${rollback_assess_bin_dir}/rollback-report.md" \
  "${rollback_assess_bin_dir}/rollback-report.html" \
  "${scan_container_dir}/terminal/stdout.txt" \
  "${scan_container_dir}/findings.json" \
  "${scan_container_dir}/report.md" \
  "${scan_container_dir}/report.html" \
  "${compare_dir}/terminal/stdout.txt" \
  "${compare_dir}/comparison.json" \
  "${compare_dir}/comparison.md" \
  "${compare_dir}/gate.json"
"${script_dir}/verify-read-only.sh"
"${script_dir}/sanitize-evidence.sh"

echo "OK: live EKS released-artifact smoke completed"
echo "sanitized evidence: ${LIVE_EKS_SANITIZED_DIR}"
