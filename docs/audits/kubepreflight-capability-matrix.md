# KubePreflight Capability Matrix

Date: 2026-07-29

Status labels:

- Strong: implemented, tested, and validated in this audit or repository certification evidence.
- Partial: useful coverage exists, but not complete for all production cases.
- Gap: not implemented as a direct capability.
- Out of scope: intentionally not a KubePreflight responsibility today.

| Capability | Status | Evidence | Notes |
| --- | --- | --- | --- |
| Read-only Kubernetes collection | Strong | `internal/collectors/k8s` | Uses Kubernetes API snapshot collection; no product mutation found. |
| Read-only EKS/AWS enrichment | Strong | `internal/collectors/aws`, `internal/rollback/eks` | Uses `Describe*`/`List*`; no `StartInsightsRefresh`. |
| Raw manifest deprecated API scan | Strong | `internal/collectors/manifest`, `API-001`, `API-002` | Manifest-only sample passed/blocked correctly. |
| Helm chart render-and-scan | Partial | `--helm-chart`, manifest collector | Depends on local `helm template`; not deeply revalidated in this audit. |
| Deprecated/removed Kubernetes APIs | Strong | `API-001`, `API-002`, API catalog | 44 catalog entries, catalog script pass. |
| Admission webhook static risk | Strong | `WH-001`, `WH-002`, `WH-004`, `WH-005` | Strong structural/readiness checks; no admission simulation. |
| Admission webhook live dry-run proof | Gap | No product dry-run write path | Would conflict with strict read-only unless opt-in. |
| PDB disruption constraints | Strong | `PDB-001`, `PDB-002` | Context-aware gating and rollback routing present. |
| Drain readiness | Strong | `DRAIN-001` to `DRAIN-005` | Covers local PV, singleton, unmanaged pods, pressure/taints, zero-ready workloads. |
| Node/kubelet version skew | Strong | `NODE-001` | Universal blocker behavior preserved. |
| Deprecated master label dependency | Strong | `NODE-003` | Warning/P3 future maintenance risk, not global blocker. |
| EKS subnet headroom precondition | Strong | `NODE-002` | Context-aware EKS control-plane precondition. |
| EKS VPC/security-group references | Strong | `NET-002` | Context-aware EKS control-plane precondition. |
| Workload current health | Partial | `WORKLOAD-001`, `DRAIN-005` | Detects obvious unhealthy states; not app-specific SLO validation. |
| EKS managed add-on compatibility | Strong | `ADDON-001`, `ADDON-002`, compat catalog | Context-aware operational impact metadata present. |
| Self-managed add-on compatibility | Partial | `ADDON-002`, compat catalog | Catalog based; no universal operator inventory. |
| CoreDNS Corefile checks | Partial | `COREDNS-001` | Useful static check; not full DNS runtime validation. |
| CRD served/storage/conversion risks | Strong | `CRD-001`, `CRD-002` | Good forward upgrade risk coverage. |
| APIService availability | Partial | `APISERVICE-001` | Kubernetes status based; no request-level proof. |
| EKS managed node group inventory | Strong | `EKS-NG-001` to `EKS-NG-004` | Provider-reported warnings/info. |
| EKS Upgrade Insights | Strong | `EKS-INSIGHT-001` to `003` | Provider-reported signals integrated. |
| AKS provider enrichment | Gap | provider stubs only | Recognized but not implemented. |
| GKE provider enrichment | Gap | provider stubs only | Recognized but not implemented. |
| On-prem audit-only support | Partial | cluster-only Kubernetes rules | Useful Kubernetes evidence, no infra-specific lifecycle model. |
| Context-aware upgrade gating | Strong | `internal/rules/contextual.go` | Six contexts supported through CLI flags. |
| Operator decision gate | Strong | `UpgradeGateOperatorDecision` | Rendered in terminal/Markdown/HTML/Console. |
| Priority P1-P4 | Strong | `internal/findings/priority.go` | Centralized assignment with tests. |
| Readiness score | Strong | `BuildUpgradeReadinessSummary` | Score is finding-based and coverage-qualified. |
| Evaluation coverage | Strong | `ruleExecutions`, PR #232 | Every default rule declares evidence dependencies; missing required evidence, failed rules, and mode exclusions are visible. |
| Compare resolved vs not re-evaluated | Strong | `internal/comparison`, v1.3 evidence | v1.3.0 core improvement. |
| Compare target/context mismatch warning | Strong | compare CLI sample | Gate can become neutral with insufficient evidence. |
| Report formats JSON/Markdown/HTML | Strong | `internal/report` | Output all sample generated expected files. |
| Console report import | Strong | `web/src`, web tests | 217 tests pass on rerun. |
| GitHub Action wrapper | Partial | action files/docs | Not directly executed in this audit. |
| Redaction | Strong | `internal/redact` | Designed not to alter gates, scores, fingerprints, exit codes. |
| Rollback eligibility/readiness | Partial | `internal/rollback`, `internal/rollback/eks` | Strong provenance gates; CRD/target-specific webhook directionality is target-aware; execution remains out of scope. |
| Rollback execution | Out of scope | CLI docs | Product never executes rollback. |
| Automatic remediation | Out of scope | README | Remediation text only; no product mutation. |
| Scheduled scanning/history | Gap | roadmap docs | Foundation exists through compare; no scheduler/store. |
| Ownership/acknowledgement workflow | Gap | roadmap docs | Later product capability. |
| Telemetry phone-home | Out of scope | no core telemetry found | OSS core stays local/read-only. |

## Capability By Environment

| Environment | What Works Today | Main Limits |
| --- | --- | --- |
| EKS | Full Kubernetes checks, EKS metadata, add-ons, node groups, insights, rollback readiness | Requires AWS permissions for full evidence; reduced IAM becomes incomplete/partial by design. |
| On-prem / self-managed Kubernetes | Cluster-only audit for APIs, webhooks, PDBs, drain, nodes, workloads, CoreDNS, CRDs, APIService | No provider lifecycle, no node replacement model, no cloud preconditions. |
| AKS | Basic config recognition only | No Azure collector or AKS-specific rules. |
| GKE | Basic config recognition only | No GCP collector or GKE-specific rules. |
| Manifest repository | Deprecated/removed API checks without cluster access | Not a cluster readiness assessment; 29/31 rules are not applicable. |

## Capability By Evidence Plane

| Plane | Status | Examples |
| --- | --- | --- |
| Live Kubernetes | Strong | Nodes, pods, webhooks, PDBs, CRDs, workloads, APIService, CoreDNS. |
| AWS/EKS | Strong for EKS | Cluster, add-ons, node groups, network preconditions, insights. |
| Manifests | Strong for APIs | Raw YAML and Helm-rendered deprecated API checks. |
| Catalog | Strong but scoped | Kubernetes API catalog and add-on compatibility catalog. |
| Derived analysis | Strong but explainable | Priorities, gates, scores, compare buckets, rollback recommendation. |

## Most Important Capability Boundary

KubePreflight can identify upgrade risk from observable evidence. It should not claim to prove that a future upgrade, rollback, node replacement, or workload rollout will succeed in every environment. The right product promise is "read-only, evidence-qualified upgrade readiness," not "guaranteed upgrade safety."
