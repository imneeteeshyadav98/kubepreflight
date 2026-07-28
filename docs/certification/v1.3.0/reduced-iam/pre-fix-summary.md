# Reduced-IAM certification — pre-fix regression (structural summary)

This is a sanitized, structural summary of the release-blocking finding
discovered during the first real-EKS reduced-IAM certification run,
against merged master `0442ad4` (before corrective PR #228). The full
raw evidence for this run is preserved locally (gitignored, not
committed — see `docs/certification/v1.3.0/reduced-iam/findings.json`/
`report.md`/`report.html` in this same directory tree) as regression
proof, but committing it in full raw form was judged unnecessary once
this structural summary and the post-fix comparison below establish the
same facts without re-embedding real cluster identifiers.

## What was observed (real, not synthetic)

A reduced-IAM scan — deliberately reduced to `eks:DescribeCluster` only,
via a temporary, scoped IAM role — against the same real cluster used
for the full-access baseline produced:

| Field | Pre-fix value |
|---|---|
| `schemaVersion` | `1.1` |
| `ruleExecutions` | 31 unique, all `evaluated` |
| `coverage.kubernetes.status` | `complete` |
| `coverage.aws.status` | `partial` (7 real `AccessDenied` collector failures: `list-insights`, `list-addons`, `list-nodegroups`, `describe-subnets`, `describe-vpc`, 2× `describe-security-group`) |
| `Result` | `INCOMPLETE` |
| Exit code | `3` |
| **Rule execution coverage (displayed)** | **Complete** |
| **Overall decision coverage (displayed)** | **Complete — this was the bug** |
| Score-qualification/advisory text | **absent** |
| Readiness score | `89` |

## The inconsistency

The same document simultaneously said `Coverage.AWS.status = partial`
and `Result = INCOMPLETE` (both correct) **and** displayed rule-execution
coverage as `Complete` with no qualification text anywhere near the
readiness score (incorrect — no signal told an operator that AWS
evidence was genuinely incomplete).

## Root cause

`report.BuildEvaluationCoverage` (pre-fix) derived its displayed coverage
status purely from `RuleExecutions`, with no path to read
`Report.Coverage` (the plane-level evidence-completeness mechanism). Since
AWS-plane rules have no `insufficient_evidence` mapping in
`internal/rules/execution.go`'s `ruleErrorsMapKeys` (only 6
Kubernetes-plane rules are covered), an AWS collector failure never
reached any individual rule's execution state — so all 31 rules
correctly-but-misleadingly read `evaluated`.

## Fix

PR #228 (merged as `a91deca`) added `report.BuildOverallCoverage`,
combining the existing rule-execution coverage with `Report.Coverage`
(evidence-plane coverage) into one honest combined status — without
rewriting any `RuleExecutionRecord`. See `../environment.md` and the
corrective PR itself for full detail.

## Post-fix proof

See `post-fix-findings.json`/`post-fix-report.md`/`post-fix-report.html`
(sanitized) and `assertions.txt` in this same directory — rerun under
equivalent conditions against the fixed binary (merged master `a91deca`).

| Field | Pre-fix | Post-fix |
|---|---|---|
| Findings | 3 | 3 (identical) |
| `coverage.aws.status` | partial | partial (identical) |
| Readiness score | 89 | 89 (identical — confirms score formula untouched) |
| `Result` / exit code | INCOMPLETE / 3 | INCOMPLETE / 3 (identical) |
| Rule execution coverage | Complete | Complete (identical — never rewritten) |
| **Overall decision coverage** | **Complete (bug)** | **Partial (fixed)** |
| Score-qualification/advisory | absent | **present, names the degraded AWS plane** |

Every structural fact is byte-identical between the two runs — confirming
this was purely a presentation/decision-context fix, exactly as scoped.
