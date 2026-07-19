#!/usr/bin/env bash
# Shared verification logic for a downloaded, extracted release binary --
# used by every Linux/macOS job in the published installation matrix
# (KP-V1-INSTALL-001), so the same checks run identically on every
# platform instead of drifting per-job copies of the same bash.
#
# Usage: verify-published-binary.sh <path-to-binary> <light|deep> [output-dir]
#
# light: version/--version parity and real provenance, --help present on
#        all five commands, --redact-sensitive-identifiers present on all
#        five surfaces. Fast, no cluster/manifest scan.
# deep:  everything in light, plus a deterministic fixture scan against
#        the removed-API positive fixture (same one scan_test.go and the
#        existing Linux release-verification job already lock a BLOCKED
#        verdict to), asserting exit code 2, verdict BLOCKED, all three
#        report formats present and findings.json parseable, and a
#        redaction-leak grep across the generated output.
#
# Must be run from the repository root (needs testdata/manifest-repo/raw
# for the deep fixture scan) with GITHUB_REF_NAME set.
set -euo pipefail

BIN="${1:?usage: verify-published-binary.sh <path-to-binary> <light|deep> [output-dir]}"
MODE="${2:?usage: verify-published-binary.sh <path-to-binary> <light|deep> [output-dir]}"
OUT_DIR="${3:-verify-scan-out}"
: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is not set}"

if [[ "${MODE}" != "light" && "${MODE}" != "deep" ]]; then
  echo "unknown mode '${MODE}', want light or deep" >&2
  exit 1
fi

echo "== version banner matches this release =="
expected="KubePreflight ${GITHUB_REF_NAME#v}"
out="$("${BIN}" version)"
echo "${out}"
[ "$(echo "${out}" | head -n1)" = "${expected}" ] || { echo "want first line '${expected}', got: ${out}"; exit 1; }
echo "${out}" | grep -q "^commit: unknown$" && { echo "commit is 'unknown' -- ldflags did not reach this binary"; exit 1; }
echo "${out}" | grep -q "^built: unknown$" && { echo "built date is 'unknown' -- ldflags did not reach this binary"; exit 1; }
diff <("${BIN}" --version) <(echo "${out}")

echo "== --help present on all five command surfaces =="
for cmd_args in "scan" "plan" "rollback plan" "rollback assess" "compare"; do
  # shellcheck disable=SC2086
  if ! "${BIN}" ${cmd_args} --help >/dev/null 2>&1; then
    echo "kubepreflight ${cmd_args} --help failed"
    exit 1
  fi
done

echo "== --redact-sensitive-identifiers present on all five surfaces =="
for cmd_args in "scan" "plan" "rollback plan" "rollback assess" "compare"; do
  # shellcheck disable=SC2086
  if ! "${BIN}" ${cmd_args} --help | grep -q -- "--redact-sensitive-identifiers"; then
    echo "missing --redact-sensitive-identifiers on: kubepreflight ${cmd_args}"
    exit 1
  fi
done

if [[ "${MODE}" == "light" ]]; then
  echo "OK (light): ${BIN}"
  exit 0
fi

echo "== deep: deterministic fixture scan =="
mkdir -p "${OUT_DIR}"
set +e
"${BIN}" scan \
  --target-version 1.34 \
  --manifests-only \
  --manifests testdata/manifest-repo/raw \
  --output all \
  --output-dir "${OUT_DIR}" \
  --redact-sensitive-identifiers \
  --serve-report never \
  --terminal-output silent
code=$?
set -e
# testdata/manifest-repo/raw/psp.yaml is a removed-API (policy/v1beta1
# PodSecurityPolicy) positive fixture -- same one scan_test.go locks a
# BLOCKED verdict to -- so this scan must always exit 2, deterministically.
if [ "${code}" -ne 2 ]; then
  echo "expected exit code 2 (BLOCKED) scanning the removed-API fixture, got ${code}"
  exit 1
fi

echo "== deep: artifacts exist, parse, and verdict is BLOCKED =="
for f in findings.json report.md report.html; do
  test -s "${OUT_DIR}/${f}" || { echo "missing ${OUT_DIR}/${f}"; exit 1; }
done
verdict="$(python3 -c "import json; print(json.load(open('${OUT_DIR}/findings.json'))['upgradeReadiness']['verdict'])")"
if [ "${verdict}" != "BLOCKED" ]; then
  echo "expected verdict BLOCKED, got ${verdict}"
  exit 1
fi

echo "== deep: redaction leak grep =="
if grep -R -n -E \
  'arn:aws:|(^|[^0-9])[0-9]{12}([^0-9]|$)|ip-[0-9]+-[0-9]+-[0-9]+-[0-9]+\.(ec2|[a-z0-9.-]*compute)\.internal' \
  "${OUT_DIR}"; then
  echo "sensitive identifier pattern found in redacted output"
  exit 1
fi

echo "OK (deep): ${BIN}"
