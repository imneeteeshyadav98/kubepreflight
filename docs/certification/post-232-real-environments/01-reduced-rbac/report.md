# Lane 1 — Disposable Kubernetes Reduced-RBAC Certification

Date: 2026-07-29
Source commit: `6197911` (`docs: sync post-232 audit and local certification evidence`, #233)
Environment: Kind v0.27.0, cluster `kp-cert-rbac`, Kubernetes `v1.32.2` (Docker 29.4.0)
Binary: real `kubepreflight` built from `master` at the commit above (`go build ./cmd/kubepreflight`), never `go run`.

All scans used `--target-version 1.33 --upgrade-context audit-only`. Every identity below is a dedicated `ServiceAccount` + `ClusterRole` + `ClusterRoleBinding` created for this certification only, authenticated via a 15-minute `kubectl create token` — never the cluster-admin kubeconfig context, and never a long-lived credential retained anywhere.

## Full-access baseline

Real cluster-admin scan against the live Kind cluster: `verdict PASSED_WITH_WARNINGS`, `readinessScore 88`, exit `1`, 4 real warnings (node-label and single-node drain-headroom findings genuinely produced by this exact 1-node Kind topology — not fabricated). `coverage.kubernetes.status: complete`. See `00-full-access-baseline/`.

## Two confirmed findings from this lane

Both are real, reproduced twice (every profile run below reproduces them independently), and neither was forced into a pass.

### Finding 1 — `scripts/certification/assert-findings.sh` and a code comment are stale post-#232

`assert-findings.sh`'s `insufficient_evidence_capable_rule_ids` allowlist (6 rules: `WH-002, PDB-001, PDB-002, CRD-001, CRD-002, APISERVICE-001`) and the doc comment in `internal/report/evaluation_coverage.go` (~line 264, "only 6 of the ~30 registered rules... `ruleErrorsMapKeys`") both describe a narrower mechanism than what `master` actually implements today. `grep -oE "func \([A-Za-z0-9]+\) EvidenceDependencies\(\)" internal/rules/execution.go` shows **all 31 registered rules** now implement `EvidenceDependencies()`, and `ruleErrorsMapKeys` no longer exists as a symbol anywhere in the repository — only in that one comment. Live evidence: Profile A alone drove 17 distinct rule IDs to `insufficient_evidence`, far beyond the 6-rule allowlist.

Severity: **Low** (documentation/tooling staleness — the actual behavior is correct and more complete than the stale comment describes; `assert-findings.sh` would currently produce false `FAIL`s on its own `bad_insufficient` check if run against current `master` output). Not fixed here per certification scope. Tracked as `RBAC-STALE-003` (P3) — see `../DEFECTS.md`.

### Finding 2 — the documented minimal RBAC (`deploy/clusterrole.yaml`) can never reach full coverage

Applied `deploy/clusterrole.yaml` verbatim (see `profile-documented-baseline/`) — exit **3**, `verdict INCOMPLETE`, even though this is the project's own documented "copy-pasteable RBAC for kubepreflight." Root cause, isolated precisely:

- `DRAIN002.EvidenceDependencies()` (`internal/rules/execution.go:232`) requires `persistentvolumes` and `persistentvolumeclaims` — neither is granted anywhere in `deploy/clusterrole.yaml`.
- `API-001`/`API-002`'s deprecated-API dynamic-client sweep additionally needs `extensions/v1beta1 podsecuritypolicies` (the doc grants `policy/podsecuritypolicies` but not the `extensions`-group variant) and `storage.k8s.io/v1beta1 csistoragecapacities` (the doc grants `csidrivers/csinodes/storageclasses/volumeattachments` but not this one).

Net effect: **`DRAIN-002`, `API-001`, and `API-002` can never reach `evaluated` under the documented RBAC, no matter how healthy the cluster is** — every real deployment following the docs exactly will see a perpetual `INCOMPLETE`/exit-3 report. This is not a false pass or false blocker (no finding is fabricated or hidden), but it does mean the coverage signal itself is permanently degraded for anyone who trusts the docs as sufficient.

Severity: **Medium** (correctness of individual findings is unaffected; the coverage/trust signal is what's broken, and exit 3 in CI would fire on every run regardless of actual readiness). Not fixed here per certification scope. Tracked as `RBAC-DOC-001` (P2) — see `../DEFECTS.md` for the three valid resolution directions; do not resolve by silently granting more permissions without reviewing whether each resource is genuinely required.

## Lane 1 results

| RBAC profile | Missing evidence | Affected rules | Coverage | Result | Exit |
|---|---|---|---|---|---:|
| Full access (cluster-admin) | none | none | complete | PASSED_WITH_WARNINGS | 1 |
| Documented baseline (`deploy/clusterrole.yaml` verbatim) | PV/PVC, ext/PSP, csistoragecapacities (doc gaps, see Finding 2) | API-001, API-002, DRAIN-002 | partial | INCOMPLETE | 3 |
| A — discovery only (nodes+pods) | everything else | 17 rule IDs incl. API-001/002, WH-001/002/004/005, DRAIN-001..005, PDB-001/002, NODE-003, WORKLOAD-001, ADDON-001/002, CRD-001/002, APISERVICE-001 | partial | INCOMPLETE | 3 |
| B — PDB denied | poddisruptionbudgets | API-001, API-002, DRAIN-001, PDB-001, PDB-002 | partial | INCOMPLETE | 3 |
| C — webhook backend evidence denied | services, endpointslices | API-001, API-002 only — **WH-002 stayed `evaluated`** (no live webhook referenced a Service in this cluster, so the conditional dependency never activated) | partial | INCOMPLETE | 3 |
| D — CRD conversion backend evidence denied | services, endpointslices | API-001, API-002 only — **CRD-002 stayed `evaluated`** (no live CRD used webhook conversion strategy) | partial | INCOMPLETE | 3 |
| E — APIService denied | apiservices | API-001, API-002, APISERVICE-001 | partial | INCOMPLETE | 3 |
| F — workloads/pods denied | pods | DRAIN-001, DRAIN-002, DRAIN-003, DRAIN-004, PDB-002, WORKLOAD-001 (PDB-001 correctly unaffected — it doesn't depend on pods) | partial | INCOMPLETE | 3 |

Profiles C and D are the most important results in this lane: they directly prove the product's conditional evidence-dependency design (`WH002.EvidenceDependenciesFor` / `CRD002.EvidenceDependenciesFor`) works exactly as intended — denying the *backend-health* evidence plane (Services/EndpointSlices) only degrades `WH-002`/`CRD-002` when a live object actually needs that evidence to reach a verdict. An empty webhook/CRD inventory never manufactures a false `insufficient_evidence`.

**Caveat on Profiles C/D — this confirms only the untriggered path.** This cluster had no live webhook referencing a Service and no live CRD using webhook conversion, so `WH-002`/`CRD-002` never actually needed the denied evidence in this run. This certifies that an *inactive* dependency correctly does not manufacture a false `insufficient_evidence` — it does **not** certify the *activated* dependency path (a cluster with a real Service-referencing webhook or conversion CRD present, where `WH-002`/`CRD-002` should correctly go `insufficient_evidence` once Services/EndpointSlices are denied). That path was not exercised on either cluster used in this certification. See `../DEFECTS.md`.

**Caveat on Profile B — not fully explained.** Denying only `poddisruptionbudgets` also drove `API-001`, `API-002`, and `DRAIN-001` to `insufficient_evidence`, not just `PDB-001`/`PDB-002`. A plausible explanation is that the deprecated-API sweep (`API-001`/`API-002`) shares the same underlying `policy/v1beta1 PodDisruptionBudget` resource type, and `DRAIN-001` may itself consult PDB evidence as part of its drain-headroom calculation — but this has not been confirmed against `execution.go`'s `EvidenceDependencies()` for `DRAIN001`/`API001`/`API002` line-by-line, the way it was for the PDB/webhook/CRD rules above. Treat this row as needing a source-level pass, not as a fully understood result. See `../DEFECTS.md`.

## Lane 1 acceptance criteria

1. ✅ Successful list returning zero resources → `evaluated` (e.g. `NODE-001`, `COREDNS-001`, `WH-002`/`CRD-002` in Profiles C/D).
2. ✅ Permission-denied required evidence → `insufficient_evidence` (every profile above).
3. ✅ Independent rules continue (e.g. `WH-001`/`WH-002` in Profile B; `NODE-001` in every profile).
4. ✅ No applicable dependent rule falsely appears `evaluated` — none observed across 7 runs.
5. ✅ Result is `INCOMPLETE` on every reduced-evidence run.
6. ✅ Exit code is `3` on every reduced-evidence run.
7. ✅ Reports written (`findings.json`, `report.md`, `report.html`) for every run — confirmed present.
8. ✅ `upgradeReadiness.upgradeContinue == false` on every reduced-evidence run.
9. Compare-as-`not_re_evaluated` behavior: deferred to Lane 3 (`04-compare/`).
10. ✅ No KubePreflight mutation: pod count/identity in `kube-system` unchanged (9 pods, same names) across every run; RBAC only ever granted `get`/`list`, making write calls impossible at the token level, not just by product intent.

## Cleanup

- All `kp-cert-*` ServiceAccounts, ClusterRoles, ClusterRoleBindings, and the `kube-system` CoreDNS Roles/RoleBindings deleted.
- All temporary kubeconfigs under `<CERT_WORKDIR>/rbac/kubeconfig-*.yaml` deleted immediately after each run (none retained).
- Kind cluster `kp-cert-rbac` deleted; deletion independently verified via `kind get clusters`.
- See `../08-cleanup/report.md` for the full cross-lane cleanup log.
