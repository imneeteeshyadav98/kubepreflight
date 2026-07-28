# Expected results — exact, field-level assertion plan

Every assertion below cites the exact JSON field name/value it checks,
and the source in this codebase that defines that behavior. Where an
assertion is automatable, the corresponding `scripts/certification/`
script and the exact invocation are given — see `validation-summary.md`
for proof each script was actually run against a real fixture before
being trusted here.

## Full access (`full-access/findings.json`)

| # | Assertion | Field/value | Source |
|---|---|---|---|
| 1 | Schema version | `.schemaVersion == "1.1"` | `internal/findings/report.go`, `SchemaVersion` const |
| 2 | Rule execution count | `.ruleExecutions | length == 31` | `rules.AllRuleIDs()` (`internal/rules/defaults.go`) |
| 3 | Native, not backfilled | `.ruleExecutionsNormalized == false` (or field absent — `omitempty`) | `internal/cli/scan.go`: "RuleExecutions is native to this scan... RuleExecutionsNormalized is deliberately left unset/false" |
| 4 | Coverage status is truthful | `.coverage.kubernetes.status`, `.coverage.aws.status`, `.coverage.manifests.status` each `"complete"` **only if** the corresponding snapshot had zero collector errors; `"partial"` with a non-empty `.errors` array otherwise — never fabricated as `"complete"` when a real collector error occurred | `internal/cli/coverage.go`'s `buildScanCoverage` |
| 5 | No false `evaluated` | No `RuleExecutionRecord` may read `state: "evaluated"` for a rule whose specific, tracked collector dependency errored — applies only to the six rules `internal/rules/execution.go`'s `ruleErrorsMapKeys` actually tracks (`WH-002`, `PDB-001`, `PDB-002`, `CRD-001`, `CRD-002`, `APISERVICE-001`); those must instead read `state: "insufficient_evidence"` with a `reason` listing the missing collector key(s) | `internal/rules/rule.go`'s `RunAllWithExecutions`, `internal/rules/execution.go` |
| 6 | Every rule ID present exactly once | `.ruleExecutions[].ruleId` as a set equals the 31-ID universe, no duplicates | `rules.AllRuleIDs()` |
| 7 | EKS metadata populated | `.eksCluster`, `.eksAddons`, `.eksNodegroups`, `.eksUpgradeInsights` all non-null (full IAM access to every AWS collector call) | `internal/cli/coverage.go`'s `eksClusterInfo`/`eksAddonInfos`/`eksNodegroupInfos`/`eksUpgradeInsightInfos` |

Automated check:

```bash
scripts/certification/assert-findings.sh full-access docs/certification/v1.3.0/full-access/findings.json
```

## Reduced IAM (`reduced-iam/findings.json`)

| # | Assertion | Field/value | Source |
|---|---|---|---|
| 1 | Same schema/count invariants as full access | `.schemaVersion == "1.1"`, `.ruleExecutions | length == 31`, `.ruleExecutionsNormalized == false` | same as above — coverage context must never change the rule-execution contract itself |
| 2 | AWS coverage genuinely partial | `.coverage.aws.status == "partial"` and `.coverage.aws.errors` is non-empty, listing (in some form) the denied `list-addons`/`list-nodegroups`/`describe-subnets`/`describe-security-group:*`/`describe-vpc:*` operations | `internal/cli/coverage.go` — populated directly from `Snapshot.Errors` |
| 3 | **Documented, not hidden**: AWS-plane rules still read `evaluated`, not `insufficient_evidence` | `ADDON-001`, `ADDON-002`, `NODE-002`, `NET-002`, `EKS-NG-001..004`'s `RuleExecutionRecord.state == "evaluated"` even though their AWS evidence was IAM-denied — this is the confirmed, current scope boundary (`ruleErrorsMapKeys` has zero `awsKeys` entries in this build), not a certification failure. The certification's job is to confirm this is exactly what happens, not to expect something the code doesn't do. | `internal/rules/execution.go`'s `ruleErrorsMapKeys` |
| 4 | `EKSCluster` still populated | `.eksCluster` non-null (DescribeCluster stays allowed under the reduced policy) | `environment.md`'s reduced-IAM policy design |
| 5 | `EKSAddons`/`EKSNodegroups` empty or absent | `.eksAddons`/`.eksNodegroups` empty (ListAddons/ListNodegroups denied, so nothing to inventory) — must not be silently populated from a cached/stale source | `internal/cli/coverage.go`'s nil/empty guards |
| 6 | ReadinessScore formula unchanged | Given the *same set of findings* two reports would produce, the score arithmetic is identical regardless of coverage — coverage context changes which findings exist (less evidence → potentially fewer findings on AWS-dependent rules), never how a given finding set maps to a score. Verify by confirming `report.ScoreQualification`'s fixed text appears verbatim in Markdown/Terminal/HTML output: `"The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score."` | `internal/report/evaluation_coverage.go`, `ScoreQualification` const |
| 7 | Gate decision policy unchanged | `compare --gate-out`'s `.evaluationCoverage`/`.evaluationAdvisories` are additive/presentation-only and never themselves flip `.decision` — confirm no gate run in this certification fails *solely* because coverage was partial (a real new blocker must still be the cause of any `fail`) | `internal/gate/model.go`: "EvaluationCoverage and EvaluationAdvisories are additive, presentation-only" |

Automated check:

```bash
scripts/certification/assert-findings.sh reduced-iam docs/certification/v1.3.0/reduced-iam/findings.json
```

## Manifests-only (`manifests-only/findings.json`)

| # | Assertion | Field/value | Source |
|---|---|---|---|
| 1 | Rule count | `.ruleExecutions | length == 31` | same universe as every mode |
| 2 | Exactly 2 applicable rules | `API-001` and `API-002` — **verified against actual code, not just the task's hint** — `rules.NewManifestsOnlyRegistry()` registers exactly `API001{}` and `API002{}` | `internal/rules/defaults.go`'s `NewManifestsOnlyRegistry` |
| 3 | Those 2 are evaluated | `.ruleExecutions[] | select(.ruleId=="API-001" or .ruleId=="API-002") | .applicability == "applicable" and .state == "evaluated"` | confirmed by an actual local run — see `validation-summary.md` |
| 4 | Remaining 29 are excluded, explicitly | `.ruleExecutions[] | select(.ruleId != "API-001" and .ruleId != "API-002") | .applicability == "not_applicable" and .state == "not_evaluated"`, with `.reason == "not registered for this scan mode"` | `internal/rules/rule.go`'s `RunAllWithExecutions`, `!invoked[ruleID]` branch |
| 5 | Cluster/AWS coverage marked skipped, not complete/partial | `.coverage.kubernetes.status == "skipped"` and `.coverage.aws.status == "skipped"` (never attempted, so neither complete nor partial applies) | `internal/cli/coverage.go`'s default `CoverageSkipped` when the corresponding `*Requested` flag is false |
| 6 | Manifests coverage reflects what was actually scanned | `.coverage.manifests.status == "complete"` if `demo/eks/manifests/old-api.yaml` parsed without error | `internal/cli/coverage.go` |

Automated check (already run against a real, locally-generated fixture —
see `validation-summary.md`):

```bash
scripts/certification/assert-findings.sh manifests-only docs/certification/v1.3.0/manifests-only/findings.json
```

## Comparison proof — the single most important assertion in this PR

Full-access baseline (`full-access/findings.json`) vs. a reduced-scope
current report (`reduced-iam/findings.json` or
`manifests-only/findings.json`).

**The invariant** (PR 5, `internal/comparison/compare.go`'s
`ruleProvenEvaluated`): a baseline finding whose responsible rule is
`not_applicable`, `not_evaluated`, `insufficient_evidence`, or `failed`
in `current` must land in `comparison.json`'s `not_re_evaluated` array,
**never** `resolved` — resolution is only claimed when `current`'s
`RuleExecutionRecord` for that rule ID proves
`applicability: "applicable"` **and** `state: "evaluated"`.

Concretely, for `full-vs-manifests-only.json`:

- Every one of the 29 rules `manifests-only` marks `not_applicable`/
  `not_evaluated` must contribute its baseline findings (if any) to
  `.not_re_evaluated`, never `.resolved`.
- Only `API-001`/`API-002` baseline findings that are genuinely absent
  from `current` may appear in `.resolved`.
- `.summary.not_re_evaluated` (top-level count) must be `> 0` whenever
  the full-access baseline has at least one finding from a rule outside
  `{API-001, API-002}` — this is the concrete, non-synthetic proof this
  PR exists to produce, not merely a unit-test fixture result.

For `full-vs-reduced-iam.json`, the same invariant holds for any rule
whose `RuleExecutionRecord` in `reduced-iam/findings.json` is not
`applicable`/`evaluated` — per the reduced-IAM section above, this build
currently marks all AWS-plane rules `evaluated` regardless of IAM
denial, so this specific comparison may show fewer (or zero)
`not_re_evaluated` entries than the manifests-only comparison; **that
divergence between the two comparisons is itself part of what this
certification proves** — it demonstrates precisely where the
`not_re_evaluated` safety net does and does not currently reach, using
real infrastructure evidence instead of only
`internal/comparison`'s synthetic test fixtures. Because of that same
divergence, `--min-not-re-evaluated` is deliberately **not** required on
`full-vs-reduced-iam.json`'s own assertion run below — zero is a
legitimate, informative outcome for that specific comparison, not a
planning failure.

**Non-vacuous proof requirement (`full-vs-manifests-only.json` only):**
`assert-comparison.sh`'s two structural checks (every `resolved` entry
proven evaluated; every `not_re_evaluated` entry proven not-evaluated)
both pass trivially on an **empty** `not_re_evaluated` array — a
`--min-not-re-evaluated`-less run would report `OK` even if the real
cluster this certification executes against happens to produce zero
findings from any rule outside `{API-001, API-002}` (e.g. a
freshly-created, otherwise-clean sandbox cluster with no PDBs, no
webhooks, no add-on issues), which would make this PR's single most
important assertion pass without ever actually observing the
`not_re_evaluated` bucket fire on real data. `assert-comparison.sh` now
takes an optional `--min-not-re-evaluated N` flag (Stage 2 addition) for
exactly this reason — pass `1` here so the check fails loudly, instead of
passing vacuously, if the chosen cluster/fixture pairing produces no
suitable baseline findings. If it does fail this way at Stage 3/4, the
fix is a different baseline or a cluster with more real, pre-existing
findings (e.g. reusing the case study's own workload fixtures under
`demo/eks-case-study/manifests/`, applied read-only-from-kubepreflight's-
perspective as ordinary cluster state before scanning, not created *by*
this certification's own commands) — never loosening this assertion.

Automated check (already run against a real, locally-generated pair —
see `validation-summary.md` for the exact run and output; the
`--min-not-re-evaluated 1` flag itself was verified separately during
Stage 2 review — see that stage's amended commit message):

```bash
scripts/certification/assert-comparison.sh \
  --current docs/certification/v1.3.0/manifests-only/findings.json \
  --comparison docs/certification/v1.3.0/comparisons/full-vs-manifests-only.json \
  --min-not-re-evaluated 1
```

## Legacy compatibility proof (PR 7)

Fixture: `demo/eks-case-study/evidence/after-upgrade/findings.json` —
confirmed by direct inspection to be `schemaVersion: "1.0"` with no
`ruleExecutions`/`ruleExecutionsNormalized` field at all, i.e. a
genuine pre-PR-1 document, not a synthetic one. No fresh fixture is
needed.

| # | Assertion | Field/value | Source |
|---|---|---|---|
| 1 | `compare` loads it | `kubepreflight compare --baseline <legacy 1.0 file> --current <native 1.1 file>` exits 0 and produces a comparison — already confirmed locally (see `validation-summary.md`) | `internal/comparison/normalize.go`'s `LoadAndNormalize` |
| 2 | Backfilled conservatively | The legacy document's in-memory `RuleExecutionsNormalized` becomes `true` and every rule ID without a matching finding in the legacy document's own `findings` array is marked `not_evaluated` (**never** `evaluated`) — absence of a finding is not evidence of a clean pass | `internal/comparison/normalize.go`'s `normalizeRuleExecutions`, its core safety-rule doc comment |
| 3 | `rollback assess --findings` loads it | `kubepreflight rollback assess --findings <legacy 1.0 file>` passes `validateRollbackFindingsDocument` (schema `"1.0"` is explicitly accepted alongside `"1.1"`) rather than being rejected | `internal/cli/rollback.go`'s `legacyFindingsSchemaVersion` const and `validateRollbackFindingsDocument` |

## Output parity across renderers

For the same underlying `findings.Report`, Terminal, Markdown, HTML, and
Console must agree on: verdict, score, blocker/warning/info counts, and
`EvaluationCoverage`'s `CoverageLabel`/`Status` — all four renderers
call the same shared `report.BuildEvaluationCoverage(r)` (confirmed in
`internal/report/terminal.go`, `internal/report/html.go`,
`internal/report/markdown.go`), so a divergence here would indicate a
renderer-specific bug, not an evidence-collection issue. Verified by
diffing the counts each renderer prints against `findings.json`'s own
`.summary`/`.ruleExecutions` for every mode's evidence.

## Redaction/evidence-sanitization safety

See `environment.md`'s "Sanitization plan" section for the full
coverage/gap analysis. Every evidence file under `full-access/`,
`reduced-iam/`, `manifests-only/`, and `comparisons/` must pass
`scripts/certification/check-evidence-sanitized.sh` before being
committed as public evidence (Phase 6 in `commands.md`).
