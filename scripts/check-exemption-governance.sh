#!/usr/bin/env bash
# Validates the false-positive exemption governance registry and audit
# inventory. Thin wrapper around cmd/exemptioncheck so local and CI execution
# share the same implementation, with an additional deterministic-output guard.
set -euo pipefail

cd "$(dirname "$0")/.."

export GOCACHE="${GOCACHE:-/tmp/kubepreflight-gocache}"

first="$(go run ./cmd/exemptioncheck)"
echo "$first"

second="$(go run ./cmd/exemptioncheck)"
if [[ "$first" != "$second" ]]; then
  echo
  echo "exemption governance check FAILED: output differs across repeated runs (non-deterministic)" >&2
  diff <(echo "$first") <(echo "$second") >&2 || true
  exit 1
fi

echo
echo "Deterministic output: OK (byte-identical across repeated runs)"
