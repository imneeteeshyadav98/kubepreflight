#!/usr/bin/env bash
# Validates the embedded compatibility catalog (internal/compatcatalog)
# and reports its coverage/staleness matrix. Thin wrapper around
# cmd/compatcatalogcheck (not part of the public CLI) so this can be run
# the same way locally and in CI's verify job. Exits non-zero if the
# catalog fails schema/field validation or is missing required add-on
# coverage for any target version it otherwise supports -- staleness is
# report-only and never affects the exit code.
#
# Usage:
#   scripts/check-compatibility-catalog.sh
#   scripts/check-compatibility-catalog.sh --stale-after-days 90
set -euo pipefail

cd "$(dirname "$0")/.."

go run ./cmd/compatcatalogcheck "$@"
