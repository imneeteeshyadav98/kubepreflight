#!/usr/bin/env bash
# End-to-end test for the KubePreflight Compare composite Action
# (compare/action.yml, compare/entrypoint.sh).
#
# Builds the released kubepreflight Docker image locally and invokes
# compare/entrypoint.sh directly against each fixture under
# compare/testdata/fixtures/, exactly as compare/action.yml's single step
# would -- exercising the real image, the real `compare` CLI, and the real
# jq/GITHUB_OUTPUT/GITHUB_STEP_SUMMARY/annotation logic together, so this
# catches anything a Go-level unit test of internal/gate or internal/cli
# can't (a jq field-name typo, a shell quoting bug, a Docker mount
# problem). This does NOT exercise action.yml's own composite-step
# wiring or a real github.action_ref -- `uses: ./compare` in a workflow
# leaves github.action_ref empty, which entrypoint.sh deliberately
# refuses to run against (a caller must pin to a released tag, not a
# branch/SHA). Testing the composite action through a real `uses:`
# reference against a real published tag is the Final milestone step's
# job, on an actual GitHub Actions runner after release.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
image_tag="compare-action-e2e-test"
image="ghcr.io/imneeteeshyadav98/kubepreflight:${image_tag}"

echo "Building ${image}..."
docker build -q -t "${image}" "${repo_root}" >/dev/null

failures=0

# run_scenario name expect_decision expect_exit
run_scenario() {
  local name="$1" expect_decision="$2" expect_exit="$3"
  local fixture_dir="${repo_root}/compare/testdata/fixtures/${name}"
  local workspace
  workspace="$(mktemp -d)"

  cp "${fixture_dir}/baseline.json" "${workspace}/baseline.json"
  cp "${fixture_dir}/current.json" "${workspace}/current.json"

  local gh_output="${workspace}/github_output"
  local gh_summary="${workspace}/github_summary"
  local log="${workspace}/log"
  : >"${gh_output}"
  : >"${gh_summary}"

  set +e
  GITHUB_WORKSPACE="${workspace}" \
    GITHUB_OUTPUT="${gh_output}" \
    GITHUB_STEP_SUMMARY="${gh_summary}" \
    ACTION_REF="v${image_tag}" \
    INPUT_BASELINE="baseline.json" \
    INPUT_CURRENT="current.json" \
    INPUT_COMPARISON_OUT="comparison.json" \
    INPUT_GATE_OUT="gate.json" \
    INPUT_FAIL_ON_NEW_BLOCKERS="true" \
    INPUT_WARNING_POLICY="ignore" \
    INPUT_FAIL_ON_VERDICT_REGRESSION="true" \
    INPUT_MINIMUM_SCORE_DELTA="0" \
    "${repo_root}/compare/entrypoint.sh" >"${log}" 2>&1
  local actual_exit=$?
  set -e

  local ok=1
  local actual_decision
  actual_decision=$(grep '^decision=' "${gh_output}" | cut -d= -f2- || true)

  if [[ "${actual_exit}" -ne "${expect_exit}" ]]; then
    echo "FAIL [${name}]: expected exit ${expect_exit}, got ${actual_exit}"
    ok=0
  fi
  if [[ "${actual_decision}" != "${expect_decision}" ]]; then
    echo "FAIL [${name}]: expected decision '${expect_decision}', got '${actual_decision}'"
    ok=0
  fi
  if [[ ! -s "${gh_summary}" ]]; then
    echo "FAIL [${name}]: GITHUB_STEP_SUMMARY was not written"
    ok=0
  fi
  if [[ "${name}" == "fail-new-blocker" ]] && ! grep -q '::error file=' "${log}"; then
    echo "FAIL [${name}]: expected a ::error file=...:: annotation for the new blocker, found none"
    ok=0
  fi

  if [[ "${ok}" -eq 1 ]]; then
    echo "PASS [${name}]: decision=${actual_decision} exit=${actual_exit}"
  else
    echo "--- entrypoint.sh output for ${name} ---"
    cat "${log}"
    echo "--- GITHUB_STEP_SUMMARY for ${name} ---"
    cat "${gh_summary}"
    failures=$((failures + 1))
  fi

  rm -rf "${workspace}"
}

run_scenario "pass" "pass" 0
run_scenario "fail-new-blocker" "fail" 1
run_scenario "neutral-incomplete" "neutral" 0

if [[ "${failures}" -gt 0 ]]; then
  echo ""
  echo "${failures} compare Action e2e scenario(s) failed."
  exit 1
fi
echo ""
echo "All compare Action e2e scenarios passed."
