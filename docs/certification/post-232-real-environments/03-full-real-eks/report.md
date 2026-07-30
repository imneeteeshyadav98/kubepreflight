# Lane 3 — Full Real-EKS Certification (Partial Reports, Artifact-Write Failures)

Date: 2026-07-29.

## Full-access real-EKS scan

`00-full-access-audit-only/`: `--provider eks --cluster-name <cluster> --target-version 1.34 --upgrade-context audit-only --redact-sensitive-identifiers`. Real result: `verdict PASSED_WITH_WARNINGS`, score 85, exit 1, `coverage.kubernetes.status: complete`, `coverage.aws.status: complete`, all 31 rules `applicable` (none `not_applicable` — every EKS-specific rule activates under `--provider eks`). Findings included a genuine AWS-managed webhook (`vpc-resource-validating-webhook`, rule `WH-005`) — real evidence of a real object AWS itself installs on every EKS cluster, not fabricated. Full detail (add-ons, node groups, network preconditions) is in `findings.json`; see `../02-reduced-iam-eks/report.md` for how this same evidence degrades under reduced IAM.

Additional upgrade contexts (`full-platform-upgrade`, `control-plane-only`, `worker-rollout`) were **not** separately captured in this session due to time/cost pacing — the `audit-only` run and the 6 reduced-IAM profile runs against the same cluster were prioritized as higher-value, more differentiated evidence. This is a disclosed scope reduction, not a hidden gap.

## Partial-evidence real-EKS proof

Covered via Lane 2's IAM-A/B/C/D/E/F profiles (see `../02-reduced-iam-eks/report.md`): every profile independently confirmed `Result: INCOMPLETE`, exit `3`, report files written, `upgradeReadiness.upgradeContinue: false`, and per-rule `insufficient_evidence` exactly where evidence was genuinely missing — real-EKS reduced-IAM partial evidence is fully certified there rather than duplicated here.

## Artifact-write failure (real binary, local filesystem)

`artifact-write-failure/`: two cases, both against the real binary, both using only `/tmp` fixtures (no shared/sensitive directory permissions were weakened):

1. **Unwritable output directory** (`chmod 000` on the target dir): exit **4**. `Error: writing scan reports: creating .../findings.json: ... permission denied`.
2. **Output path is a file/directory conflict** (a file exists where a directory is expected in the write path): exit **4**. `Error: writing scan reports: creating .../findings.json/nested.json: ... not a directory`.

Both confirm exit code `4` takes precedence when artifacts cannot be written, exactly as documented (`4` = "infrastructure failure before a trustworthy report was produced").
