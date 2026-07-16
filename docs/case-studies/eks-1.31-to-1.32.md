# Real EKS case study: 1.31 → 1.32

Status: **executed** (PR3 of `v0.14.0-real-eks-case-study`, real cluster
`kubepreflight-case-study`, account `000000000000`, `us-east-1`, real
`eksctl upgrade cluster` 1.31 → 1.32, run and torn down 2026-07-16).
Sections 3–9 below are real captured results, not predictions. A
polished public write-up with screenshots is PR4's job; this section fills
in what actually happened, including three real bugs this run found and
fixed along the way.

## Purpose

Every other KubePreflight milestone this project has shipped was validated
against a real cluster at merge time, but no single artifact shows the
*whole* product story end to end: pre-upgrade readiness, an actual control-
plane upgrade, post-upgrade rollback assessment, and CI regression gating,
all on one cluster, with the evidence kept. This case study is that
artifact — and, going by every prior milestone in this project, also the
cheapest way to find the next real bug, since bugs so far have come from
actually running things, not from planning them (see
`docs/ci-integration.md`'s "Comparing two scans" section, whose two fixed
bugs were both found by running a real example, not writing one).

This is explicitly **not** a feature milestone. See "Boundary" below.

## 1. Environment

**Cluster**: EKS, managed node group, upgraded `1.31` → `1.32` during
execution (PR3). Config: [`demo/eks-case-study/cluster.yaml`](../../demo/eks-case-study/cluster.yaml)
— one `t3.small` node, no NAT gateway, no HA, tagged `AutoDelete: "true"`.
Region defaults to `us-east-1` (matching `demo/eks/eks-demo.yaml`'s existing
convention); `ap-south-1` is an acceptable substitute if quota/capacity
makes more sense there when PR3 actually runs — nothing about this case
study is region-sensitive.

Managed add-ons (CoreDNS, kube-proxy, VPC CNI) are left at whatever version
EKS provisions by default for 1.31 — deliberately not pinned to a specific
add-on version, since add-on/control-plane version skew after the upgrade
is itself one of the things `kubepreflight scan`'s add-on compatibility
checks (`ADDON-001`) are supposed to catch.

**Fixtures** — five categories, reusing proven, already-documented fixtures
wherever one already exists rather than forking them:

| Category | Source | Rule(s) | Live-applied or manifest-only? |
|---|---|---|---|
| Sample workloads | `demo/eks/manifests/pdb-lab.yaml`'s `critical-app` Deployment | — (baseline context) | Live |
| PDB risk | `demo/eks/manifests/pdb-lab.yaml` | `PDB-001`, `PDB-002` | Live |
| Unhealthy workload | `demo/eks-case-study/manifests/unhealthy-workload.yaml` (**new**) | `WORKLOAD-001` | Live |
| Deprecated API | `demo/eks/manifests/old-api.yaml` | `API-001` | **Manifest-only** |
| Admission webhook risk | `demo/eks/manifests/broken-webhook.yaml` | `WH-001`, `WH-002` | Live |

The deprecated-API fixture is manifest-only for a real technical reason,
not just caution: `policy/v1beta1 PodDisruptionBudget`/`PodSecurityPolicy`
stopped being *served* at all starting in Kubernetes 1.25 — a live 1.31
control plane rejects creating one outright. `demo/eks/manifests/old-api.yaml`
already documents this (`# SCAN-ONLY — do not apply this manifest to any
cluster`); `API-001` fires from `kubepreflight scan --manifests
demo/eks/manifests` regardless of whether the object ever touches the
cluster, exactly like it would for a stale manifest sitting in a real repo,
chart, or GitOps source.

`unhealthy-workload.yaml` is the one genuinely new fixture this case study
needed — nothing in `demo/` or `demo/eks/` already exercises `WORKLOAD-001`
(a live-cluster check for pods that were already broken, e.g.
`ImagePullBackOff`/`CrashLoopBackOff`, before any upgrade happens at all —
see `internal/rules/workload001.go`). It's a single-replica Deployment with
a nonexistent image tag: immediate, reliable, and its only footprint is one
pod stuck in `ErrImagePull` — no capacity consumed, no restart storm, fully
reversible by deleting the namespace.

Cluster is never deliberately broken at the infrastructure level (no
starved nodes, no severed networking, no IAM sabotage) — every risk is a
controlled, namespaced Kubernetes object that cleanup can remove
independently of cluster teardown.

## 2. Initial risks

Going in, the five fixtures above are expected to produce, on the very
first scan after they're applied (before any upgrade or remediation):

- `BLOCKED` overall verdict
- `API-001` (Blocker) — from the manifest-only scan-only file
- `PDB-001`, `PDB-002` (Blocker) — from `pdb-lab.yaml`
- `WH-001`, `WH-002` (Blocker) — from `broken-webhook.yaml`
- `WORKLOAD-001` (Warning or Blocker, per that rule's own severity — pre-
  existing breakage, not an upgrade-caused regression) — from
  `unhealthy-workload.yaml`

This list is a prediction, not a captured result — PR3 replaces it with
the real `kubepreflight scan` output, and if reality disagrees with this
table, that disagreement is itself a finding worth writing up (see
"Boundary" below on what counts as an in-scope fix).

## 3. KubePreflight findings

**Clean baseline** (before any fixture, `kubepreflight scan --provider eks
--target-version 1.32`, no `--manifests`): the design doc's own prediction
in section 2 (`CLEAN`) was wrong — reality is `PASSED_WITH_WARNINGS`,
score 78/100. A genuinely minimal single-node cluster has real, legitimate
warnings a multi-node cluster wouldn't: `DRAIN-003` (CoreDNS's
nodeAffinity is satisfiable by exactly the one node that exists),
`ADDON-002` ×3 (the specific EKS-shipped add-on versions for `coredns`/
`kube-proxy`/`vpc-cni` have no compatibility-catalog entry for target
1.32 yet), and one Admission Webhooks warning from EKS's own
`vpc-resource-validating-webhook`. None of these are seeded fixtures —
they're what a real, minimal EKS cluster actually looks like. Corrected
here rather than left as a wrong prediction.

**Before** (after seeding, `--manifests` pointed at `old-api.yaml` only —
see the tooling-bug note in section 8): `BLOCKED`, score 19/100, 7
Blockers — `WH-005` ×2 (the catch-all webhook can intercept writes to
admission webhook configs and to node objects), `API-001` (the scan-only
`old-pdb-api` manifest), `PDB-001` ×2, `PDB-002`, `WH-002`. Plus Warnings
for `WORKLOAD-001` (the pre-existing broken pod), `DRAIN-001` ×2,
`DRAIN-003`, `ADDON-002` ×3, `EKS-NG-002`, `WH-001`, `WH-004`, and a P4
`WH-005` on EKS's own `vpc-resource-validating-webhook` (a real system
webhook, not a fixture — informational-priority, correctly not a
Blocker). The very first run of this scan also surfaced a real product
bug — see section 8 — before it was fixed and re-run.

## 4. Remediation decisions

Removed the fail-closed webhook and its backend Service (fixes `WH-001`/
`WH-002`/`WH-004`/`WH-005`), scaled `critical-app` to 2 replicas (fixes
`PDB-001`, since a 1-replica Deployment behind a `minAvailable: 1` PDB has
zero disruption headroom by construction), and deleted the overlapping
PDB (fixes `PDB-002`). Left `WORKLOAD-001` (the pre-existing broken pod)
and the manifest-only `API-001` finding untouched, exactly as planned —
neither blocks an EKS control-plane upgrade by itself, and both are
useful negative-control evidence that the scan (and later, the compare
gate) correctly leaves alone what it wasn't asked to fix.

**After remediation**: `BLOCKED`, score 57/100, exactly 1 Blocker left —
the deliberately-untouched `old-pdb-api` manifest finding. Comparing
before → after-remediation: **9 resolved (6 Blockers), 0 new, gate
decision `pass`**, score 19 → 57. The remediation script's own step order
had a real bug on the first attempt — see section 8.

## 5. Upgrade execution

`eksctl upgrade cluster --name kubepreflight-case-study --region us-east-1
--approve`: started 22:33:59 UTC, control plane reported upgraded to
`1.32` at 22:42:32 UTC — about 8.5 minutes. `aws eks describe-cluster`
confirmed `status: ACTIVE, version: "1.32"` immediately after. As
designed (see `04-upgrade.sh`'s own comment), this upgrades the control
plane only — the managed node group's kubelet stayed at `1.31.14`
afterward, a real, supported 1-minor skew, not a bug. Cluster creation
(not the upgrade itself) hit an unrelated eksctl client-side timeout —
see section 8.

## 6. Rollback readiness result

`kubepreflight rollback assess` against the post-upgrade cluster:
**eligible**, readiness **blocked** (1 blocker, 4 warnings, 1 unknown),
recommendation **do_not_proceed** at high confidence, rollback window "at
least 167h 48m remaining." The one hard-fail check was API/CRD/webhook
compatibility — correctly citing the same still-outstanding manifest-only
`API-001` finding from section 4 (deliberately never remediated) as the
reason. Every other check that could resolve at all resolved (`ACTIVE`
status, exact N-1 rollback target, extended-support policy, node group
compatibility); the `_UNVERIFIED`/`_UNKNOWN` reason codes on EKS-API-only
signals (end-of-extended-support origin, backward-incompatible feature
list) are honest "can't confirm from available evidence" rather than a
guessed pass — matches the tool's own stated "don't guess" design.

## 7. CI comparison result

Two comparisons captured. **before → after-remediation** (section 4):
9 resolved, 0 new, gate `pass`. **after-remediation → after-upgrade**:
gate `pass`, 0 new, but **26 resolved (0 Blockers)** — all 26 turned out
to be the `Info`-severity flowcontrol bootstrap findings (`FlowSchema`/
`PriorityLevelConfiguration`), not real fixes. Once `--target-version`
equals the cluster's actual post-upgrade version, the CLI correctly
switches into a "no version upgrade required" mode and skips
upgrade-transition-only live-cluster checks (there's no forward-looking
transition left to check) while still fully evaluating manifest-safety
and current-state findings — confirmed by checking `resolved[]` in
`comparison.json` directly: every single one was an `Info` flowcontrol
finding, nothing Warning/Blocker silently vanished. The gate's `pass`
decision was still the right answer (nothing got worse), but a reader
skimming "26 resolved" without reading the detail could mistake mode-
driven noise for real fixes — see section 8's documentation note on this.

The **hosted** `compare` composite Action run against these real
findings (not just this local CLI invocation) needs `gh` auth this
sandbox doesn't have — carried forward to PR4/Final as a handoff item,
not silently dropped.

## 8. Bugs discovered

1. **Real product bug, fixed** — EKS-injected `flowcontrol.apiserver.k8s.io`
   defaults (`FlowSchema` `eks-exempt`/`eks-leader-election`/
   `eks-monitoring`/`eks-workload-high`, `PriorityLevelConfiguration`
   `eks-monitoring`) were misclassified as Blocker instead of Info.
   `internal/collectors/k8s.IsAutoManagedObject` only checked the
   `apf.kubernetes.io/autoupdate-spec: "true"` annotation, but EKS's own
   control plane sets these to `"false"` or omits the annotation
   entirely, reconciling them through a separate mechanism instead —
   confirmed via `managedFields`, where every one of the 5 objects
   carries field manager `eks-internal`, absent on every vanilla
   kube-apiserver default. Fixed by also recognizing that field manager.
   This was 6 spurious Blockers (out of 25 total on a first, unfixed
   scan of an otherwise completely vanilla cluster) — a real
   false-positive that would mislead any SRE running KubePreflight
   against EKS into thinking they own a migration task for objects they
   have no ability to edit or delete. See `internal/collectors/k8s/collector.go`
   and `internal/rules/api001.go`.
2. **Case-study tooling bug, fixed (not a product bug)** —
   `scripts/case-study/02-scan.sh` pointed `--manifests` at the whole
   `demo/eks/manifests/` directory instead of just `old-api.yaml`,
   double-counting `pdb-lab.yaml`/`broken-webhook.yaml` as both live
   cluster objects and static manifests for the same real objects,
   producing redundant manifest-plane findings. Fixed by pointing
   `--manifests` directly at `old-api.yaml` (the manifest collector
   explicitly supports a single-file path, confirmed by reading
   `internal/collectors/manifest/collector.go`'s own
   `relativeSourcePath` comment before relying on it).
3. **Case-study tooling bug, fixed (not a product bug)** —
   `scripts/case-study/03-remediate.sh` tried to `kubectl scale` (an
   UPDATE) in the `preflight-lab` namespace before removing the
   still-active fail-closed webhook, which correctly rejected the write
   ("no endpoints available for service dead-webhook"). Fixed by
   reordering: webhook removal (a DELETE, unaffected by the webhook's
   own CREATE/UPDATE-only rule) first.
4. **Documentation correction** — the design doc's section 2 predicted
   the clean-baseline scan would come back `CLEAN`; a real single-node
   cluster legitimately comes back `PASSED_WITH_WARNINGS` (see section
   3). Corrected here and in `demo/eks-case-study/README.md`'s expected-
   results table rather than left wrong.
5. **Documented nuance, not a bug** — comparing a mid-transition scan
   against an already-at-target scan produces a large "resolved" count
   that's mostly upgrade-transition checks becoming moot, not real
   fixes (see section 7). Worth a callout in `docs/ci-integration.md`'s
   compare section for real users chaining pre/post-upgrade scans, so a
   large resolved-count doesn't get over-read — tracked as a documentation
   follow-up, not bundled into this already-large PR.
6. **Infrastructure quirk, informational only** — `eksctl create cluster`
   reported `exceeded max wait time for StackCreateComplete waiter` and
   a nonzero exit code, but `aws cloudformation describe-stacks` showed
   both stacks at `CREATE_COMPLETE` and the cluster/node group fully
   `ACTIVE` moments later — a client-side wait-timeout in this specific
   account/environment, not a real failure. Not a kubepreflight or
   eksctl config bug; noted here so a future run of this case study
   knows to verify via `aws eks describe-cluster`/`describe-nodegroup`
   rather than trusting `eksctl`'s own exit code alone (the same
   file-existence-over-exit-code principle the GitHub Action entrypoints
   already apply, just relearned here against a different tool).

## 9. Cleanup

`demo/eks-case-study/cleanup.sh` run after all evidence was captured:
webhook, both seeded namespaces, and the cluster itself
(`eksctl delete cluster --wait`) all deleted; `aws eks list-clusters
--region us-east-1` confirmed empty immediately after. See this PR's
commit for the exact log.

## 10. Final outcome

PR3 is complete: a real EKS 1.31 → 1.32 upgrade exercised the full
product story — pre-upgrade readiness, manual remediation, the real
control-plane upgrade, post-upgrade rollback assessment, and the CI
comparison gate — end to end, with one real product bug found and fixed
(section 8, item 1) and every temporary AWS resource confirmed removed.
Remaining before this milestone is release-locked: the hosted `compare`
Action run against these real findings (needs `gh` auth), and PR4's
polished public write-up with real screenshots.

## Boundary

This case study is validation, not a feature vehicle. In scope: real bug
fixes, misleading-output corrections, missing validation, docs/behavior
mismatches uncovered by running the real thing, unsafe failure semantics,
test gaps. Out of scope, regardless of how tempting mid-execution: new
rules unrelated to what this case study actually exercises, AKS/GKE
support, any SaaS/history work, broad UI redesign, automatic remediation.
A discovered idea that doesn't fit this case study's own findings gets
written down for later, not built here.

## PR split

1. **Case-study environment and fixture design** (this document) —
   `docs/case-studies/eks-1.31-to-1.32.md`, `demo/eks-case-study/cluster.yaml`,
   `demo/eks-case-study/manifests/unhealthy-workload.yaml`.
2. **Reproducible scripts and evidence capture** — `scripts/case-study/`
   (apply fixtures, run scans, capture JSON/Markdown/HTML/Console
   screenshots, run the comparison gate) plus `demo/eks-case-study/cleanup.sh`.
3. **Real EKS execution and discovered fixes** — actually run PR2's
   scripts against a real cluster; sections 3–9 above filled in with real
   captured evidence; any real bugs found get fixed in this PR.
4. **Case-study documentation and screenshots** — this document's section
   10 filled in, plus a polished narrative write-up suitable for public
   reading (the case study's ultimate deliverable).
5. **Final** — audit, cleanup verification (no lingering AWS resources),
   release.

## Success criteria

Not "release-locked" until:

- a real EKS cluster was used (not `kind`, not mocked)
- before/after evidence is captured, not synthetic
- the rollback path was actually exercised (`rollback assess` against a
  real post-upgrade cluster)
- the hosted `compare` composite Action ran on a real GitHub Actions
  runner, not just locally
- every screenshot in the final write-up is a real capture
- all temporary AWS resources are confirmed removed
- every real product bug this case study found is either fixed or
  explicitly tracked, not silently dropped
- the final case-study document reads as public-quality documentation
