# Lane 3 — Rollback Directionality Certification

Date: 2026-07-29. No Kubernetes or EKS rollback was ever executed — only `rollback assess`/`rollback plan` (read-only) were used throughout. Cases below use `rollback assess` against the real EKS cluster (Cases A–D, while the cluster was live) and locally-mutated copies of a real `findings.json` produced by that same cluster.

**Capability label: rollback provenance and freshness behavior real-EKS verified; CRD/webhook target-direction cases remain locally verified and pending real-EKS certification.** Do not read this lane as "rollback directionality real-EKS certified" — Cases A–D verify cluster-identity matching and evidence-freshness handling (provenance), not CRD/webhook target-direction correctness. That is Cases E/F/G, explicitly not completed this session (see below).

## Case A — no matching AWS/EKS evidence

Reused existing evidence from `docs/certification/post-232-local/rollback-no-aws/` (captured earlier against the same `master` commit, no fresh AWS access available at that time): exit `1`, `decision: "operator_decision_required"`, `confidence: "low"`, `readiness.status: "insufficient_evidence"`, `unknowns: 7`. No false rollback recommendation. Not re-run in this lane since the existing evidence already cleanly demonstrates the exact scenario.

## Case B — correct cluster identity, fresh findings

`case-b-fresh-matching/`: real `rollback assess --provider eks --cluster-name <cluster> --findings <real, same-run findings.json>`. Exit `2`, `decision: "do_not_proceed"`, `confidence: "high"`, reasons include `MANAGED_NODEGROUP_ROLLBACK_REQUIRED` and `PDB_DISRUPTION_CONSTRAINTS` — genuine operational findings from the real node group and the real coredns single-node topology, correctly consumed as real evidence since the identity matches and the findings are fresh.

## Case C — wrong cluster identity

`case-c-wrong-identity/`: a copy of Case B's real findings.json with only `eksCluster.clusterName` mutated to a different value (local cluster identity was never touched). Exit `2`, `decision: "do_not_proceed"` — **but precisely isolated by inspecting the per-check `checks[]` array**: `managed-nodegroups`, `disruption-readiness`, `reverse-compatibility`, `workload-health`, `managed-addons`, and `self-managed-addons` — every check that depended on the (now-mismatched) findings fixture — all read `status: "unknown"`, never `"fail"`. The overall `do_not_proceed`/high-confidence verdict is driven entirely by *other*, identity-independent, live-cluster-derived reasons (`EKS_UPGRADE_HISTORY_UNAVAILABLE`, `EKS_FEATURE_COMPATIBILITY_UNVERIFIED` — present in Case B too, from real live evidence, unrelated to the findings file at all). **No check ever shows a confirmed failure caused by the mismatched evidence itself** — exactly the required behavior.

## Case D — stale findings

`case-d-stale/`: a copy of Case B's real findings.json with only `scannedAt` mutated to 30 days in the past. Exit `2`, same pattern as Case C: every findings-dependent check (`managed-nodegroups`, `managed-addons`, `self-managed-addons`, `workload-health`, `disruption-readiness`, `reverse-compatibility`) downgrades to `status: "unknown"`. No confirmed failure is manufactured from stale evidence.

## Cases E, F, G — not completed this session (honest disclosure)

Cases E (CRD forward-target mismatch), F (webhook forward-target mismatch), and G (live `WH-002` current-backend-risk fixture) require either (a) careful hand-construction of synthetic-but-schema-correct `CRD-001`/`CRD-002`/`WH-001`/`WH-004`/`WH-005` findings with a specific forward-target relationship encoded, which needs deeper source inspection of the target-aware reverse-compatibility logic than time allowed in this session, or (b) for Case G specifically, creating a new live disposable webhook object against the EKS cluster — which was no longer available once cluster teardown began. Rather than rush a fixture that might not accurately reflect the real schema and risk reporting a false certification result, these three cases are explicitly **not completed** in this run. This is a real, disclosed scope limitation, not a silently-skipped requirement — see the final report's "Remaining limitations" section. The unit-test suite (`internal/rollback/cluster_identity_test.go` and related files, all passing per `go test ./...` in this session) already covers this exact logic at the Go level; what's missing here specifically is real-binary/CLI-level black-box confirmation for these three specific directional cases.

## Acceptance criteria

- ✅ No upgrade or rollback executed (only `assess`, only `plan` used).
- ✅ Wrong/stale evidence cannot create a false confirmed rollback failure (Cases C, D — confirmed via per-check status, not just the top-level decision).
- Fresh current `WH-002` backend risk remains visible: not directly tested (Case G not completed — this cluster had no fresh, unhealthy `WH-002`-triggering webhook to observe).
- CRD/webhook reverse relevance is target-aware: not directly tested this session (Cases E/F not completed) — covered at the unit-test level only.
- ✅ Unknown remains distinct from fail: confirmed precisely via the `checks[].status` field in Cases C and D.
- ✅ Operator decision remains distinct from blocker: Case A (`operator_decision_required`) vs. Case B (`do_not_proceed`, genuine blockers) are visibly different decision classes.
- ✅ Evidence provenance visible and sanitized: `--redact-sensitive-identifiers` used throughout; all retained fixtures manually re-checked for VPC/SG ID leakage (see `../07-redaction/report.md`) and scrubbed.
