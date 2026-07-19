#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/live-eks/lib.sh
source "${script_dir}/lib.sh"

need_cmd python3
mkdirs

python3 - "${LIVE_EKS_WORKDIR}" "${LIVE_EKS_SANITIZED_DIR}" "${EXPECTED_AWS_ACCOUNT_ID:-}" <<'PY'
import os
import re
import shutil
import sys
from pathlib import Path

root = Path(sys.argv[1])
dest = Path(sys.argv[2])
account = sys.argv[3]

skip_parts = {"raw", "release", "sanitized"}
token_re = re.compile(r"(^\s*token:\s*).+$", re.MULTILINE)
arn_re = re.compile(r"arn:aws:[A-Za-z0-9_./+=,@-]+:[A-Za-z0-9-]*:[0-9]{12}:[A-Za-z0-9_./+=,@:-]+")
account_re = re.compile(r"(?<![0-9])[0-9]{12}(?![0-9])")
host_re = re.compile(r"ip-[0-9]+-[0-9]+-[0-9]+-[0-9]+\.(?:ec2|[a-z0-9.-]*compute)\.internal")

if dest.exists():
    shutil.rmtree(dest)
dest.mkdir(parents=True)

for path in root.rglob("*"):
    if not path.is_file():
        continue
    rel = path.relative_to(root)
    if rel.parts and rel.parts[0] in skip_parts:
        continue
    out = dest / rel
    out.parent.mkdir(parents=True, exist_ok=True)
    data = path.read_bytes()
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        continue
    text = token_re.sub(r"\1<REDACTED_EKS_TOKEN>", text)
    text = arn_re.sub("<REDACTED_AWS_ARN>", text)
    text = host_re.sub("<REDACTED_EC2_PRIVATE_HOSTNAME>", text)
    if account:
        text = text.replace(account, "<REDACTED_AWS_ACCOUNT_ID>")
    text = account_re.sub("<REDACTED_AWS_ACCOUNT_ID>", text)
    out.write_text(text, encoding="utf-8")
PY

"${script_dir}/check-redaction.sh" "${LIVE_EKS_SANITIZED_DIR}"
echo "OK: sanitized evidence written to ${LIVE_EKS_SANITIZED_DIR}"
