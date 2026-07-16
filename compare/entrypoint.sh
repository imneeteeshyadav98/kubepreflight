#!/usr/bin/env bash
# Entrypoint for the KubePreflight Compare composite GitHub Action
# (compare/action.yml).
#
# Runs the exact released kubepreflight Docker image's `compare` subcommand
# rather than rebuilding from source, so the action stays fast and always
# matches whatever version the caller pinned in
# `uses: .../kubepreflight/compare@X` -- same image-resolution approach as
# the top-level scan action's entrypoint.sh, see that file's own comment
# for why the tag is derived from github.action_ref this way.
#
# This action never touches a cluster or AWS -- baseline and current are
# both already-produced findings.json files on the runner, so no
# kubeconfig/credential wiring exists here at all.
set -euo pipefail

: "${INPUT_BASELINE:?baseline input is required}"
: "${INPUT_CURRENT:?current input is required}"
: "${GITHUB_WORKSPACE:?GITHUB_WORKSPACE is not set}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is not set}"

action_ref="${ACTION_REF:-}"
image_tag="${action_ref#v}"
if [[ -z "${image_tag}" ]]; then
  echo "::error::Could not resolve a KubePreflight image tag from github.action_ref (got '${action_ref}'). Pin this action to a released tag, e.g. imneeteeshyadav98/kubepreflight/compare@v0.13.0-github-action-comparison, not a branch or SHA." >&2
  exit 1
fi
image="ghcr.io/imneeteeshyadav98/kubepreflight:${image_tag}"

comparison_out="${INPUT_COMPARISON_OUT:-comparison.json}"
gate_out="${INPUT_GATE_OUT:-gate.json}"

if [[ ! -f "${GITHUB_WORKSPACE}/${INPUT_BASELINE}" ]]; then
  echo "::error::baseline input '${INPUT_BASELINE}' does not exist in the workspace. Run a KubePreflight scan against the baseline ref first and point this action at its findings-out path." >&2
  exit 1
fi
if [[ ! -f "${GITHUB_WORKSPACE}/${INPUT_CURRENT}" ]]; then
  echo "::error::current input '${INPUT_CURRENT}' does not exist in the workspace. Run a KubePreflight scan against the current ref first and point this action at its findings-out path." >&2
  exit 1
fi

compare_args=(compare --baseline "/work/${INPUT_BASELINE}" --current "/work/${INPUT_CURRENT}" \
  --json-out "/work/${comparison_out}" --gate-out "/work/${gate_out}" \
  --fail-on-new-blockers="${INPUT_FAIL_ON_NEW_BLOCKERS:-true}" \
  --warning-policy "${INPUT_WARNING_POLICY:-ignore}" \
  --fail-on-verdict-regression="${INPUT_FAIL_ON_VERDICT_REGRESSION:-true}" \
  --minimum-score-delta "${INPUT_MINIMUM_SCORE_DELTA:-0}")

# Same UID-matching reason as the top-level scan action: the image runs as
# a fixed nonroot UID that won't match whatever UID owns GITHUB_WORKSPACE
# on the runner, so a plain bind-mount write (comparison.json/gate.json)
# fails with permission denied without this.
docker_args=(run --rm --user "$(id -u):$(id -g)" -v "${GITHUB_WORKSPACE}:/work" -w /work)

echo "::group::kubepreflight compare"
set +e
docker "${docker_args[@]}" "${image}" "${compare_args[@]}"
compare_exit=$?
set -e
echo "::endgroup::"

gate_path="${GITHUB_WORKSPACE}/${gate_out}"
comparison_path="${GITHUB_WORKSPACE}/${comparison_out}"

# compare's own exit code is 1 for BOTH a real usage/document error and a
# gate "fail" decision (see internal/cli/compare.go) -- the gate.json file
# actually existing is what distinguishes "the gate ran and failed" from
# "compare never got that far", the same file-presence-over-exit-code
# principle the top-level scan entrypoint already applies for its own
# INFRA_FAILURE check.
if [[ ! -f "${gate_path}" ]]; then
  echo "::error::KubePreflight compare did not produce a gate decision (container exit code ${compare_exit}). Check the compare log above for the real cause (malformed findings.json, an invalid --warning-policy value)." >&2
  exit 1
fi

decision=$(jq -r '.decision' "${gate_path}")
reasons=$(jq -r '.reasons // [] | join(",")' "${gate_path}")
new_blockers=$(jq -r '.newBlockers' "${gate_path}")
new_warnings=$(jq -r '.newWarnings' "${gate_path}")
current_warnings=$(jq -r '.currentWarnings' "${gate_path}")
resolved_findings=$(jq -r '.resolvedFindings' "${gate_path}")
score_delta=$(jq -r '.scoreDelta' "${gate_path}")

{
  echo "decision=${decision}"
  echo "reasons=${reasons}"
  echo "new-blockers=${new_blockers}"
  echo "new-warnings=${new_warnings}"
  echo "current-warnings=${current_warnings}"
  echo "resolved-findings=${resolved_findings}"
  echo "score-delta=${score_delta}"
} >>"${GITHUB_OUTPUT}"

if [[ -f "${comparison_path}" ]]; then
  echo "comparison-file=${comparison_out}" >>"${GITHUB_OUTPUT}"
fi
echo "gate-file=${gate_out}" >>"${GITHUB_OUTPUT}"

case "${decision}" in
pass)
  exit 0
  ;;
neutral)
  echo "::warning::KubePreflight compare gate decision is neutral (insufficient evidence: ${reasons}) -- not failing the job, but the comparison could not confidently confirm no regression. See the compare log above." >&2
  exit 0
  ;;
fail)
  echo "::error::KubePreflight compare gate decision is fail (${reasons})." >&2
  exit 1
  ;;
*)
  echo "::error::Unrecognized KubePreflight gate decision '${decision}'." >&2
  exit 1
  ;;
esac
