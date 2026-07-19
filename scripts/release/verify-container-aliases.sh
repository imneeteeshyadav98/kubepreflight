#!/usr/bin/env bash
# Verifies both published GHCR tags (bare and v-prefixed) exist and share
# the same immutable digest, and that the container reports the same
# release provenance as the release binaries -- KP-V1-INSTALL-001's
# permanent regression guard for the exact bug found and fixed in
# v0.16.1-security-trust/v0.16.2-security-trust (only the bare alias was
# ever published; docker/metadata-action silently discarded the
# v-prefixed pattern for every pre-release-shaped tag this project has
# ever cut).
#
# Uses `docker buildx imagetools inspect` to compare immutable registry
# digests, not local image IDs -- a local image ID is mutable/rebuildable
# and proves nothing about what the registry actually serves under each
# tag.
#
# Requires: GITHUB_REF_NAME set, docker already authenticated to ghcr.io.
set -euo pipefail

: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is not set}"

image="ghcr.io/imneeteeshyadav98/kubepreflight"
bare_tag="${GITHUB_REF_NAME#v}"
v_tag="${GITHUB_REF_NAME}"

echo "== both aliases exist =="

# Wraps `docker buildx imagetools inspect` so a missing tag produces one
# clean FAIL line instead of a raw docker error followed by a Python
# traceback from trying to parse that error text as JSON.
resolve_digest() {
  local tag="$1" manifest_json
  if ! manifest_json="$(docker buildx imagetools inspect "${tag}" --format '{{json .Manifest}}' 2>&1)"; then
    echo "FAIL: ${tag} -- docker buildx imagetools inspect failed:" >&2
    echo "${manifest_json}" >&2
    exit 1
  fi
  python3 -c "import sys,json; print(json.loads(sys.argv[1])['digest'])" "${manifest_json}"
}

bare_digest="$(resolve_digest "${image}:${bare_tag}")"
v_digest="$(resolve_digest "${image}:${v_tag}")"

if [ -z "${bare_digest}" ]; then
  echo "FAIL: ${image}:${bare_tag} has no resolvable digest"
  exit 1
fi
if [ -z "${v_digest}" ]; then
  echo "FAIL: ${image}:${v_tag} has no resolvable digest"
  exit 1
fi

echo "  ${bare_tag} -> ${bare_digest}"
echo "  ${v_tag} -> ${v_digest}"

echo "== both aliases share one digest =="
if [ "${bare_digest}" != "${v_digest}" ]; then
  echo "FAIL: ${bare_tag} and ${v_tag} resolve to different digests"
  exit 1
fi

echo "== container provenance matches this release =="
expected="KubePreflight ${bare_tag}"
out="$(docker run --rm "${image}:${v_tag}" version)"
echo "${out}"
[ "$(echo "${out}" | head -n1)" = "${expected}" ] || { echo "want first line '${expected}', got: ${out}"; exit 1; }
echo "${out}" | grep -q "^commit: unknown$" && { echo "commit is 'unknown' -- ldflags did not reach the published image"; exit 1; }
echo "${out}" | grep -q "^built: unknown$" && { echo "built date is 'unknown' -- ldflags did not reach the published image"; exit 1; }

echo "OK: ${image}:${bare_tag} and ${image}:${v_tag} both resolve to ${bare_digest}"
