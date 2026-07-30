# Post-#232 Real-Environment Certification

Date: 2026-07-29
Source commit: `6197911` (`docs: sync post-232 audit and local certification evidence`, #233)

This directory contains real-environment certification evidence closing the gap `docs/certification/post-232-local/README.md` explicitly left open: *"fresh real-cluster Kubernetes validation, reduced-RBAC service-account validation, reduced-IAM EKS validation, fresh real-EKS certification."* All of it is now covered here against a real, disposable Kind cluster (Lane 1) and a real, disposable, low-cost Amazon EKS cluster (Lane 2), using the real `kubepreflight` binary throughout — never mocks, never `go run`.

**Certification found confirmed defects. This evidence is not a claim that every tested capability passed.** See [Confirmed defects](#confirmed-defects) below and `DEFECTS.md` for the full, formally-tracked list.

## Final certification classification

> Post-#232 KubePreflight is real-cluster verified for reduced RBAC and real-EKS verified for full/reduced-IAM evidence behavior, partial-report semantics, and rollback provenance Cases A–D.

This statement is the authoritative summary of this certification's scope and must remain visible in public documentation. It is deliberately narrower than "fully certified" — the following are **explicitly NOT yet certified** by this work:

- CRD rollback target-direction mismatch (Case E).
- Webhook rollback target-direction mismatch (Case F).
- `WH-002` current-health preservation under a live, unhealthy webhook on real EKS (Case G).
- CloudTrail-level (request-level) runtime API proof of read-only behavior.
- All real-EKS upgrade contexts other than `audit-only` (`full-platform-upgrade`, `control-plane-only`, `worker-rollout` were not separately captured).

## Scope completed

| Lane | Status |
|---|---|
| 1. Disposable Kubernetes reduced-RBAC | ✅ Complete — full-access baseline, documented-baseline, Profiles A–F |
| 2. Non-production Amazon EKS reduced-IAM | ✅ Complete — full-access baseline, IAM Profiles A–F (B/C/E/F partially contaminated by an account-wide restriction outside this certification's control — precisely documented, not hidden) |
| 3. Compare certification | ✅ Complete |
| 3. Rollback directionality | ⚠️ Partial — Cases A–D complete and clean; Cases E/F/G not completed this session (disclosed) |
| 3. Partial-report / artifact-write-failure | ✅ Complete |
| Read-only verification | ✅ Complete (source + RBAC + IAM proof; CloudTrail-level runtime proof not attempted, disclosed) |
| Redaction certification | ✅ Complete — found two real gaps |
| Cleanup | ✅ Complete — independently verified, zero orphaned resources |
| Repo validation (tests, vet, gofmt, catalogs, web tests) | ✅ Complete — all green |

## Directory layout

```
DEFECTS.md                Formally tracked defect index (RED-TERMINAL-001, RED-CLOUD-ID-002, RBAC-DOC-001, RBAC-STALE-003)
01-reduced-rbac/        Lane 1: Kind cluster, full-access + documented + 6 profiles
02-reduced-iam-eks/      Lane 2: real EKS, full-access + 6 IAM profiles
03-full-real-eks/         Lane 3: full real-EKS scan, artifact-write-failure
04-compare/                Lane 3: compare certification (not_re_evaluated, mismatches)
05-rollback/                 Lane 3: rollback directionality Cases A-D
06-read-only/                  Source/RBAC/IAM proof of no mutation
07-redaction/                    Redaction certification, 2 real gaps found
08-cleanup/                        Full cleanup verification, all lanes
```

## Test-environment mutation vs. product behavior

Every mutation performed anywhere in this certification — Kind cluster create/delete, RBAC fixture create/delete, EKS cluster/node-group create/delete, IAM user/policy/access-entry create/delete — was performed **directly by this certification's own harness, via `kubectl`/`eksctl`/`aws` CLI commands**, never by invoking the `kubepreflight` binary itself. The binary was only ever invoked with `scan`, `compare`, `rollback plan`, and `rollback assess` — all read-only by contract, and independently confirmed read-only by source inspection (`06-read-only/report.md`). This distinction is load-bearing: certification setup/teardown mutating disposable infrastructure it created itself is expected and safe; the product mutating anything would be a certification failure. No such product mutation occurred anywhere in this session.

## Confirmed defects

Formally tracked with IDs, severities, and acceptance criteria in `DEFECTS.md`. Summary:

1. **`RED-TERMINAL-001`** (P1/High, real-EKS verified) — terminal (`stdout`) output is not redacted at all, even for patterns that do exist, contradicting `--redact-sensitive-identifiers`' own documented promise to cover "terminal" output. The most important finding in this certification; should be fixed before the next public release.
2. **`RED-CLOUD-ID-002`** (P2/Medium, real-EKS verified) — no redaction pattern exists for VPC/Security-Group/Subnet/Instance/Volume IDs. Recommended for the same PR as `RED-TERMINAL-001`, tracked and tested independently.
3. **`RBAC-DOC-001`** (P2/Medium, real-cluster verified) — the documented minimal RBAC (`deploy/clusterrole.yaml`), applied verbatim, can never reach a clean report: `DRAIN-002`/`API-001`/`API-002` permanently read `insufficient_evidence` due to missing PV/PVC/PSP/CSIStorageCapacity grants. This is a product-contract question with three valid resolution paths (see `DEFECTS.md`) — not to be resolved by silently granting more permissions without reviewing whether each resource is genuinely required.
4. **`RBAC-STALE-003`** (P3/Low, real-cluster verified) — `assert-findings.sh`'s 6-rule `insufficient_evidence` allowlist and a code comment referencing a removed symbol (`ruleErrorsMapKeys`) both understate current behavior; all 31 rules now support per-rule evidence-dependency tracking. Lowest priority — do not let it distract from `RED-TERMINAL-001`.

**Not classified as defects, but explicitly flagged as not fully understood** (see `DEFECTS.md`'s closing section): the Lane 1 Profile B anomaly (why `API-001`/`API-002`/`DRAIN-001` also degrade when only PDB access is denied — plausible but not source-traced) and the Lane 1 Profile C/D conditional-dependency result (confirmed correct only for the specific case tested — an empty webhook/CRD inventory; does **not** certify the activated dependency path, where a live Service-referencing webhook or conversion CRD is actually present).

**Account-wide AWS restriction, not a product defect** (`02-reduced-iam-eks/report.md`): this specific AWS account blocks `eks:ListAddons`/`ListNodegroups`/`ListInsights` regardless of identity policy (confirmed via `simulate-principal-policy` saying "allowed" while the real call 403s), and separately blocks non-free-tier EC2 instance types on Spot. Both are environmental, disclosed for transparency, and the product behaved perfectly honestly around both.

These defects were not fixed by the original evidence-only certification commit. A later local implementation pass marks them `fixed locally` in `DEFECTS.md`; that status does not imply a new real-EKS recertification unless a separate evidence lane records one.

## Corrected capability labels

- **Compare semantics: real-binary verified using real-EKS-derived reports.** Compare itself never queries Kubernetes or AWS — it consumes `findings.json` pairs, one or both of which happened to be produced from real-EKS scans in this certification. See `04-compare/report.md`.
- **Rollback provenance and freshness behavior real-EKS verified; CRD/webhook target-direction cases remain locally verified and pending real-EKS certification.** Cases A–D (identity matching, fresh evidence, wrong-identity, stale-evidence handling) are real-EKS certified. Cases E/F/G (CRD/webhook target-direction mismatch, live current-health preservation) are covered at the unit-test level only — not completed this session. See `05-rollback/report.md`.
- **APIService evidence**: `APISERVICE-001` does use Kubernetes-reported availability status as real evidence — it is not evidence-free. The correct limitation statement is: *"No request-level or TLS proof for APIService; assessment relies on Kubernetes-reported availability status."* ("No APIService status-based evidence" would be inaccurate and was never written into any evidence file in this directory.)
- **Runtime request-level (CloudTrail) proof: not evaluated** this session — disclosed limitation, not claimed. See `06-read-only/report.md`.
