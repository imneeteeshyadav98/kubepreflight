#!/usr/bin/env bash
# Sanity-checks the captured evidence in demo/live-eks/evidence/ before it
# gets recorded/committed anywhere: expected finding IDs actually fired,
# no account ID/ARN/private hostname leaked, before/after are the same
# cluster, reports exist and open, and the comparison's own numbers are
# internally consistent. Run this after compare.sh and before recording
# or destroying the cluster -- catching a surprise here is much cheaper
# than catching it after teardown.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
evidence_dir="${repo_root}/demo/live-eks/evidence"

python3 - "${evidence_dir}" <<'PYEOF'
import json
import re
import sys
from pathlib import Path

evidence_dir = Path(sys.argv[1])
failures = []

def check(label, condition):
    if not condition:
        failures.append(label)

before = json.loads((evidence_dir / "before" / "findings.json").read_text())
after = json.loads((evidence_dir / "after" / "findings.json").read_text())
gate = json.loads((evidence_dir / "compare" / "gate.json").read_text())

before_rule_ids = {f["ruleId"] for f in before["findings"]}
after_rule_ids = {f["ruleId"] for f in after["findings"]}

expected_before = {"API-001", "PDB-001", "PDB-002", "WH-001", "WH-002", "WORKLOAD-001"}
check(f"before findings include {sorted(expected_before)}", expected_before.issubset(before_rule_ids))

expected_resolved = {"PDB-001", "PDB-002", "WH-001", "WH-002"}
check(f"after findings no longer include {sorted(expected_resolved)}", expected_resolved.isdisjoint(after_rule_ids))

expected_remaining = {"API-001", "WORKLOAD-001"}
check(f"after findings still include {sorted(expected_remaining)} (deliberately not remediated)", expected_remaining.issubset(after_rule_ids))

check("before and after scanned the same cluster", before["clusterContext"] == after["clusterContext"])
check("before verdict is BLOCKED", before["upgradeReadiness"]["verdict"] == "BLOCKED")

check("gate decision is 'pass' (no new blockers introduced by remediation)", gate["decision"] == "pass")
check("gate reports zero new blockers", gate["newBlockers"] == 0)
check("gate reports resolved findings > 0", gate["resolvedFindings"] > 0)

# No AWS account ID (12 consecutive digits not embedded in a longer hex
# run -- avoids false positives inside SHA-like fingerprint hashes), no
# raw ARN, no private EC2-style hostname/IP anywhere in captured evidence.
account_id_pattern = re.compile(r"(?<![0-9a-fA-F])[0-9]{12}(?![0-9a-fA-F])")
arn_pattern = re.compile(r"arn:aws:[a-zA-Z0-9:/_-]+")
ip_pattern = re.compile(r"\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b")
hostname_pattern = re.compile(r"ip-[0-9-]+\.[a-zA-Z0-9.-]*\.internal")

leak_findings = []
for path in evidence_dir.rglob("*"):
    if not path.is_file() or path.suffix not in (".json", ".md", ".html"):
        continue
    text = path.read_text(errors="ignore")
    for pattern, name in [(account_id_pattern, "account ID"), (arn_pattern, "ARN"), (ip_pattern, "IP address"), (hostname_pattern, "private hostname")]:
        m = pattern.search(text)
        if m:
            leak_findings.append(f"{path.relative_to(evidence_dir)}: possible {name} ({m.group()!r})")

check("no account ID / ARN / IP / private hostname found in captured evidence", not leak_findings)
for item in leak_findings:
    failures.append(f"  -> {item}")

for phase in ("before", "after"):
    report = evidence_dir / phase / "report.html"
    check(f"{phase}/report.html exists and is non-trivial", report.exists() and report.stat().st_size > 1000)

if failures:
    print(f"verify-expected-output failed ({len(failures)}):", file=sys.stderr)
    for f in failures:
        print(f"- {f}", file=sys.stderr)
    sys.exit(1)

print("verify-expected-output: all checks passed.")
PYEOF
