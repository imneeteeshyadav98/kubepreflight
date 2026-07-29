# KubePreflight Rule Inventory

Date: 2026-07-29

Source of truth: `internal/rules/defaults.go`, `internal/findings/priority.go`, `internal/findings/report.go`, and individual rule files under `internal/rules`.

## Registry Summary

- Default registry: 31 rules.
- Manifests-only registry: `API-001`, `API-002`.
- Manifest-only execution record behavior: 2 `applicable`/`evaluated`, 29 `not_applicable`/`not_evaluated`.
- Evidence dependency mapping today: every default-registry rule declares explicit required evidence. `WH-002`, `CRD-002`, and `DRAIN-001` use conditional dependencies so empty inventories stay `evaluated` while failed collection of actually-required follow-on evidence becomes `insufficient_evidence`. `ADDON-002` continues to emit its own warning finding for add-on verification uncertainty where that is the rule's intended product signal.

## Rule Table

| Rule | Category | What It Detects | Evidence | Default Priority | Context/Gate Notes |
| --- | --- | --- | --- | --- | --- |
| `API-001` | API Compatibility | Resources using APIs removed by target Kubernetes version | Live deprecated API resources, manifests, API catalog | P2 | Blocker when removed API reaches target; can also emit same-version info. |
| `API-002` | API Compatibility | Resources using APIs deprecated before target but not yet removed | Live resources, manifests, API catalog | P4 | Warning/allow. |
| `WH-001` | Admission Webhooks | Webhooks with broad/admission-impacting scope and risky failure behavior | Webhook configurations | P4 | Can become P1 through `GlobalBlocker` override if rule marks global write blocker. |
| `WH-002` | Admission Webhooks | Webhook backend missing, invalid, or zero ready endpoints | Webhook config, Service, EndpointSlice | P4 | FailurePolicy Fail can block API writes; endpoint collector failure maps to insufficient evidence. |
| `WH-004` | Admission Webhooks | Webhook side effects/dry-run compatibility risk | Webhook configurations | P4 | API write/future maintenance impact. |
| `WH-005` | Admission Webhooks | Excessive timeout, wildcard operations, self-interception, high-risk scopes | Webhook configurations | P4 | Mostly warning/operator-decision; global/critical flags affect priority/scope. |
| `DRAIN-001` | Drain Readiness | Pods not safely managed by drain-aware controllers | Pods/workload ownership | P3 | Warning; worker rollout, node drain, workload restart impacts. |
| `DRAIN-002` | Drain Readiness | Local PV / singleton data durability risk during drain | Workloads, Pods, PV/PVC | P3 | Context-aware: blocks worker rollout/full platform when severe; audit/control-plane allow. |
| `DRAIN-003` | Drain Readiness | Pods with risky termination/grace/lifecycle behavior | Pods/workloads | P3 | Warning; drain/workload-restart impact. |
| `DRAIN-004` | Drain Readiness | Node pressure/taints/readiness risks | Nodes | P3 | Warning; node drain/current health. |
| `DRAIN-005` | Drain Readiness | Zero-ready StatefulSet/DaemonSet or critical workload readiness risk | Workloads | P3 | Context-aware current-health gate; routed to rollback workload health, not disruption. |
| `PDB-001` | Disruption Safety | PDB with zero allowed disruptions | PDBs | P3 | Blocks worker rollout/full platform; allow for audit/control-plane; unspecified operator decision. |
| `PDB-002` | Disruption Safety | Overlapping PDB selectors that can make disruption math unsafe | PDBs, Pods | P3 | Same drain-dependent gate as `PDB-001`. |
| `NODE-001` | Node Readiness | Kubelet version skew unsupported for target | Nodes | P3 | Universal blocker when skew violates supported path. |
| `NODE-002` | Node Readiness | EKS subnet IP headroom below control-plane upgrade expectations | AWS subnet/network preflight | P2 | Blocks control-plane-only/full-platform; audit allow; other contexts operator decision. |
| `NODE-003` | Node Readiness | Workload depends on deprecated `node-role.kubernetes.io/master` label | Workload scheduling constraints | P3 | Warning/allow, future scheduling maintenance risk. |
| `NET-002` | Node Readiness | Missing/unavailable EKS VPC or security group references | AWS network preflight | P2 | Same EKS control-plane precondition gate as `NODE-002`. |
| `WORKLOAD-001` | Workload Health | Unhealthy workload status before change | Workloads | P4 | Warning/operator decision depending context and criticality. |
| `ADDON-001` | Add-ons | EKS-managed or cataloged add-on incompatible with target | AWS add-on APIs, compatibility catalog, workload inventory | P2 | Context-aware by operational impact metadata. |
| `ADDON-002` | Add-ons | Add-on compatibility unknown or upgrade recommended | AWS add-on verification, catalog/self-managed inventory | P3/P4 | Warning; emits explicit uncertainty finding instead of insufficient-evidence execution state. |
| `EKS-NG-001` | Node Readiness | Managed node group health issues | AWS DescribeNodegroup | P4 | Provider-reported warning. |
| `EKS-NG-002` | Node Readiness | Managed node group has limited rolling update headroom | AWS node group scaling/update config | P3 | Warning. |
| `EKS-NG-003` | Node Readiness | Managed node group uses launch template/custom AMI needing manual validation | AWS node group metadata | P4 | Info. |
| `EKS-NG-004` | Node Readiness | Managed node group Kubernetes version differs from target | AWS node group metadata | P4 | Info; kubelet skew handled by `NODE-001`. |
| `EKS-INSIGHT-001` | EKS Upgrade Insights | AWS EKS Upgrade Insight blocker status | AWS EKS Insights | P2 | Provider-reported blocker. |
| `EKS-INSIGHT-002` | EKS Upgrade Insights | AWS EKS Upgrade Insight warning status | AWS EKS Insights | P4 | Provider-reported warning. |
| `EKS-INSIGHT-003` | EKS Upgrade Insights | AWS EKS Upgrade Insight unknown/error/stale status | AWS EKS Insights | P4 | Provider-reported warning/info depending rule logic. |
| `COREDNS-001` | CoreDNS | CoreDNS Corefile plugin/configuration risk | `kube-system/coredns` ConfigMap | P4 | Warning; static config check only. |
| `CRD-001` | Extension APIs | CRD served/storage version removed or incompatible by target | CRDs, API catalog | P2 | Blocker/warning based on target API compatibility. |
| `CRD-002` | Extension APIs | CRD conversion webhook has no ready backend or unsafe conversion path | CRDs, Services, EndpointSlices | P2 | Blocker for confirmed conversion risk; insufficient evidence mapped for CRD/EndpointSlice failure. |
| `APISERVICE-001` | Extension APIs | Aggregated APIService not Available | APIService status | P2 | Warning/operator decision; not proof upgrade itself fails. |

## Rule Execution States

| State | Meaning | Operator Reading |
| --- | --- | --- |
| `evaluated` | Rule was applicable and completed from available evidence | Absence of finding is meaningful for that rule, subject to plane coverage. |
| `not_evaluated` + `not_applicable` | Rule is outside scan mode, for example manifest-only excludes live cluster rules | Do not treat absent findings as fixed or safe for that domain. |
| `insufficient_evidence` | Rule needs data that collector could not provide | Rerun with permissions/connectivity or review manually. |
| `failed` | Rule returned an error | User-visible in generated reports. The scan/plan result becomes `INCOMPLETE`; a successfully written incomplete report exits `3`. |

## Context Helpers

| Helper | Used By | Behavior |
| --- | --- | --- |
| `drainDependentGate` | `PDB-001`, `PDB-002`, parts of `DRAIN-002` | Block for worker rollout/full platform; allow audit/control-plane; operator decision otherwise. |
| `currentHealthGate` | `WORKLOAD-001`, `DRAIN-005` style current health | Blocks only critical zero-ready workloads in worker/full-platform contexts; otherwise warning/operator decision or allow. |
| `eksControlPlanePreconditionGate` | `NODE-002`, `NET-002` | Block for control-plane/full-platform; allow audit-only; operator decision for worker/workload/unspecified. |
| `addonCompatibilityGate` | `ADDON-001` | Uses compatibility catalog operational impact metadata by selected context. |

## Inventory Conclusions

The 31-rule registry is coherent and materially useful. After PR #232, the main evidence-integrity gap is closed: every default rule has an explicit dependency contract and registry tests lock duplicate/missing rule coverage. Future rule work should preserve this contract and avoid adding a rule without dependency metadata and renderer/Console coverage.
