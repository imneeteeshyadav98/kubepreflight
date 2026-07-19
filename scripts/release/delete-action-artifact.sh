#!/usr/bin/env bash
# Deletes a GitHub Actions run artifact by name, retrying briefly since the
# artifact upload from a just-completed composite-action step isn't always
# immediately visible via the API. Used between repeated invocations of
# the same Action within one validation workflow (e.g. running the
# kubepreflight scan Action multiple times against different fixtures),
# since actions/upload-artifact rejects a second upload under a name
# that's still in use by an existing artifact from earlier in the same
# run.
#
# Usage: delete-action-artifact.sh <artifact-name>
# Requires: GH_TOKEN, GITHUB_REPOSITORY, GITHUB_RUN_ID (standard on a
# GitHub Actions runner).
set -euo pipefail

name="${1:?usage: delete-action-artifact.sh <artifact-name>}"

for _ in 1 2 3 4 5 6; do
  ids="$(gh api "repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/artifacts?per_page=100" \
    --jq ".artifacts[] | select(.name == \"${name}\") | .id")"
  if [[ -n "${ids}" ]]; then
    while IFS= read -r id; do
      [[ -z "${id}" ]] && continue
      gh api -X DELETE "repos/${GITHUB_REPOSITORY}/actions/artifacts/${id}"
    done <<<"${ids}"
    exit 0
  fi
  sleep 5
done
