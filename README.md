# KubePreflight

**Know what will break before your EKS upgrade.**

KubePreflight is **CLI-first**: the read-only CLI is the readiness engine, and
the optional local Console reads `findings.json` for demo, review, and evidence
exploration. Hosted SaaS/fleet mode remains deferred until pilot validation.

KubePreflight is a read-only CLI that correlates deprecated APIs, admission webhooks, PodDisruptionBudgets, EKS add-ons, node/kubelet skew, and AWS provider constraints into a single go/no-go readiness report — before you touch your change window.

The example below is real, captured output from `kubepreflight scan` against a local kind cluster seeded with 7 of the 10 locked-MVP failure modes (see [`demo/`](./demo)); AWS-only checks (API-002, ADDON-001, NODE-002) aren't shown here since they need a real EKS cluster. Full output for all three formats is in [`demo/sample-output/`](./demo/sample-output).

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

Next Actions (8)
  1. [Blocker] PodSecurityPolicy/demo-restricted (API-001)
  2. [Blocker] PodDisruptionBudget/demo/shared-app-pdb-b (API-001, PDB-001)
  3. [Blocker] PodDisruptionBudget/demo/shared-app-pdb-a,shared-app-pdb-b (PDB-002)
  4. [Blocker] PodDisruptionBudget/demo/singleton-app-pdb (API-001, PDB-001)
  5. [Blocker] Node/kubepreflight-demo-control-plane (NODE-001)
  6. [Blocker] PodDisruptionBudget/demo/shared-app-pdb-a (API-001)
  7. [Blocker] ValidatingWebhookConfiguration/demo-catchall-guard (WH-001, WH-002)
     ...    Also see WH-001: narrow scope + namespaceSelector, or migrate to ValidatingAdmissionPolicy
  8. [Warning] ConfigMap/kube-system/coredns (COREDNS-001)

Summary: 9 blocker(s), 2 warning(s), 0 info(s)
Reports written: findings.json · report.md · report.html
```

Note item 6: WH-001 and WH-002 fired on the *same* webhook (broad scope + a dead backend), but Next Actions merges them into one item instead of two separate, potentially contradictory instructions — the Blockers/Warnings sections above still list both separately, since that's correlation evidence worth keeping.

Full captured output: [`terminal-output.txt`](./demo/sample-output/terminal-output.txt) · [`findings.json`](./demo/sample-output/findings.json) · [`report.md`](./demo/sample-output/report.md) · [`report.html`](./demo/sample-output/report.html)

---

## Why

Kubernetes upgrades on EKS are mandatory (fixed support lifecycle), irreversible (no supported downgrade), and distributed (control plane + nodes + add-ons + webhooks + CRDs all move independently). Existing tools each cover one slice — deprecated APIs, or cluster hygiene, or native EKS insights — but nobody correlates evidence across manifests, live cluster state, AWS APIs, and telemetry into one risk graph with sequenced remediation. KubePreflight does.

## What it checks (v0.1 — the locked 10-check MVP, plus 1 added from real-world research)

| ID | Check | Data source | Severity | Confidence |
|---|---|---|---|---|
| API-001 | Deprecated/removed APIs vs target version | Live objects + raw/rendered manifests | Blocker | `STATIC_CERTAIN` |
| API-002 | EKS Upgrade Insights ingestion (30-day staleness annotated) | `eks:ListInsights`/`DescribeInsight` | Blocker/Warning | `PROVIDER_REPORTED` |
| WH-001 | Broad/catch-all fail-closed webhooks | ValidatingWebhookConfiguration | Warning | `STATIC_CERTAIN` |
| WH-002 | Fail-closed webhook, no ready endpoints | Service + EndpointSlice | Blocker | `STATIC_CERTAIN` |
| PDB-001 | `disruptionsAllowed=0` on critical path | PodDisruptionBudget status | Blocker | `STATIC_CERTAIN` |
| PDB-002 | Overlapping PDBs (incl. CoreDNS duplicate-PDB case) | PDB selectors vs live pods | Blocker | `STATIC_CERTAIN` |
| ADDON-001 | Add-on incompatible with target version | `eks:DescribeAddonVersions` | Blocker | `STATIC_CERTAIN` |
| NODE-001 | kubelet skew outside supported policy | Node status | Blocker | `STATIC_CERTAIN` |
| NODE-002 | Control-plane subnet IP headroom | `ec2:DescribeSubnets` | Blocker | `STATIC_CERTAIN` |
| NET-002 | Cluster's security group or VPC no longer exists | `ec2:DescribeSecurityGroups`/`DescribeVpcs` | Blocker | `STATIC_CERTAIN` |
| COREDNS-001 | Corefile missing `ready` plugin | ConfigMap (single allowlisted Get) | Warning | `STATIC_CERTAIN` |

`NET-002` is an 11th check, added after real-world research (AWS's own EKS upgrade troubleshooting documentation) surfaced `SecurityGroupNotFound`/`VpcIdNotFound` as common control-plane upgrade failures alongside IP exhaustion (`NODE-002`) — not part of the original locked 10-check MVP scope, but a natural sibling using the same AWS collector.

Every finding carries a confidence tier so a clean local scan is never silently contradicted by a stale EKS Insight — `API-002`'s evidence always states the 30-day audit-window staleness caveat explicitly, not as a footnote.

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
kubepreflight scan --target-version 1.34

# Specific context, all three output formats
kubepreflight scan --context prod-cluster --target-version 1.34 --output all

# Limit namespaced findings; cluster-scoped and AWS findings remain included
kubepreflight scan --target-version 1.34 --namespace-allowlist payments,platform

# Scan raw manifests alongside the required live-cluster scan
kubepreflight scan --target-version 1.34 --manifests ./k8s --output md \
  --findings-out ./out/findings.json

# With AWS/EKS enrichment (API-002, ADDON-001, NODE-002) — opt-in
kubepreflight scan --target-version 1.34 --provider eks --cluster-name my-cluster

# CI/script mode: canonical JSON, no local server, no blocking
kubepreflight scan --target-version 1.34 --output json --serve-report never
```

AWS enrichment degrades gracefully: missing credentials or IAM permissions print a one-line notice and the scan continues with cluster-only checks — it never fails the whole run. `--cluster-name` is required when `--provider=eks` is explicitly set, since that's an explicit ask that needs the info (this one *does* hard-fail, deliberately — silent skipping would contradict what you asked for).

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

## Permissions

KubePreflight is **read-only by design**. It never requests `secrets` access.

- **Kubernetes RBAC:** `get/list/watch` on nodes, pods, poddisruptionbudgets, validating/mutatingwebhookconfigurations, services, endpointslices, customresourcedefinitions, deployments, daemonsets, plus a single allowlisted `get` on the `kube-system/coredns` ConfigMap (not a blanket ConfigMap list, enforced via a separate namespace-scoped `Role` with `resourceNames`). Copy-pasteable manifest: [`deploy/clusterrole.yaml`](./deploy/clusterrole.yaml) — every rule in it is cross-checked against what the collector actually calls, verified against a real API server with `kubectl auth can-i`.
- **AWS IAM:** `eks:DescribeCluster`, `eks:ListInsights`, `eks:DescribeInsight`, `eks:ListAddons`, `eks:DescribeAddon`, `eks:DescribeAddonVersions`, `ec2:DescribeSubnets`, `ec2:DescribeSecurityGroups`, `ec2:DescribeVpcs`. All read-only. Copy-pasteable policy: [`deploy/iam-policy.json`](./deploy/iam-policy.json).

## Architecture

```
cmd/kubepreflight/          CLI entrypoint (Cobra)
internal/collectors/k8s/    Kubernetes API collector (client-go + dynamic client, read-only)
internal/collectors/aws/    EKS/EC2 collector (aws-sdk-go-v2, read-only, gracefully degrades)
internal/apicatalog/        Deprecated/removed Kubernetes API ruleset (data, not code)
internal/rules/             Rule interface, Registry, and all 11 check implementations
internal/findings/          Finding schema, confidence tiers, fingerprinting
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
| `OBSERVED` (later) | Confirmed via live probe or telemetry evidence |
| `INFERRED` (later) | Risk pattern flagged without direct proof |

## Next Actions vs. Blockers/Warnings — why findings aren't just deduped

A resource hit by multiple rules (e.g. a webhook firing both WH-001 and WH-002) still gets two separate entries in the Blockers/Warnings sections — that's correlation evidence, and collapsing it would hide *why* something is risky. The **Next Actions** section is different: it groups by resource and picks the higher-severity finding's remediation as the one instruction to follow, with any other finding's distinct guidance appended as a one-line pointer — so you get one clear step per resource, not several that might read as contradictory.

## KubePreflight Console (local viewer)

Interactive scans open a local embedded Console automatically. A React app
(`web/`) is built once at release time and embedded into the `kubepreflight`
binary via `go:embed` — **end users never install Node or run a separate
server.** `kubepreflight scan` starts a local, `127.0.0.1`-only HTTP server
(see "Usage" above) that serves `report.html`, `findings.json`, and the
Console together:

```text
Open report:
  http://127.0.0.1:<port>/report.html

Open Console:
  http://127.0.0.1:<port>/console/?findings=/findings.json#summary

Press Ctrl+C to stop serving reports.
```

The Console URL's `?findings=` query param is pre-filled with the
just-completed scan's results, so opening it loads the dashboard
immediately — no blank import screen, no manual file picker. It derives the
readiness dashboard, filters by severity/confidence/namespace/search, and
shows evidence and remediation (with a copy button) in a detail drawer per
finding. It has no backend, authentication, database, telemetry, or cluster
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

## Roadmap

- **v0.1.0** (this state) — CLI, all 10 locked-MVP checks, terminal/JSON/Markdown/HTML reports, graceful AWS degradation, kind demo walkthrough
- **v0.2.0** — SARIF, CI/GitOps integration, waivers
- **v0.3.0** — Opt-in network probes, CloudWatch telemetry, CRD conversion-webhook checks, Slack/Jira

Full technical background: [`docs/kubepreflight-deep-dive.md`](./docs/kubepreflight-deep-dive.md) (not yet added to this repo).

## Contributing

Read-only checks only. No auto-remediation, no write actions, no telemetry phone-home in the OSS core. New checks should include a fixture test (see `internal/rules/*_test.go` for the pattern: positive fixture, negative fixture, Registry wiring).

## License

Apache 2.0. See [`LICENSE`](./LICENSE).
