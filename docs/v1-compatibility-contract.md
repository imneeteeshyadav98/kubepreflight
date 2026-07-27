# KubePreflight v1 compatibility contract

This document defines the surfaces KubePreflight treats as stable for v1. It
does not promise bug-for-bug compatibility. It promises that automation can
depend on the documented command names, flags, exit codes, schema identifiers,
finding IDs, priorities, fingerprints, ordering, and conservative incomplete
evidence behavior unless a future release follows the deprecation policy below.

The executable checker is:

```bash
./scripts/check-v1-compatibility-contract.sh
```

CI runs the same check through `cmd/v1compatcheck`.

## Stable CLI surface

The stable v1 command paths are:

- `kubepreflight scan`
- `kubepreflight plan`
- `kubepreflight compare`
- `kubepreflight rollback plan`
- `kubepreflight rollback assess`
- `kubepreflight version`

No command aliases are part of the v1 contract. Adding an alias is additive,
but removing or repurposing a command path is breaking.

Stable flags and defaults are locked by `internal/v1compat`. The required
operator inputs are:

- `scan --target-version`
- `plan --to-version`
- `compare --baseline`
- `compare --current`
- `rollback plan --provider=eks`
- `rollback plan --cluster-name`
- `rollback assess --provider=eks`
- `rollback assess --cluster-name`

Provider-specific flags that are recognized but not implemented for enrichment
remain documented as intentionally unavailable surfaces. The read-only product
model is part of the contract: stable commands must not mutate Kubernetes,
cloud, or local infrastructure as part of scan, plan, compare, or rollback
assessment.

## Exit codes and results

`scan` and `plan` use the same result priority order for the real immediate
assessment:

| Exit code | Result | Meaning |
|---:|---|---|
| 0 | `CLEAN` | Complete evidence and no findings that require review |
| 1 | `PASSED_WITH_WARNINGS` | Complete evidence with warnings or operator decisions only |
| 2 | `BLOCKED` | Complete evidence with one or more findings whose effective upgrade gate is `block` |
| 3 | `INCOMPLETE` | One or more evidence planes were partial; rerun after fixing coverage |
| 4 | infrastructure failure | No trustworthy report was produced before evidence collection completed, OR evidence collection and assessment succeeded but a requested persistent report/artifact could not be created or fully written (output directory creation failed, or a `findings.json`/`report.md`/`report.html`/`upgrade-plan.json`/action-plan file could not be written) |

Incomplete evidence outranks findings in the top-level result. If a partial
scan observes blockers, the blockers remain visible in `findings`, but the
result and exit code remain `INCOMPLETE`/3 because the assessment is not fully
trusted.

A requested output artifact that cannot be durably written is treated the
same as a failed evidence collection: exit code 4, regardless of what the
underlying assessment's own result/exit code would otherwise have been. This
never changes the assessment itself (findings, scores, verdicts) — only the
process exit code, and only when the write genuinely failed. When `--output
all` (or an equivalent multi-format rollback output) writes some formats
successfully before a later one fails, the earlier files are left in place
(no atomic rollback of partial output) and the command still exits 4.

`scan` and `plan` accept `--upgrade-context` with stable values
`unspecified`, `audit-only`, `control-plane-only`, `worker-rollout`,
`full-platform-upgrade`, and `workload-restart`. New findings may include
`impactScopes` and `upgradeGate`; blocker counts use the effective upgrade gate,
not raw severity alone. EKS control-plane provider preconditions block only for
`control-plane-only` and `full-platform-upgrade`; under `unspecified` they
require operator decision instead of assuming a control-plane operation. For
older findings that do not include `upgradeGate`,
the effective gate remains backward-compatible: `GlobalBlocker` or
`Severity: Blocker` behaves as `block`, and all other findings behave as
`allow`.

`compare` exits 0 after valid comparison output unless `--gate-out` is used and
the gate decision is `fail`, in which case it exits 1. A neutral gate decision
does not fail CI because neutral means insufficient evidence, not a proven
regression.

`rollback plan` and `rollback assess` currently map rollback recommendations as
follows:

| Exit code | Recommendation |
|---:|---|
| 0 | `rollback_preferred` |
| 1 | `fix_forward_preferred` or `operator_decision_required` |
| 2 | `do_not_proceed` |

## Stable JSON schemas

Stable v1 schema identifiers:

- scan findings JSON: `1.1`
- plan JSON: `1.1`
- action plan JSON: `kubepreflight.io/upgrade-action-plan/v1`
- comparison JSON: `kubepreflight.io/scan-comparison/v1`
- API catalog: `apicatalog.kubepreflight.io/v1`
- add-on compatibility catalog: `compatcatalog.kubepreflight.io/v1`

The EKS rollback assessment schema remains:

```text
kubepreflight.io/rollback-assessment/v1alpha1
```

Rollback assessment behavior, command availability, exit-code mapping, and
reason-code validation are tested and documented, but the rollback JSON schema
is explicitly excluded from the stable v1 schema guarantee until its semantics
are promoted through a tested migration. The rollback schema, and the
`--findings` input-document validation rollback plan/assess perform, are
unaffected by the `1.1` bump below: that validation has always required an
exact match to the build's own current `findings.SchemaVersion` (never a
broader "any known past version" acceptance), a pre-existing, unrelated
invariant this PR does not change.

### Findings/plan schema `1.1` (v1.3.0)

`1.1` is a purely additive bump over `1.0`, executed as v1.3.0's PR 7 per
the deprecation policy below. Nothing in `1.0` was renamed, retyped, or
removed; two fields were added to `Report`:

- `ruleExecutions` (array of `RuleExecutionRecord`, omitted when empty/nil):
  one record per rule ID in this build's rule universe, each with
  `ruleId` (string), `applicability` (`applicable` or `not_applicable`),
  `state` (`evaluated`, `not_evaluated`, `insufficient_evidence`, or
  `failed`), and an optional free-text `reason`.
- `ruleExecutionsNormalized` (bool, omitted when false): true only when
  `ruleExecutions` was backfilled from a legacy pre-`1.1` document by
  `comparison.LoadAndNormalize`, rather than computed natively during the
  scan that produced the report.

**Old and new behavior.** A `1.0` document (no `ruleExecutions` field at
all) remains fully readable: `encoding/json` already ignores fields it
doesn't have and leaves the two new fields at their zero value, and
`comparison.LoadAndNormalize` additionally backfills `ruleExecutions` for
such a document so downstream comparison/scorecard logic has something to
read. A `1.1` document produced by this build always carries
`ruleExecutions` natively (see `internal/rules/rule.go`'s
`RunAllWithExecutions`) and never sets `ruleExecutionsNormalized`.

**Migration guidance.** Nothing needs to change for a consumer that only
reads `1.0`-era fields — they are untouched. A consumer that wants
per-rule evaluation-coverage data should read `ruleExecutions` (present
natively on any `1.1`+ report) and check `ruleExecutionsNormalized` to
tell native data apart from a backfilled inference before treating it as
this scan's own evidence.

**Conservative normalization guarantee.** The absence of rule-execution
metadata in a `1.0` document is never read as "evaluated clean." When
`comparison.LoadAndNormalize` backfills `ruleExecutions` for a legacy
document, a rule ID is only ever marked `evaluated` when that document
actually contains a finding from it; every other rule ID — including one
that would have run cleanly with zero findings — is marked `not_evaluated`,
never `evaluated`. Every backfilled record sets `ruleExecutionsNormalized:
true` so a consumer can always tell inferred data from this scan's own
evidence.

**Tests for both `1.0` and `1.1` inputs:** `internal/findings/report_test.go`
(native `1.1` output, JSON round-trip), `internal/comparison/normalize_test.go`
(legacy `1.0` fixtures through `LoadAndNormalize`, including the
zero-findings safety invariant), and `internal/v1compat/contract_test.go`
(structural checks against the real Go types/constants, not hardcoded
strings) all exercise this explicitly.

**Compatibility checker updated:** `internal/v1compat.StableScanSchemaVersion`
is `"1.1"`, and the checker additionally locks `RuleApplicability`/
`RuleExecutionState`'s wire values, `RuleExecutionRecord`'s JSON field
names, and `Report`'s full JSON field list (the `1.0`-stable fields plus
the two `1.1` additive ones) — see `internal/v1compat/contract.go`'s
`checkRuleExecutionContract`.

Note: `internal/gate.Result`'s `EvaluationCoverage`/`EvaluationAdvisories`
fields (added alongside this work) live on the separate
`kubepreflight.io/comparison-gate/v1` gate-result document, not on
`findings.Report` — they are not part of, and do not participate in, the
findings schema version described here.

## Finding IDs, priorities, and fingerprints

Registered rule IDs are stable v1 identifiers. Adding a new rule ID is allowed
only when the new rule has explicit priority, scorecard category, schema, docs,
and tests. Renaming or reusing an existing rule ID for different semantics is
breaking.

Finding priority values are stable:

- `P1`
- `P2`
- `P3`
- `P4`

The default rule-ID-to-priority mapping is locked by
`cmd/v1compatcheck`. Dynamic evidence-based overrides remain part of the
contract:

- `GlobalBlocker` escalates to `P1` and `affectedScope: global`.
- `CriticalInfra` escalates lower-priority findings to at least `P2`, except
  `NODE-003`, which remains a Warning/P3 until contextual replacement evidence
  exists.
- `ADDON-002` with `compatibility status: upgrade recommended` remains `P4`
  rather than the ordinary ADDON-002 `P3`.

`FingerprintV2` uses the `finding-v2` domain with rule ID, target version,
optional discriminator, and sorted resource concept keys. A fingerprint is
scoped to the target version; comparing scans from different target versions is
not a stable identity operation.

## Unknown and insufficient evidence

KubePreflight must not guess safety from missing evidence.

- Unknown catalog lookups remain unknown or unverifiable; absence is not
  compatibility.
- Missing provider enrichment is coverage behavior, not proof of safety.
- Missing current Kubernetes version does not produce a downgrade conclusion.
- Unsupported target versions outside this build's reviewed catalog range are
  rejected before collection.
- Manifest-only scans must not imply live-cluster or provider safety.

The supported Kubernetes target range for this build is:

```text
1.25-1.39
```

## Deterministic ordering

Automation may rely on deterministic ordering for:

- registered rule ID list
- rendered checker output
- sorted fingerprints and concept keys
- report and comparison finding ordering where the renderer defines a stable
  priority or severity order
- catalog and governance checker output

Nondeterministic map iteration must not leak into stable JSON or checker output.

## Intentionally unstable surfaces

The following are not stable v1 contract surfaces:

- HTML/CSS class names and DOM structure, except where end-to-end tests lock a
  user workflow
- prose-only copy in terminal, Markdown, and HTML reports
- Console internal React component structure
- benchmark wall-clock numbers
- unpublished scripts under development
- `kubepreflight.io/rollback-assessment/v1alpha1` JSON field shape
- future AKS/GKE enrichment behavior beyond current recognized-but-unavailable
  flag validation

## Deprecation policy

A future change that removes or changes a stable v1 surface must:

1. document the old and new behavior;
2. add migration guidance;
3. preserve backward-compatible reading where feasible;
4. add tests for old and new inputs during the transition;
5. fail the compatibility checker until the contract is deliberately updated.

Security fixes may tighten unsafe behavior faster, but must still document the
change and prefer conservative visible findings over silent compatibility.
