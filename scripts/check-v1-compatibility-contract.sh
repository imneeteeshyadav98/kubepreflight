#!/usr/bin/env bash
set -euo pipefail

# Validates the v1 compatibility contract and checks the output is stable.

export GOCACHE="${GOCACHE:-/tmp/kubepreflight-gocache}"

first="$(go run ./cmd/v1compatcheck)"
second="$(go run ./cmd/v1compatcheck)"

if [[ "${first}" != "${second}" ]]; then
  echo "v1 compatibility contract check FAILED: output differs across repeated runs (non-deterministic)" >&2
  exit 1
fi

printf '%s\n' "${first}"
echo "Deterministic output: OK (byte-identical across repeated runs)"
