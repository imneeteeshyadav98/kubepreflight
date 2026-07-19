#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/live-eks/lib.sh
source "${script_dir}/lib.sh"

pattern="$(forbidden_mutation_pattern)"
mkdirs

if find "${script_dir}" -type f ! -name verify-read-only.sh ! -name lib.sh -print0 | xargs -0 grep -n -E "${pattern}"; then
  die "forbidden mutation command found in live EKS harness scripts"
fi

if [ -s "${LIVE_EKS_COMMAND_LOG}" ] && grep -n -E "${pattern}" "${LIVE_EKS_COMMAND_LOG}"; then
  die "forbidden mutation command found in recorded evidence commands"
fi

if command -v kubectl >/dev/null 2>&1 && [ -n "${EXPECTED_KUBE_CONTEXT:-}" ]; then
  mkdirs
  kubectl auth can-i --list >"${LIVE_EKS_RAW_DIR}/verify-read-only-can-i-list.txt" || true
fi

cat >"${LIVE_EKS_WORKDIR}/read-only-verification.md" <<'EOF'
# Read-Only Verification

The harness command inventory rejects Kubernetes and EKS mutation commands,
including kubectl apply/create/delete/patch/edit/label/annotate/replace/scale,
node drain/cordon operations, EKS update/start-update operations, and any
fictional rollback execution command. KubePreflight rollback coverage is limited
to `rollback plan` and `rollback assess`.
EOF

echo "OK: read-only command inventory verified"
