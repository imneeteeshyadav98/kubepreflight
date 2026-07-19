#!/usr/bin/env bash
# Downloads one platform archive plus the shared checksums file and SBOM
# from the GitHub Release for $GITHUB_REF_NAME, verifies the archive's
# checksum, confirms the SBOM is present and valid, and extracts the
# archive. Shared by every native-platform verification job
# (Linux/macOS) so the download-verify-extract boilerplate exists once,
# not once per job -- KP-V1-INSTALL-001.
#
# Usage: verify-artifacts.sh <archive-suffix e.g. linux_amd64> <extract-dir>
#
# Requires: GITHUB_REF_NAME, GH_TOKEN (or gh already authenticated), and
# GITHUB_REPOSITORY set in the environment (all standard on a GitHub
# Actions runner). Not used for the Windows job, which downloads and
# checks its own .zip archive directly in PowerShell -- .tar.gz
# extraction doesn't translate, and there is no cross-platform shell to
# share this exact script with.
set -euo pipefail

archive_suffix="${1:?usage: verify-artifacts.sh <archive-suffix> <extract-dir>}"
extract_dir="${2:?usage: verify-artifacts.sh <archive-suffix> <extract-dir>}"
: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is not set}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"

mkdir -p "${extract_dir}"

archive="kubepreflight_${GITHUB_REF_NAME}_${archive_suffix}.tar.gz"
checksums="kubepreflight_${GITHUB_REF_NAME}_checksums.txt"
sbom="kubepreflight_${GITHUB_REF_NAME}_sbom.spdx.json"

gh release download "${GITHUB_REF_NAME}" \
  --repo "${GITHUB_REPOSITORY}" \
  --dir "${extract_dir}" \
  --pattern "${archive}" \
  --pattern "${checksums}" \
  --pattern "${sbom}"

# Everything in this subshell is redirected to stderr -- sha256sum/shasum
# print a verbose "<file>: OK" line to *stdout* on success, which would
# otherwise get captured alongside the real return value by a caller
# doing `bin="$(verify-artifacts.sh ...)"`, silently producing a
# two-line value that breaks GITHUB_ENV's KEY=VALUE parser downstream.
# Reproduced and confirmed locally before this fix: capturing this
# script's output via command substitution returned the checksum tool's
# "OK" line as line 1 and the real binary path as line 2. This script's
# stdout contract is exactly one line, the binary path, printed once,
# at the very end -- nothing else may ever reach stdout.
(
  cd "${extract_dir}"
  # macOS has no sha256sum by default (BSD ships shasum -a 256 instead);
  # Linux runners always have sha256sum. Prefer sha256sum when present so
  # this stays identical to every other checksum check in this repo.
  if command -v sha256sum >/dev/null 2>&1; then
    grep -- "${archive}" "${checksums}" | sha256sum -c -
  else
    grep -- "${archive}" "${checksums}" | shasum -a 256 -c -
  fi
  test -s "${sbom}"
  python3 -c "import json,sys; d=json.load(open('${sbom}')); assert d.get('spdxVersion') and d.get('packages'), 'SBOM missing spdxVersion/packages'"
) >&2

tar -xzf "${extract_dir}/${archive}" -C "${extract_dir}"

bin_dir="${extract_dir}/kubepreflight_${GITHUB_REF_NAME}_${archive_suffix}"
chmod +x "${bin_dir}/kubepreflight"

echo "${bin_dir}/kubepreflight"
