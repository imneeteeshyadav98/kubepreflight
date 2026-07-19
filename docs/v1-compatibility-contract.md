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
| 1 | `PASSED_WITH_WARNINGS` | Complete evidence with warnings only |
| 2 | `BLOCKED` | Complete evidence with one or more Blocker findings |
| 3 | `INCOMPLETE` | One or more evidence planes were partial; rerun after fixing coverage |
| 4 | infrastructure failure | No trustworthy report was produced before evidence collection completed |

Incomplete evidence outranks findings in the top-level result. If a partial
scan observes blockers, the blockers remain visible in `findings`, but the
result and exit code remain `INCOMPLETE`/3 because the assessment is not fully
trusted.

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

- scan findings JSON: `1.0`
- plan JSON: `1.0`
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
are promoted through a tested migration.

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
- `CriticalInfra` escalates lower-priority findings to at least `P2`.
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
