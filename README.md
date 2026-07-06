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
- Multi-hop upgrade planner (`kubepreflight plan`) with plan-aware HTML and
  an interactive Console planner — see [Multi-hop upgrade planner](#multi-hop-upgrade-planner)

The example below is a captured baseline scan against a local kind cluster seeded with the original MVP failure modes (see [`demo/`](./demo)). Newer coverage/CRD/APIService fields are exercised by automated fixtures; refresh captured live-demo artifacts after any real-cluster demo run.

```text
KubePreflight scan — cluster: kind-kubepreflight-demo  target: 1.34  provider: cluster-only
Result: BLOCKED

Blockers (9)
  [API-001] PodDisruptionBudget "demo/shared-app-pdb-a" (apiVersion policy/v1beta1) still exists
  at a version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API...
  (also fires for shared-app-pdb-b and singleton-app-pdb — policy/v1beta1 PodDisruptionBudget is
  its own removed API, distinct from the PodSecurityPolicy case below)

  [API-001] PodSecurityPolicy "demo-restricted" (apiVersion policy/v1beta1) still exists at a
  version removed in Kubernetes 1.25 — target version 1.34 will no longer serve this API...

  [NODE-001] Node "kubepreflight-demo-control-plane": kubelet version v1.24.15 is outside the
  supported skew window for target version 1.34 — 10 minor versions behind, exceeds n-3 policy

  [PDB-001] PodDisruptionBudget demo/singleton-app-pdb: disruptionsAllowed=0 (minAvailable: 1,
  currentHealthy: 1) — matching pods cannot be voluntarily evicted...
  (also fires for shared-app-pdb-b)

  [PDB-002] PodDisruptionBudgets demo/shared-app-pdb-a and demo/shared-app-pdb-b select an
  overlapping set of pods — the Eviction API rejects eviction when multiple PDBs match...

  [WH-002] ValidatingWebhookConfiguration "demo-catchall-guard": fail-closed, zero ready
  endpoints — matching API writes will be rejected

Warnings (2)
  [COREDNS-001] CoreDNS Corefile is missing the `ready` plugin...
  [WH-001] ValidatingWebhookConfiguration "demo-catchall-guard": catch-all scope...

Next Actions (6)
  1. [Blocker] PodSecurityPolicy/demo-restricted (API-001)
  2. [Blocker] PodDisruptionBudget/demo/shared-app-pdb-a (API-001, PDB-001, PDB-002)
  3. [Blocker] PodDisruptionBudget/demo/singleton-app-pdb (API-001, PDB-001)
  4. [Blocker] Node/kubepreflight-demo-control-plane (NODE-001)
  5. [Blocker] ValidatingWebhookConfiguration/demo-catchall-guard (WH-001, WH-002)
     ...    Also see WH-001: narrow scope + namespaceSelector, or migrate to ValidatingAdmissionPolicy
  6. [Warning] ConfigMap/kube-system/coredns (COREDNS-001)

Summary: 9 blocker(s), 2 warning(s), 0 info(s)
Reports written: findings.json · report.md · report.html
```

Note item 5: WH-001 and WH-002 fired on the *same* webhook (broad scope + a dead backend), but Next Actions merges them into one item instead of two separate, potentially contradictory instructions — the Blockers/Warnings sections above still list both separately, since that's correlation evidence worth keeping.

Full captured output: [`terminal-output.txt`](./demo/sample-output/terminal-output.txt) · [`findings.json`](./demo/sample-output/findings.json) · [`report.md`](./demo/sample-output/report.md) · [`report.html`](./demo/sample-output/report.html)

---

## What it checks

14 checks today:

| ID | Check | Data source | Severity | Confidence |
|---|---|---|---|---|
| API-001 | Deprecated/removed APIs vs target version | Live objects + raw/rendered manifests | Blocker | `STATIC_CERTAIN` |
| API-002 | EKS Upgrade Insights ingestion (30-day staleness annotated) | `eks:ListInsights`/`DescribeInsight` | Blocker/Warning | `PROVIDER_REPORTED` |
| WH-001 | Broad/catch-all fail-closed webhooks | ValidatingWebhookConfiguration | Warning | `STATIC_CERTAIN` |
| WH-002 | Fail-closed webhook, no ready endpoints | Service + EndpointSlice | Blocker | `OBSERVED` |
| PDB-001 | Fresh `disruptionsAllowed=0` with selected pods | PodDisruptionBudget status | Blocker | `OBSERVED` |
| PDB-002 | Overlapping PDBs (incl. CoreDNS duplicate-PDB case) | PDB selectors vs live pods | Blocker | `OBSERVED` |
| ADDON-001 | Add-on incompatible with target version | `eks:DescribeAddonVersions` | Blocker | `PROVIDER_REPORTED` |
| NODE-001 | kubelet skew outside supported policy | Node status | Blocker | `STATIC_CERTAIN` |
| NODE-002 | Control-plane subnet IP headroom | `ec2:DescribeSubnets` | Blocker | `STATIC_CERTAIN` |
| NET-002 | Cluster's security group or VPC no longer exists | `ec2:DescribeSecurityGroups`/`DescribeVpcs` | Blocker | `STATIC_CERTAIN` |
| COREDNS-001 | Corefile missing `ready` plugin | ConfigMap (single allowlisted Get) | Warning | `STATIC_CERTAIN` |
| CRD-001 | Legacy CRD stored versions need migration | CustomResourceDefinition status | Warning | `STATIC_CERTAIN` |
| CRD-002 | CRD conversion webhook has no ready endpoints | CRD + EndpointSlice | Blocker | `OBSERVED` |
| APISERVICE-001 | Aggregated APIService is unavailable | APIService status | Blocker | `OBSERVED` |

`NET-002` was added after AWS upgrade troubleshooting guidance surfaced `SecurityGroupNotFound`/`VpcIdNotFound` as common hard failures alongside IP exhaustion. CRD storage/conversion and aggregated APIService availability extend that same principle to Kubernetes extension APIs.

Every finding carries a confidence tier so a clean local scan is never silently contradicted by a stale EKS Insight — `API-002`'s evidence always states the 30-day audit-window staleness caveat explicitly, not as a footnote.

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

# With AWS/EKS enrichment (API-002, ADDON-001, NODE-002) — opt-in
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

# CI/script mode: canonical JSON, no local server, no blocking
kubepreflight scan --target-version 1.36 --output json --serve-report never

# Keep a run's artifacts together
kubepreflight scan --target-version 1.36 --output all --output-dir ./preflight-output
```

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
- **AWS IAM:** `eks:DescribeCluster`, `eks:ListInsights`, `eks:DescribeInsight`, `eks:ListAddons`, `eks:DescribeAddon`, `eks:DescribeAddonVersions`, `ec2:DescribeSubnets`, `ec2:DescribeSecurityGroups`, `ec2:DescribeVpcs`. All read-only. Copy-pasteable policy: [`deploy/iam-policy.json`](./deploy/iam-policy.json).

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

## Contributing

Read-only checks only. No auto-remediation, no write actions, no telemetry phone-home in the OSS core. New checks should include a fixture test (see `internal/rules/*_test.go` for the pattern: positive fixture, negative fixture, Registry wiring).

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
