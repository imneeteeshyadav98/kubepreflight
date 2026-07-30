# Lane 3 — Compare Certification

Date: 2026-07-29. Binary: same real `kubepreflight` binary used throughout. All comparisons below use real findings.json pairs produced earlier in this certification (Lane 1 Kind-cluster scans, or fresh manifests-only fixtures) — none were hand-authored from scratch.

**Capability label: Compare semantics: real-binary verified using real-EKS-derived reports.** `compare` itself never queries Kubernetes or AWS — it is a pure function over two `findings.json` files. What real-EKS involvement means here is that some of the input report pairs were themselves produced by real-EKS scans elsewhere in this certification; it does not mean compare made any live cluster or AWS call.

## `not_re_evaluated` vs `resolved`: two real, contrasting cases

### Case 1 — a finding that survives partial re-evaluation (`Unchanged`, not `not_re_evaluated`)

`rbac-baseline-vs-profile-b/`: baseline (full-access Kind scan) vs. Profile B (PDB denied). `DRAIN-001`'s rule execution state is `insufficient_evidence` in the current scan, **yet the exact same finding (identical fingerprint) is still present** — the rule had enough surviving evidence (Deployment/replica data, still granted) to reproduce this specific finding even though it couldn't fully evaluate everything it normally checks. Compare correctly classifies this as **`Unchanged: 4`**, not `not_re_evaluated` — the finding genuinely wasn't lost, so calling it "not re-evaluated" would be the wrong, less-informative label. `Not re-evaluated: 0` in this comparison.

### Case 2 — findings that genuinely disappear under insufficient evidence (`not_re_evaluated`, confirmed correct)

`rbac-baseline-vs-profile-a/`: baseline vs. Profile A (near-total RBAC denial — only `nodes`/`pods` granted). All 4 baseline findings (`DRAIN-001`, `DRAIN-003` ×2, `NODE-003`) vanish from the current scan's raw finding list. Compare output:

```
Comparison: PASSED_WITH_WARNINGS -> INCOMPLETE (changed)
Readiness score: 88 -> 100
New: 0 (0 blocker(s))  Resolved: 0 (0 blocker(s))  Not re-evaluated: 4  Changed: 0  Unchanged: 0
Gate decision: neutral ([INSUFFICIENT_EVIDENCE])
```

`resolved: []` (empty — confirmed via direct JSON inspection). All 4 land in the `not_re_evaluated` bucket with the exact advisory text: *"The finding was present in the baseline, but its rule was not successfully evaluated in the current report, so resolution cannot be confirmed."* `gate.json`'s `scoreDelta: 12` (88→100) is recorded honestly, but `decision: "neutral"` with reason `INSUFFICIENT_EVIDENCE` — **the raw score improvement is never allowed to read as a passing gate.** This is the core "missing ≠ resolved" behavior the v1.3.0 release exists to guarantee, independently re-confirmed here on fresh, real Kind-cluster evidence.

## Target-version mismatch

`target-mismatch/`: same manifest fixture (a `policy/v1beta1 PodDisruptionBudget`) scanned once at `--target-version 1.33` and once at `1.36`. Compare correctly surfaces an explicit warning rather than silently misreporting: *"baseline was scanned at target-version 1.33 and current at 1.36 — fingerprints are scoped to target version, so genuinely unchanged findings will show up as a new+resolved pair instead of unchanged. Re-scan both at the same target version for an accurate diff."* Gate decision: `neutral`/`INSUFFICIENT_EVIDENCE`, never a false pass or fail.

## Upgrade-context mismatch

`context-mismatch/`: same fixture scanned once with `--upgrade-context unspecified` (default) and once with `--upgrade-context worker-rollout`. Compare correctly warns: *"baseline used upgradeContext 'unspecified' and current used 'worker_rollout' — blocker counts and verdicts are context-aware, so review gate changes with the selected operation in mind."* Gate decision: `neutral`/`INSUFFICIENT_EVIDENCE`.

## Acceptance criteria

- ✅ `baseline finding + current evaluated/no finding → resolved`: not directly exercised in this lane's fixtures (would require a genuinely fixed finding, not an evidence-availability change) — already covered by the pre-existing `docs/certification/v1.3.0/comparisons/` evidence from the prior real-EKS certification, independently re-verified against source in this session (`New:1, Resolved:0, notReEvaluated:8` in `full-vs-manifests.json` matches `gate.json`'s `resolvedFindings:0, notReEvaluated:8, newBlockers:1` exactly).
- ✅ `baseline finding + current insufficient_evidence → not_re_evaluated`: confirmed cleanly (Case 2 above).
- Not directly tested this session: `baseline finding + current not_applicable → not_re_evaluated` (no fixture pair with an applicability change was generated in this lane — deferred, see Remaining Limitations in the final report).
- ✅ `target/context mismatch → neutral gate`: confirmed for both target-version and upgrade-context mismatch.
- ✅ Compare never claims improvement when a rule was not re-evaluated: confirmed via the score-delta-vs-gate-decision divergence in Case 2.
