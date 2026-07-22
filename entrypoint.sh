#!/usr/bin/env bash
# Entrypoint for the KubePreflight composite GitHub Action (action.yml).
#
# Runs the exact released kubepreflight Docker image via `docker run`
# rather than rebuilding from source, so the action stays fast and always
# matches whatever version the caller pinned in `uses: .../kubepreflight@X`.
# The image tag is derived from github.action_ref (passed in as ACTION_REF)
# by stripping the leading "v" -- see docs/ci-integration.md for why the
# release workflow's docker/metadata-action step publishes tags without it.
set -euo pipefail

: "${INPUT_TARGET_VERSION:?target-version input is required}"
: "${GITHUB_WORKSPACE:?GITHUB_WORKSPACE is not set}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is not set}"
: "${GITHUB_STEP_SUMMARY:?GITHUB_STEP_SUMMARY is not set}"

action_ref="${ACTION_REF:-}"
image_tag="${action_ref#v}"
if [[ -z "${image_tag}" ]]; then
  echo "::error::Could not resolve a KubePreflight image tag from github.action_ref (got '${action_ref}'). Pin this action to a released tag, e.g. imneeteeshyadav98/kubepreflight@v0.4.1-ci-integration, not a branch or SHA." >&2
  exit 1
fi
image="ghcr.io/imneeteeshyadav98/kubepreflight:${image_tag}"

findings_out="${INPUT_FINDINGS_OUT:-findings.json}"
report_out="${INPUT_REPORT_OUT:-.}"

scan_args=(scan --target-version "${INPUT_TARGET_VERSION}" --upgrade-context "${INPUT_UPGRADE_CONTEXT:-unspecified}" --output all \
  --findings-out "/work/${findings_out}" --output-dir "/work/${report_out}" \
  --serve-report never --terminal-output compact)

# The image runs as a fixed nonroot UID (see Dockerfile) that won't match
# whatever UID owns GITHUB_WORKSPACE on the runner, so a plain bind mount
# write (findings.json/report.html) fails with permission denied -- the
# same problem docker-compose.yml already solves for local dev via a
# user: override. Match the invoking user here for the same reason.
docker_args=(run --rm --user "$(id -u):$(id -g)" -v "${GITHUB_WORKSPACE}:/work" -w /work)

if [[ -n "${INPUT_MANIFESTS:-}" ]]; then
  scan_args+=(--manifests "${INPUT_MANIFESTS}")
fi
if [[ "${INPUT_MANIFESTS_ONLY:-false}" == "true" ]]; then
  scan_args+=(--manifests-only)
fi
if [[ -n "${INPUT_PROVIDER:-}" ]]; then
  scan_args+=(--provider "${INPUT_PROVIDER}")
fi
if [[ -n "${INPUT_CLUSTER_NAME:-}" ]]; then
  scan_args+=(--cluster-name "${INPUT_CLUSTER_NAME}")
fi
if [[ -n "${INPUT_REGION:-}" ]]; then
  # Not a kubepreflight CLI flag -- AWS enrichment always resolves its
  # region through the standard AWS SDK credential/region chain.
  docker_args+=(-e "AWS_REGION=${INPUT_REGION}")
fi
if [[ -n "${INPUT_KUBECONFIG:-}" ]]; then
  if [[ ! -f "${INPUT_KUBECONFIG}" ]]; then
    echo "::error::kubeconfig input '${INPUT_KUBECONFIG}' does not exist on the runner." >&2
    exit 1
  fi
  docker_args+=(-v "${INPUT_KUBECONFIG}:/work/.kubeconfig-mounted:ro")
  scan_args+=(--kubeconfig /work/.kubeconfig-mounted)
fi

# Forward standard AWS SDK env vars when the caller already resolved them
# in an earlier step (e.g. aws-actions/configure-aws-credentials). Only
# forwarded when actually set in this shell -- `docker run -e VAR` with an
# unset VAR passes an empty override into the container instead of leaving
# it unset there.
for var in AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE AWS_DEFAULT_REGION; do
  if [[ -n "${!var:-}" ]]; then
    docker_args+=(-e "${var}")
  fi
done

echo "::group::kubepreflight scan"
set +e
docker "${docker_args[@]}" "${image}" "${scan_args[@]}"
scan_exit=$?
set -e
echo "::endgroup::"

findings_path="${GITHUB_WORKSPACE}/${findings_out}"
if [[ "${report_out}" == "." ]]; then
  report_path="${GITHUB_WORKSPACE}/report.html"
else
  report_path="${GITHUB_WORKSPACE}/${report_out}/report.html"
fi

# Exit code 4 (and any other pre-report failure, e.g. a bad flag
# combination) means no findings.json was ever written -- checking for the
# file directly is more reliable than trusting the numeric exit code alone,
# since kubepreflight's own contract documents that ordinary usage errors
# also exit 1, the same code as "PASSED_WITH_WARNINGS".
if [[ ! -f "${findings_path}" ]]; then
  echo "::error::KubePreflight did not produce a findings report (container exit code ${scan_exit}) -- treating this as an infrastructure failure, not a scan result. Check the scan log above for the real cause (bad kubeconfig, unreachable cluster, invalid inputs)." >&2
  {
    echo "## KubePreflight: INFRA_FAILURE"
    echo ""
    echo "No report was produced (container exit code \`${scan_exit}\`). See the job log above for the actual error."
  } >>"${GITHUB_STEP_SUMMARY}"
  echo "verdict=INFRA_FAILURE" >>"${GITHUB_OUTPUT}"
  exit 1
fi

# .upgradeReadiness.verdict is Report.Result() verbatim (see
# internal/findings/report.go) -- the same authoritative string that
# drives kubepreflight's own exit code, so this never has to duplicate or
# guess at that mapping.
verdict=$(jq -r '.upgradeReadiness.verdict' "${findings_path}")
score=$(jq -r '.upgradeReadiness.readinessScore' "${findings_path}")
can_continue=$(jq -r '.upgradeReadiness.upgradeContinue' "${findings_path}")
blockers=$(jq -r '.summary.blockers' "${findings_path}")
warnings=$(jq -r '.summary.warnings' "${findings_path}")
operator_decisions=$(jq -r '.summary.operatorDecisions // 0' "${findings_path}")

{
  echo "verdict=${verdict}"
  echo "blockers=${blockers}"
  echo "warnings=${warnings}"
  echo "operator-decisions=${operator_decisions}"
  echo "readiness-score=${score}"
  echo "can-upgrade-continue=${can_continue}"
  echo "findings-file=${findings_path}"
} >>"${GITHUB_OUTPUT}"

if [[ -f "${report_path}" ]]; then
  echo "report-file=${report_path}" >>"${GITHUB_OUTPUT}"
fi

{
  echo "## KubePreflight — Upgrade Readiness: ${verdict}"
  echo ""
  echo "| | |"
  echo "|---|---|"
  echo "| **Readiness score** | ${score}/100 |"
  echo "| **Upgrade continue** | ${can_continue} |"
  echo "| **Blockers** | ${blockers} |"
  echo "| **Warnings** | ${warnings} |"
  echo "| **Operator decisions** | ${operator_decisions} |"
  echo ""
  echo "| Category | Status | Blockers | Warnings |"
  echo "|---|---|---|---|"
  jq -r '.upgradeReadiness.categories[] | "| \(.name) | \(.status) | \(.blockerCount) | \(.warningCount) |"' "${findings_path}"
} >>"${GITHUB_STEP_SUMMARY}"

fail_on_warning="${INPUT_FAIL_ON_WARNING:-false}"

case "${verdict}" in
CLEAN)
  exit 0
  ;;
PASSED_WITH_WARNINGS)
  if [[ "${fail_on_warning}" == "true" ]]; then
    echo "::error::KubePreflight found Warning-severity findings or operator decisions and fail-on-warning is true." >&2
    exit 1
  fi
  exit 0
  ;;
BLOCKED)
  echo "::error::KubePreflight found ${blockers} Blocker finding(s) -- upgrade should not proceed." >&2
  exit 1
  ;;
INCOMPLETE)
  echo "::error::KubePreflight's scan coverage was incomplete -- treating this as a failure, not a pass." >&2
  exit 1
  ;;
*)
  echo "::error::Unrecognized KubePreflight verdict '${verdict}'." >&2
  exit 1
  ;;
esac
