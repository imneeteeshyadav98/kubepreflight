#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/live-eks/lib.sh
source "${script_dir}/lib.sh"

pattern="$(sensitive_pattern)"
paths=("$@")
if [ "${#paths[@]}" -eq 0 ]; then
  paths=("${LIVE_EKS_SANITIZED_DIR}")
fi

for path in "${paths[@]}"; do
  [ -e "${path}" ] || die "path does not exist: ${path}"
done

tmp_matches="$(mktemp)"
trap 'rm -f "${tmp_matches}"' EXIT
for path in "${paths[@]}"; do
  if [ -f "${path}" ]; then
    grep -n -E "${pattern}" "${path}" >>"${tmp_matches}" || true
    continue
  fi
  find "${path}" -type f -print0 |
    xargs -0 grep -n -E "${pattern}" >>"${tmp_matches}" || true
done

if [ -s "${tmp_matches}" ]; then
  cat "${tmp_matches}"
  die "sensitive AWS account ID, ARN, or private EC2 hostname pattern found"
fi

echo "OK: redaction leak scan clean"
