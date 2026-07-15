#!/usr/bin/env bash
# Validates the embedded versioned API catalog (internal/apicatalog) and
# reports its coverage/staleness matrix. Thin wrapper around
# cmd/apicatalogcheck (not part of the public CLI) so this can be run the
# same way locally and in CI's verify job, plus a determinism check the
# Go command itself can't observe from a single run: running it twice
# must produce byte-identical output, proving the catalog's deterministic
# sort/validation actually holds end-to-end, not just at the unit-test
# level. Exits non-zero if the catalog fails schema/field validation, has
# drifted from the frozen legacy inventory, has a gap in its declared
# target-version range, or produces non-deterministic output --
# staleness is report-only and never affects the exit code.
#
# Usage:
#   scripts/check-api-version-catalog.sh
#   scripts/check-api-version-catalog.sh --stale-after-days 90
set -euo pipefail

cd "$(dirname "$0")/.."

first="$(go run ./cmd/apicatalogcheck "$@")"
echo "$first"

second="$(go run ./cmd/apicatalogcheck "$@")"
if [[ "$first" != "$second" ]]; then
  echo
  echo "api-version-catalog check FAILED: output differs across repeated runs (non-deterministic)" >&2
  diff <(echo "$first") <(echo "$second") >&2 || true
  exit 1
fi
echo
echo "Deterministic output: OK (byte-identical across repeated runs)"
