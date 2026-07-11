<p align="center">
  <img src="docs/assets/kubepreflight-readme-banner.png" alt="KubePreflight logo" width="760" />
</p>

# KubePreflight

Read-only EKS upgrade readiness CLI with evidence-backed reports and a local Console.

## Problem

EKS upgrades fail or get delayed because teams discover deprecated APIs, PDB
drain blockers, fail-closed webhooks, add-on incompatibilities, or node
kubelet skew too late — usually mid-upgrade, in the middle of a change
window. Existing tools each cover one slice of this (deprecated APIs, or
cluster hygiene, or native EKS insights), but nobody correlates evidence
across manifests, live cluster state, and AWS APIs into one risk graph with
sequenced remediation.

## What it does

KubePreflight scans before the upgrade and answers: **"Will this upgrade
break production?"** It correlates deprecated APIs, admission webhooks,
PodDisruptionBudgets, EKS add-ons, node/kubelet skew, and AWS provider
constraints into a single go/no-go readiness report — before you touch your
change window.

KubePreflight is **CLI-first**: the read-only CLI is the readiness engine, and
the optional local Console reads `findings.json` for demo, review, and evidence
exploration. Hosted SaaS/fleet mode remains deferred until pilot validation.

## Current capabilities

- Read-only scan against a live cluster (cluster-only or `--provider=eks`)
- JSON, Markdown, and HTML reports, plus a compact terminal summary
- Embedded local React Console (no Node/browser account required) with a
  split-pane findings workspace, filters, and evidence/remediation detail
- Local-only report server (`127.0.0.1`) serving `report.html`,
  `findings.json`, and the Console together
- Every finding is evidence-backed (raw values, not just a rule name) with a
  confidence tier and a specific remediation
- AWS/EKS enrichment (EKS Upgrade Insights, add-on compatibility, subnet/VPC
  checks) when `--provider=eks` is used — degrades gracefully without it
- **Validated against a real EKS cluster**, both clean and seeded
  worst-case (see [Validated on real EKS](#validated-on-real-eks) below)
- Every finding is also assigned an upgrade **Priority (P1–P4)** — what to
  fix first, separate from Severity (go/no-go) and Confidence (how certain
  the evidence is) — see [Priority (P1–P4)](#priority-p1p4)
- Multi-hop upgrade planner (`kubepreflight plan`) with plan-aware HTML,
  an interactive Console planner, and an optional generated action-plan
  checklist — see [Multi-hop upgrade planner](#multi-hop-upgrade-planner)

The example below is from a real scan against a local kind cluster seeded with the original MVP failure modes (see [`demo/`](./demo)) — run it yourself and you'll get this exact shape of output; nothing here is a committed, aging capture (see [Demo output isn't committed](#demo-output-isnt-committed) below for why).

```text
KubePreflight scan — cluster: kind-kubepreflight-demo  target: 1.34  provider: cluster-only
Result: BLOCKED

Blockers (13)
  [P2/API-001] PodDisruptionBudget "demo/shared-app-pdb-a" (apiVersion policy/v1beta1) still exists
  at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API...
    Priority P2 (do not attempt other remediation until this is fixed): Resource or behavior may
    fail after the target Kubernetes upgrade.
  (also fires for shared-app-pdb-b and singleton-app-pdb — policy/v1beta1 PodDisruptionBudget is
  its own removed API, distinct from the PodSecurityPolicy case below)

  [P2/API-001] PodSecurityPolicy "demo-restricted" (apiVersion policy/v1beta1) still exists at a
  version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API...

  [P2/API-001] EndpointSlice "default/kubernetes" (apiVersion discovery.k8s.io/v1beta1) still
  exists at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this
  API... (also fires for 2 more EndpointSlices — controller-managed, not user-authored; see
  [Known limitations](#known-limitations))

  [P3/NODE-001] Node "kubepreflight-demo-control-plane": kubelet version v1.24.15 is outside the
  supported skew window for target version 1.34 — 10 minor versions behind, exceeds n-3 policy

  [P3/PDB-001] PodDisruptionBudget demo/singleton-app-pdb: disruptionsAllowed=0 (minAvailable: 1,
  currentHealthy=1, desiredHealthy=1, expectedPods=1) — healthy matching pods cannot currently be
  voluntarily evicted, so a node drain or node upgrade can stall or fail
  (also fires for shared-app-pdb-b)

  [P3/PDB-002] PodDisruptionBudgets demo/shared-app-pdb-a and demo/shared-app-pdb-b select an
  overlapping set of pods (2 overlapping pods) — the Eviction API rejects eviction when multiple
  PDBs match the same pod...

  [P4/WH-002] ValidatingWebhookConfiguration "demo-catchall-guard": fail-closed, zero ready
  endpoints — matching API writes will be rejected

Warnings (3)
  [P4/COREDNS-001] CoreDNS Corefile is missing the `ready` plugin...
  [P4/WH-001] ValidatingWebhookConfiguration "demo-catchall-guard": catch-all scope...

Info (21)
  21 FlowSchema/PriorityLevelConfiguration objects kube-apiserver itself owns and recreates — see
  [Known limitations](#known-limitations) for why these are Info, not Blocker.

Next Actions (11)
  1. [P2/Blocker] PodSecurityPolicy/demo-restricted (API-001)
  ...
  9. [P4/Blocker] ValidatingWebhookConfiguration/demo-catchall-guard (WH-001, WH-002)
     ...    Also see WH-001: Inspect the webhook's current rules and selectors: ...

Summary: 13 blocker(s), 3 warning(s), 21 info(s)
Reports written: findings.json · report.md · report.html
```

WH-001 and WH-002 fired on the *same* webhook (broad scope + a dead backend), but Next Actions merges them into one item instead of two separate, potentially contradictory instructions — the Blockers section above still lists both separately, since that's correlation evidence worth keeping.

### Demo output isn't committed

`demo/sample-output/` used to be a captured, committed example — it went stale repeatedly as the product evolved, and a full scan's realistic finding count (dominated by cluster-plumbing objects like EndpointSlices, not just the demo's own seeded failures) made a frozen snapshot actively misleading rather than illustrative. Run the block above yourself (see [`demo/README.md`](./demo/README.md)) for real, current output. Nothing in this repo's own tests depends on a committed capture staying accurate either — `web/tests/browser_smoke.py` drives the Console against a small fixture generated fresh from `internal/report` on every run (see [CI / dev verification](#ci--dev-verification)), not demo output.

---

## What it checks

22 checks today:

| ID | Check | Data source | Severity | Confidence |
|---|---|---|---|---|
| API-001 | Deprecated/removed APIs vs target version | Live objects + raw/rendered manifests | Blocker | `STATIC_CERTAIN` |
| WH-001 | Broad/catch-all fail-closed webhooks | ValidatingWebhookConfiguration | Warning | `STATIC_CERTAIN` |
| WH-002 | Fail-closed webhook, no ready endpoints | Service + EndpointSlice | Blocker | `OBSERVED` |
| PDB-001 | Fresh `disruptionsAllowed=0` with selected pods | PodDisruptionBudget status | Blocker | `OBSERVED` |
| PDB-002 | Overlapping PDBs (incl. CoreDNS duplicate-PDB case) | PDB selectors vs live pods | Blocker | `OBSERVED` |
| ADDON-001 | Add-on incompatible with target version | `eks:DescribeAddonVersions` | Blocker | `PROVIDER_REPORTED` |
| EKS-NG-001 | EKS managed node group health issues | `eks:ListNodegroups`/`DescribeNodegroup` | Warning | `PROVIDER_REPORTED` |
| EKS-NG-002 | EKS managed node group limited update headroom | `eks:ListNodegroups`/`DescribeNodegroup` | Warning | `PROVIDER_REPORTED` |
| EKS-NG-003 | EKS managed node group launch template/custom AMI review | `eks:ListNodegroups`/`DescribeNodegroup` | Info | `PROVIDER_REPORTED` |
| EKS-NG-004 | EKS managed node group version context | `eks:ListNodegroups`/`DescribeNodegroup` | Info | `PROVIDER_REPORTED` |
| EKS-INSIGHT-001 | EKS Upgrade Insight reports ERROR | `eks:ListInsights`/`DescribeInsight` | Warning | `PROVIDER_REPORTED` |
| EKS-INSIGHT-002 | EKS Upgrade Insight reports WARNING | `eks:ListInsights`/`DescribeInsight` | Warning | `PROVIDER_REPORTED` |
| EKS-INSIGHT-003 | EKS Upgrade Insight status UNKNOWN | `eks:ListInsights`/`DescribeInsight` | Info | `PROVIDER_REPORTED` |
| NODE-001 | kubelet skew outside supported policy | Node status | Blocker | `STATIC_CERTAIN` |
| NODE-002 | Control-plane subnet IP headroom | `ec2:DescribeSubnets` | Blocker | `STATIC_CERTAIN` |
| NODE-003 | Deprecated `node-role.kubernetes.io/master` scheduling label | Live and manifest workload pod templates | Warning; Blocker for critical infrastructure | `STATIC_CERTAIN` |
| NET-002 | Cluster's security group or VPC no longer exists | `ec2:DescribeSecurityGroups`/`DescribeVpcs` | Blocker | `STATIC_CERTAIN` |
| COREDNS-001 | Corefile missing `ready` plugin | ConfigMap (single allowlisted Get) | Warning | `STATIC_CERTAIN` |
| CRD-001 | Legacy CRD stored versions need migration | CustomResourceDefinition status | Warning | `STATIC_CERTAIN` |
| CRD-002 | CRD conversion webhook has no ready endpoints | CRD + EndpointSlice | Blocker | `OBSERVED` |
| APISERVICE-001 | Aggregated APIService is unavailable | APIService status | Blocker | `OBSERVED` |
| WORKLOAD-001 | Pod already unhealthy before the upgrade (ImagePullBackOff, CrashLoopBackOff, etc.) | Live pod status | Warning | `OBSERVED` |

`NET-002` was added after AWS upgrade troubleshooting guidance surfaced `SecurityGroupNotFound`/`VpcIdNotFound` as common hard failures alongside IP exhaustion. EKS-NG checks cover EKS managed node groups returned by the EKS `ListNodegroups` API only; self-managed node groups are not listed by that AWS API. EKS Upgrade Insights are AWS-native signals surfaced as warning/info findings plus inventory; `ERROR` is not treated as a blocker yet. CRD storage/conversion and aggregated APIService availability extend that same principle to Kubernetes extension APIs.

Every finding carries a confidence tier so a clean local scan is never silently contradicted by a stale provider signal. EKS Upgrade Insight evidence is marked `PROVIDER_REPORTED` and includes freshness/staleness context rather than replacing KubePreflight's local checks.

## Provider support

| Provider | Status |
|---|---|
| cluster-only (no `--provider`) | Current — Kubernetes-plane checks run against any cluster |
| `eks` | Current — validated against a real EKS cluster (see [Validated on real EKS](#validated-on-real-eks)) |
| `aks` | Planned — CLI flags recognized and validated today; enrichment checks not implemented yet |
| `gke` | Planned — CLI flags recognized and validated today; enrichment checks not implemented yet |

See [`docs/provider-roadmap.md`](./docs/provider-roadmap.md) for what each
provider's enrichment checks do (or will do), and which checks are already
portable across all of them.

## Install

```bash
# Build from source (only supported path today; binary releases land later)
git clone <this-repo>
cd kubepreflight && go build -o kubepreflight ./cmd/kubepreflight

# Or via Docker
docker build -t kubepreflight:local .
docker compose up   # mounts ~/.kube read-only, writes findings.json to ./out
```

The current distroless Docker image does not include the `helm` binary, so use
`--manifests` with raw/rendered YAML in the container or run KubePreflight on the
host for Helm-chart scanning. A CI-friendly Helm strategy is tracked for the
CI/GitOps integration milestone.

`docker-compose.yml` uses `network_mode: host` — required on Linux because kind
(and most local clusters) bind their API server to `127.0.0.1` on the host, which
is unreachable from inside a container without host networking. **This is
Linux-only**: Docker Desktop on macOS/Windows runs containers inside a VM, where
host networking doesn't provide the same access to a locally-running kind
cluster. On those platforms, run KubePreflight natively (`go run`/built binary)
against a local kind cluster rather than via `docker compose` — this hasn't been
verified against Docker Desktop, so treat it as a known gap, not a working path.

## Usage

```bash
# Cluster-only scan (no AWS setup required)
./kubepreflight scan --target-version 1.36 --serve-report always

# With AWS/EKS enrichment (EKS-INSIGHT, ADDON, EKS-NG, NODE/NET) — opt-in
AWS_PROFILE=<profile> AWS_REGION=<region> ./kubepreflight scan \
  --provider eks \
  --cluster-name <cluster-name> \
  --target-version 1.36 \
  --serve-report always

# Scan raw manifests alongside the required live-cluster scan
./kubepreflight scan \
  --provider eks \
  --cluster-name <cluster-name> \
  --target-version 1.36 \
  --manifests ./k8s \
  --serve-report always

# Limit namespaced findings; cluster-scoped and AWS findings remain included
kubepreflight scan --target-version 1.36 --namespace-allowlist payments,platform

# CI/script mode: canonical JSON file, no local server, no blocking,
# and no human-readable text on stdout — see the note below.
kubepreflight scan \
  --target-version 1.36 \
  --output json \
  --terminal-output silent \
  --findings-out findings.json \
  --serve-report never

jq '.summary' findings.json

# Keep a run's artifacts together
kubepreflight scan --target-version 1.36 --output all --output-dir ./preflight-output
```

`--output json` controls which **report file** gets written (`findings.json`, always written regardless of `--output`) — it does not by itself change what prints to stdout. Stdout is controlled separately by `--terminal-output` (`full` by default): unless you also pass `--terminal-output silent`, stdout still gets the full human-readable report even in JSON mode, which will break a naive `kubepreflight scan --output json | jq .` pipeline. Add `--terminal-output silent` in CI if stdout must stay machine-safe, and read the JSON from the file it's written to.

AWS enrichment degrades gracefully: missing credentials or IAM permissions do not discard Kubernetes findings, but the report is marked `INCOMPLETE` and exits 3 so CI cannot mistake missing provider evidence for readiness. `--cluster-name` is required when `--provider=eks` is explicitly set.

`--findings-out` always writes the canonical JSON report, including when
`--output=md` or `--output=html`; `--output` selects additional human-readable
artifacts. Manifest scanning is currently additive and still requires a live
cluster connection. A standalone no-cluster CI mode is deliberately deferred
because every live rule needs an explicit nil-safety audit before that contract
is safe.

By default, a scan attached to an interactive terminal writes
`findings.json`, `report.md`, and `report.html`, then serves the report and
local Console on a random `127.0.0.1` port until you press Ctrl+C. Redirected
stdout, `CI` environments, and explicit `--output=json` runs do not start or
wait on the server. Use `--serve-report=never` for scripts,
`--serve-report=always` to override non-interactive detection, `--listen` to
choose the local address, and `--open-report` to ask the OS to open the report
URL. Browser-open failure never invalidates the scan.

Once the local server is starting, stdout switches to a compact summary
(cluster/target/provider/result/counts + the report and Console URLs)
instead of the full per-finding listing — report.html and the Console
already show every finding's evidence and remediation, so repeating it on
stdout is redundant. Use `--terminal-output full` to keep the old detailed
output even while serving, or `--terminal-output silent` to print nothing
but the URLs (and fatal errors). Runs that aren't serving a local report
keep today's full terminal output by default, so scripts and CI output are
unaffected unless you pass `--terminal-output` explicitly.

When `--namespace-allowlist` is set, findings with known namespaced resources
are included only when every namespaced reference belongs to the allowlist.
Cluster-scoped Kubernetes and AWS findings remain visible. Namespace-less
namespaced manifests are excluded because their apply-time namespace cannot be
inferred safely; the active allowlist is recorded in every report format.

### Exit codes (for CI)

| Code | Meaning |
|---|---|
| `0` | Clean — no blockers or warnings |
| `1` | Warnings only |
| `2` | Blockers found |
| `3` | Assessment incomplete because requested evidence could not be collected |
| `4` | Scan infrastructure failure — no trustworthy report was produced at all (bad kubeconfig, cannot build a Kubernetes client, or the collector failed outright). Distinct from `3`: `3` means a report exists but some evidence is missing; `4` means no report was written. A CI gate checking `exit code <= 1` for "safe to proceed" must not treat `4` as safe. |

## Output

A `scan` run writes to the current directory by default. Use `--output-dir`
to place all generated artifacts together; `--findings-out` can override the
canonical JSON filename/path.

- `findings.json` — canonical machine-readable report, always written
- `report.md` — Markdown, written with `--output=md` or `all`
- `report.html` — single-file, self-contained HTML report, written with
  `--output=html`/`all` or whenever a local server starts

When a local report server starts (see above), it prints:

```text
Open report:
  http://127.0.0.1:<port>/report.html

Open Console:
  http://127.0.0.1:<port>/console/?findings=/findings.json#summary
```

The Console URL's `?findings=` query param is pre-filled with the
just-completed scan's results, so opening it loads the dashboard
immediately.

## Permissions

KubePreflight is **read-only by design**. It never requests `secrets` access.

- **Kubernetes RBAC:** `get/list/watch` on nodes, pods, poddisruptionbudgets, validating/mutatingwebhookconfigurations, services, endpointslices, customresourcedefinitions, deployments, daemonsets, plus a single allowlisted `get` on the `kube-system/coredns` ConfigMap (not a blanket ConfigMap list, enforced via a separate namespace-scoped `Role` with `resourceNames`). Copy-pasteable manifest: [`deploy/clusterrole.yaml`](./deploy/clusterrole.yaml) — every rule in it is cross-checked against what the collector actually calls, verified against a real API server with `kubectl auth can-i`.
- **AWS IAM:** `eks:DescribeCluster`, `eks:ListInsights`, `eks:DescribeInsight`, `eks:ListAddons`, `eks:DescribeAddon`, `eks:DescribeAddonVersions`, `eks:ListNodegroups`, `eks:DescribeNodegroup`, `ec2:DescribeSubnets`, `ec2:DescribeSecurityGroups`, `ec2:DescribeVpcs`. All read-only; KubePreflight does not call `eks:StartInsightsRefresh`. Copy-pasteable policy: [`deploy/iam-policy.json`](./deploy/iam-policy.json).

## Safety

- **No cluster writes.** Every Kubernetes and AWS call KubePreflight makes is
  a read (`get`/`list`/`watch`/`Describe*`/`List*`). It never creates,
  patches, or deletes anything in your cluster or AWS account.
- **No auto-remediation.** Findings include remediation guidance and
  copy-pasteable commands; nothing is ever applied automatically.
- **No SaaS upload required.** `findings.json`/`report.html`/the Console are
  local files served from `127.0.0.1` by default. Binding a non-loopback
  address requires the explicit `--allow-remote-report` acknowledgement
  because the local server has no authentication.
- **AWS credentials stay local.** KubePreflight uses whatever the AWS SDK's
  standard credential chain resolves (env vars, shared config/credentials
  file, IAM role) — it never reads, logs, or transmits credentials
  elsewhere.
- **Seeded demo manifests are for test/demo clusters only.** `pdb-lab.yaml`
  and `broken-webhook.yaml` deliberately create a fail-closed webhook and
  disruption-blocking PDBs to exercise the worst-case checks — never apply
  them to a production cluster.

## Architecture

```
cmd/kubepreflight/          CLI entrypoint (Cobra)
internal/collectors/k8s/    Kubernetes API collector (client-go + dynamic client, read-only)
internal/collectors/aws/    EKS/EC2 collector (aws-sdk-go-v2, read-only, gracefully degrades)
internal/apicatalog/        Deprecated/removed Kubernetes API ruleset (data, not code)
internal/rules/             Rule interface, Registry, and all 11 check implementations
internal/findings/          Finding schema, confidence tiers, fingerprinting
internal/plan/              Multi-hop upgrade planner: version discovery, hop generation, per-rule projection policy
internal/report/            Terminal / JSON / Markdown / HTML renderers (shared dedup logic)
internal/reportserver/      Local-only post-scan HTTP report serving (report.html, findings.json, embedded Console)
web/                        React Console (Vite + TypeScript), built once and embedded into the Go binary via go:embed
testdata/                   Fixture clusters for deterministic rule testing
demo/                       kind demo cluster manifests + captured sample output
deploy/                     ClusterRole, IAM policy (Terraform module planned, not shipped)
```

## Confidence tiers

| Tier | Meaning |
|---|---|
| `STATIC_CERTAIN` | Provable directly from live objects or provider data treated as ground truth |
| `PROVIDER_REPORTED` | Relayed from AWS's own judgment (e.g. EKS Insights), with caveats |
| `OBSERVED` | Confirmed from time-sensitive live state such as endpoint/PDB/APIService status |
| `INFERRED` | Risk pattern flagged without direct proof |

## Priority (P1–P4)

Every finding carries three independent axes, and it's easy to conflate
them:

- **Severity** — Blocker/Warning/Info — drives the go/no-go `Result` and
  exit code.
- **Confidence** — the tier above — how certain the evidence is.
- **Priority** — P1 through P4 — what to fix **first**, and whether that
  fix has to happen before you touch anything else.

Two Blocker-severity findings can carry very different priority: a global
admission-webhook outage needs attention right now (P1), while an
incompatible EKS add-on (P4) needs fixing before the upgrade starts but
isn't actively breaking anything yet.

| Priority | Meaning |
|---|---|
| **P1** | Global API write blocker — may block `kubectl apply`/`patch`/`scale`, Helm upgrades, or controller reconciliation cluster-wide, including the commands needed to fix every other finding. Always wins regardless of which rule caught it. |
| **P2** | Removed API, or a cluster-critical infrastructure component (CNI, DNS, kube-system) affected by an otherwise-lower-priority condition — a resource or behavior that will fail once the target upgrade actually happens. |
| **P3** | Node-drain risk — something that can cause a node drain to stall or fail during maintenance or a managed node group upgrade. |
| **P4** | Unhealthy workload, add-on, or node-readiness risk — the upgrade shouldn't begin while these are unhealthy, but they aren't an active blocker on their own. |

Every finding also carries three fields that explain the priority, not
just state it (real fields, from a live scan's `findings.json`):

```json
{
  "ruleId": "PDB-001",
  "severity": "Blocker",
  "priority": "P3",
  "priorityReason": "Node drain may fail during maintenance or a managed node group upgrade.",
  "affectedScope": "workload",
  "canUpgradeContinue": false
}
```

- **`priorityReason`** — why this priority, in one sentence.
- **`affectedScope`** — `global`, `cluster`, `node`, `workload`, or
  `addon`: what class of resource the fix actually touches.
- **`canUpgradeContinue`** — `false` for every Blocker-severity finding
  and for anything at P1; `true` otherwise. The terminal output spells
  this out too: `"do not attempt other remediation until this is fixed"`
  vs. `"can continue upgrade planning"`.

Two conditions escalate past a rule's normal priority, regardless of
which rule caught them:

- **Global blocker** — a fail-closed admission webhook with catch-all
  scope and no healthy backend escalates straight to P1, because its
  outage can break the `kubectl`/Helm commands needed to fix everything
  else.
- **Critical infrastructure** — the same condition on an ordinary
  application workload vs. a `kube-system` or CNI/DNS/kube-proxy/
  autoscaler component escalates to at least P2, since the blast radius
  is the whole cluster rather than one workload (this is what makes
  `NODE-003`'s deprecated-master-label check a Warning on a normal app
  but a Blocker on `calico-node`).

**Findings are sorted Priority-first everywhere a human reads them** —
terminal, Markdown, HTML, the Console's Findings and Evidence tabs, and
Next Actions grouping — so the first thing listed is always the most
urgent thing to fix, not just the first thing the scanner happened to
find. `report.html` and the Console both show a one-line P1–P4 legend
near the first finding. `findings.json`'s `findings` array itself is
**not** re-sorted — it stays in rule-registration order as the canonical
machine-readable record; a `jq` pipeline that cares about priority order
should sort on the `priority` field itself (`P1` < `P2` < `P3` < `P4`
lexically, so a plain string sort already works).

## Next Actions vs. Blockers/Warnings — why findings aren't just deduped

A resource hit by multiple rules (e.g. a webhook firing both WH-001 and WH-002) still gets two separate entries in the Blockers/Warnings sections — that's correlation evidence, and collapsing it would hide *why* something is risky. The **Next Actions** section is different: it groups by resource and picks the higher-severity finding's remediation as the one instruction to follow, with any other finding's distinct guidance appended as a one-line pointer — so you get one clear step per resource, not several that might read as contradictory.

## KubePreflight Console (local viewer)

Interactive scans open a local embedded Console automatically. A React app
(`web/`) is built once at release time and embedded into the `kubepreflight`
binary via `go:embed` — **end users never install Node or run a separate
server.** `kubepreflight scan` starts a local, `127.0.0.1`-only HTTP server
(see [Output](#output) above) that serves `report.html`, `findings.json`,
and the Console together.

The Console URL's `?findings=` query param is pre-filled with the
just-completed scan's results, so opening it loads the dashboard
immediately — no blank import screen, no manual file picker. It derives the
readiness dashboard, filters by severity/confidence/namespace/search, and
shows evidence plus structured safe/emergency/break-glass remediation and
verification commands in a detail drawer per finding. It has no backend,
authentication, database, telemetry, or cluster
connector; imported files stay in the browser. `report.html` remains the
static, shareable CAB/export artifact — the Console is for interactive
investigation.

Use `--serve-report never` for CI, scripts, or any run where nothing should
block on a local server (this is also the default when stdout isn't a
terminal or `CI` is set — see "Usage" above).

See [`web/README.md`](./web/README.md) for how the Console is built and
tested. This is intentionally not a multi-tenant SaaS surface; hosted fleet
features remain gated on discovery and pilot signal. The staged product
boundary is documented in [`docs/product-shape.md`](./docs/product-shape.md).

## Multi-hop upgrade planner

Real EKS upgrades from an old cluster (e.g. 1.29) to a much newer target
(e.g. 1.36) happen one minor version at a time — a single scan against the
final target is misleading, since live-cluster-state findings (node/kubelet
skew, PDB overlap, webhook health) won't necessarily still be true several
hops from now. `kubepreflight plan` provides a sequenced, hop-by-hop readiness view:

```bash
./kubepreflight plan \
  --from-version 1.29 \
  --to-version 1.36 \
  --manifests ./k8s
```

Honestly, not optimistically:

- The **immediate next hop** is a real, exact scan — identical to what
  `scan` would produce for that target version.
- **Further hops** only project what's honestly predictable (a
  manifest's deprecated API doesn't change hop to hop; AWS's own EKS
  Upgrade Insights/add-on compatibility API is authoritative for whatever
  version it's asked about) and label everything else — node skew, PDB
  overlap, webhook health — as **checks requiring a rescan**
  once that hop is actually reached, with the exact `scan` command to run
  at that point.
- Writes `upgrade-plan.json` alongside the immediate hop's normal
  `findings.json`/`report.html`.
- Plan-generated `report.html` includes the upgrade path and readiness
  verdict. The Console automatically loads `upgrade-plan.json`, adds an
  Upgrade Planner tab, distinguishes current-live from projected findings,
  and shows exact rescan commands for future-hop coverage requirements.

### Upgrade action plan (change-ticket checklist)

`plan` can also emit a phased, operator-facing checklist derived from the
immediate hop's findings — built for pasting into a change ticket, not
for re-parsing programmatically:

```bash
./kubepreflight plan \
  --from-version 1.32 --to-version 1.36 \
  --action-plan-out action-plan.json \
  --action-plan-md action-plan.md
```

`action-plan.md` groups actions into four phases — Critical Blockers,
Upgrade Preparation, Upgrade, and Validation — with Phase 3 gated on
every required Phase 1 action being resolved. Each action lists its
source rule IDs, success criteria, and copy-pasteable inspect commands —
for example, a real run against a cluster with a stuck PDB and a
deprecated node-role label produces:

```markdown
### Phase 1 - Critical Blockers

Fix findings that can prevent a safe upgrade or make upgrade remediation fail.

**Gate:** Phase 3 is blocked until every required action in this phase is resolved.

- [ ] **Resolve disruption budget and unhealthy workload risks** (`required`, required)
  - Why: Required because matching findings were detected in the current assessment.
  - Source rules: `PDB-001`
  - Success: Workloads protected by PodDisruptionBudgets can tolerate at least one voluntary disruption.
  - Command: `kubectl get pdb --all-namespaces`
- [ ] **Replace deprecated master node label selectors** (`recommended`, optional)
  - Source rules: `NODE-003`
  - Command: `kubectl get nodes --show-labels | grep -E 'node-role.kubernetes.io/(master|control-plane)'`
```

Most actions are always `required` once their source rule fires at all.
A couple — currently the WORKLOAD-001 and NODE-003 checklist items — are
`recommended` instead when only a Warning-severity finding matches and no
Blocker does, since those two checks are explicitly designed to not be
hard upgrade blockers on their own (see
[Priority (P1–P4)](#priority-p1p4)'s `canUpgradeContinue`).
`action-plan.json` carries the same structure machine-readably
(`schemaVersion: kubepreflight.io/upgrade-action-plan/v1`) if you want to
drive a ticketing integration instead of pasting Markdown by hand.

## Validated on real EKS

This isn't just tested against fixtures — it's been run against a real,
throwaway EKS cluster (EKS 1.35, `us-east-1`) end to end:

- A clean cluster scan (`--provider eks`) returned `Result: CLEAN` with
  `AWS enrichment: true`.
- The same cluster, seeded with worst-case resources (overlapping
  zero-disruption PDBs, a fail-closed catch-all webhook, a manifest with a
  removed API), returned `Result: BLOCKED` and correctly fired
  `API-001`, `PDB-001`, `PDB-002`, `WH-001`, and `WH-002`.
- Finding counts matched exactly across `findings.json`, `report.html`, and
  the Console in both runs.
- The test cluster and all seeded resources were deleted afterward, with
  `aws eks list-clusters` confirming no orphaned resources remained.

To validate against a throwaway real EKS cluster yourself, see
[`demo/eks/README.md`](./demo/eks/README.md). **This creates billable AWS
resources** — use a sandbox account and delete the cluster immediately
after testing.

## Not included yet

- SaaS/hosted backend
- SARIF output
- Auto-remediation (and never planned as a default — see [Safety](#safety))

### Known limitations

- **`API-001` still reports live `EndpointSlice` objects as normal Blockers.**
  `discovery.k8s.io/v1beta1` EndpointSlices are controller-managed, not
  hand-authored — closer in spirit to the `Event`/`FlowSchema` cases below
  than to a PDB or PSP a person actually owns — but unlike those two,
  there's no confirmed reliable signal (an annotation, a naming
  convention) to distinguish "this will just regenerate at the new API
  version" from a genuine migration task. Left as a real Blocker rather
  than guessed at; tracked as an open follow-up.
- **`API-001` excludes live `Event` objects entirely** (not even as Info) —
  they're emitted by whatever client-go version the calling controller
  links, self-expire in about an hour, and there's no single object to
  fix. **`FlowSchema`/`PriorityLevelConfiguration` objects kube-apiserver
  itself owns** (marked with its own `apf.kubernetes.io/autoupdate-spec`
  annotation, confirmed against a live cluster) report as **Info**, not
  Blocker, with remediation text that says there's usually nothing to do
  — a user-created FlowSchema/PriorityLevelConfiguration without that
  annotation is unaffected and still reports as a normal Blocker.

## Roadmap

- **v0.1.0** — CLI, all 10 locked-MVP checks, terminal/JSON/Markdown/HTML reports, graceful AWS degradation, kind demo walkthrough
- **v0.2.0-alpha** (this state) — full-width Console/report, multi-hop planner, explicit scan coverage, CRD/APIService checks, validated against a real EKS cluster
- **v0.2.0** — SARIF, waivers, release packaging, and expanded provider checks
- **v0.3.0** — Opt-in network probes, CloudWatch telemetry, Slack/Jira

Full technical background: [`docs/kubepreflight-deep-dive.md`](./docs/kubepreflight-deep-dive.md) (not yet added to this repo).

## CI / dev verification

```bash
go test ./...
go vet ./...
npm --prefix web test
npm --prefix web run build
scripts/check-console-dist.sh
docker build -t kubepreflight:local .
```

CI runs this verification matrix on pushes and pull requests.
`scripts/check-console-dist.sh` rebuilds the Console and diffs it against
the committed `web/dist` — it fails if a `web/src` change was committed
without also committing the rebuilt, embedded Console assets.

### Manually generating a report against a real cluster

`go test`/`npm test` don't catch real layout or click/scroll behavior —
Vitest's jsdom environment can't compute CSS grid/box layout, and Go's HTML
tests only check for output substrings. To visually verify a
`report.html`/Console change, build the binary and run a real scan against
a connected cluster:

```bash
cd ~/kubepreflight
rm -rf bin && mkdir -p bin
go build -o bin/kubepreflight ./cmd/kubepreflight
./bin/kubepreflight --help

# Confirm the target cluster is reachable
kubectl config current-context
kubectl get nodes

# Or, for a local kind cluster:
kind get kubeconfig --name kp-smoke > /tmp/kp-smoke.kubeconfig
kubectl --kubeconfig /tmp/kp-smoke.kubeconfig get nodes

# Generate the report into its own directory
mkdir -p /tmp/kp-report && cd /tmp/kp-report
~/kubepreflight/bin/kubepreflight scan \
  --kubeconfig /tmp/kp-smoke.kubeconfig \
  --target-version 1.36 \
  --findings-out findings.json \
  --output all \
  --serve-report never || true

ls -lah   # findings.json, report.md, report.html

python3 -m http.server 8080
# then open http://127.0.0.1:8080/report.html
```

`|| true` matters here: a scan that finds blockers exits `2` (see
[Exit codes](#exit-codes-for-ci)) — that's the scan working correctly, not
a tooling failure, and the report is written either way.
`--output all --serve-report never` writes `findings.json`/`report.md`/
`report.html` to disk without blocking on the CLI's own server, which is
what lets `python3 -m http.server` serve those same files afterward. If
you'd rather use the CLI's built-in server instead (auto-opens the report,
prints the report/Console URLs, no separate `python3` step needed), drop
`--output all --serve-report never` — see [Usage](#usage) above.

## Contributing

Read-only checks only. No auto-remediation, no write actions, no telemetry phone-home in the OSS core. New checks should include a fixture test (see `internal/rules/*_test.go` for the pattern: positive fixture, negative fixture, Registry wiring).

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
