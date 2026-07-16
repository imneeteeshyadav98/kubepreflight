# Real EKS case study: 1.31 → 1.32

Status: **plan** (PR1 of `v0.14.0-real-eks-case-study`). Sections 1–2 below
are the committed methodology; sections 3–10 are filled in as later PRs in
this milestone actually execute against a real cluster — nothing in this
document is a fabricated or synthetic result.

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

_Pending — captured in PR3 (real EKS execution): the actual pre-upgrade
`kubepreflight scan --provider eks --target-version 1.32 --manifests
demo/eks/manifests --output all` run against the seeded cluster, with
`findings.json`/`report.md`/`report.html` committed as evidence and linked
here._

## 4. Remediation decisions

_Pending — captured in PR3: which of the seeded findings get remediated
before the upgrade proceeds (the PDB overlap and the webhook are the two
genuinely upgrade-blocking ones; the manifest-only API-001 and the
pre-existing `WORKLOAD-001` pod are deliberately left as-is, since neither
blocks an EKS control-plane upgrade by itself) and why, with the resulting
"after remediation" scan captured per the Evidence checklist below._

## 5. Upgrade execution

_Pending — captured in PR3: the real `eksctl upgrade cluster --approve`
run (1.31 → 1.32), timing, and anything that happened during it worth
recording._

## 6. Rollback readiness result

_Pending — captured in PR3: `kubepreflight rollback assess` against the
post-upgrade cluster — rollback window, EKS Upgrade Insights, node/add-on/
workload evidence, and the resulting rollback-vs-fix-forward
recommendation._

## 7. CI comparison result

_Pending — captured in PR3/PR4: baseline (pre-upgrade) vs current
(post-upgrade) `findings.json` run through `kubepreflight compare`, and —
per the milestone's success criteria — through the **hosted** `compare`
composite Action on a real GitHub Actions runner, not just locally._

## 8. Bugs discovered

_Pending — running total of every real product bug this case study
surfaces, each either fixed in the PR that found it or explicitly tracked
if genuinely out of scope. Two are already on the board from the
`v0.13.0-github-action-comparison` milestone's own real-example testing
(absolute-path `findings-file` chaining, `null`-vs-`[]` JSON serialization)
— not this case study's find, but the same category of bug this exercise
exists to keep finding._

## 9. Cleanup

_Pending — captured in PR3/Final: confirmation that the seeded namespaces,
webhook, and the EKS cluster itself are fully removed, following
`demo/eks/cleanup.sh`'s existing pattern (delete webhook + namespace, then
`eksctl delete cluster --wait`, then verify `aws eks list-clusters` returns
empty)._

## 10. Final outcome

_Pending — Final PR: overall summary once every section above is filled in
for real._

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
