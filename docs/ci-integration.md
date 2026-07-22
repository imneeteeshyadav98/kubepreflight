# KubePreflight in GitHub Actions

`imneeteeshyadav98/kubepreflight` is a composite GitHub Action that runs the
same read-only `kubepreflight scan` you'd run locally, inside your CI, and
turns the result into a job pass/fail decision, a Step Summary scorecard,
and downloadable `findings.json`/`report.html` artifacts.

It runs the exact released Docker image (`ghcr.io/imneeteeshyadav98/kubepreflight`)
via `docker run` rather than rebuilding from source, so it stays fast and
always matches whatever tag you pin `uses:` to — there is no separate build
step, and it works on any `ubuntu-latest` runner with Docker available.

## Quick start: manifest-only scan (no cluster credentials)

```yaml
- uses: imneeteeshyadav98/kubepreflight@v0.4.1-ci-integration
  with:
    target-version: '1.36'
    manifests: './deploy'
    manifests-only: 'true'
```

This flags removed/deprecated APIs in your raw YAML — nothing else needs to
be true about the runner. Good for gating every PR that touches manifests,
with no AWS or cluster access to provision in CI at all.

`manifests-only: 'true'` matters: without it, `kubepreflight scan` still
tries to load a kubeconfig and reach a live cluster even when `manifests`
is set — that's a real, currently-existing limitation of the underlying
CLI (`--manifests` is additive to a live scan, not a replacement for one),
not something this action papers over. Setting it also narrows which
checks run to just the two that actually read manifest data (`API-001`,
`API-002`) — every other rule needs live cluster/AWS state and is skipped
entirely, so **most Upgrade Readiness categories will read "Passed" in a
manifests-only scan without ever having been checked** (Extension APIs,
Admission Webhooks, Disruption Safety, Node Readiness, Add-ons, CoreDNS,
Workload Health, EKS Upgrade Insights). Only trust the "API Compatibility"
row and the overall verdict from a manifests-only run; run the live-cluster
scan too before treating the others as clean.

## Live-cluster scan

```yaml
- uses: imneeteeshyadav98/kubepreflight@v0.4.1-ci-integration
  with:
    target-version: '1.36'
    provider: 'eks'
    cluster-name: 'production'
    region: 'ap-south-1'
    kubeconfig: './kubeconfig'
```

Additionally correlates live cluster state — PDB drain blockers, fail-closed
webhooks, node/kubelet skew, CoreDNS health — and, with `provider: eks`,
AWS-native EKS Upgrade Insights and add-on compatibility.

### Live-cluster scans on EKS: the kubeconfig has to be pre-resolved

The action's container is built from `gcr.io/distroless/static-debian12` —
no shell, no `aws` CLI, nothing but the `kubepreflight` binary itself. A
kubeconfig straight out of `aws eks update-kubeconfig` uses an **exec
plugin** (`command: aws, args: [eks, get-token, ...]`) that Kubernetes
client libraries invoke at connection time — which fails inside this
container, since there's no `aws` binary to exec.

The fix is two extra lines in the step that resolves the kubeconfig,
*before* calling this action — replace the exec-plugin user entry with a
static bearer token fetched once, on the runner, where `aws` actually
exists:

```yaml
- uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: ${{ secrets.KUBEPREFLIGHT_AWS_ROLE_ARN }}
    aws-region: ap-south-1

- name: Resolve a static-token kubeconfig for the EKS cluster
  run: |
    aws eks update-kubeconfig --name production --region ap-south-1 --kubeconfig ./kubeconfig
    export TOKEN=$(aws eks get-token --cluster-name production --region ap-south-1 --output json | jq -r '.status.token')
    yq -i '.users[0].user = {"token": strenv(TOKEN)}' ./kubeconfig

- uses: imneeteeshyadav98/kubepreflight@v0.4.1-ci-integration
  with:
    target-version: '1.36'
    provider: 'eks'
    cluster-name: 'production'
    region: 'ap-south-1'
    kubeconfig: './kubeconfig'
```

A kubeconfig using a static client certificate or bearer token (common for
self-managed/kind/non-EKS clusters) needs no such rewrite — mount it as-is
via `kubeconfig:`.

`region` is passed through as `AWS_REGION` to the AWS SDK's own credential
chain — it is not a `kubepreflight scan` CLI flag. AWS credentials
(`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN`/`AWS_PROFILE`/`AWS_DEFAULT_REGION`)
already present in the job environment — e.g. exported by
`aws-actions/configure-aws-credentials` — are forwarded into the container
automatically when set.

## Inputs

| Input | Required | Default | Meaning |
|---|---|---|---|
| `target-version` | yes | — | Target Kubernetes version, e.g. `1.36` |
| `upgrade-context` | no | `unspecified` | Planned operation: `unspecified`, `audit-only`, `control-plane-only`, `worker-rollout`, `full-platform-upgrade`, or `workload-restart` |
| `manifests` | no | `''` | Directory of raw YAML to scan for deprecated APIs |
| `manifests-only` | no | `'false'` | If `'true'`, skip kubeconfig/cluster/AWS collection entirely — requires `manifests` to be set |
| `provider` | no | `''` | `eks` for AWS enrichment (`aks`/`gke` not implemented yet) |
| `cluster-name` | no | `''` | EKS cluster name (required when `provider: eks`) |
| `region` | no | `''` | AWS region, forwarded as `AWS_REGION` |
| `kubeconfig` | no | `''` | Path on the runner to an already-resolved kubeconfig |
| `fail-on-warning` | no | `'false'` | Fail the job on Warning findings or operator decisions too, not just blockers |
| `findings-out` | no | `findings.json` | Path, relative to the workspace, for the JSON report |
| `report-out` | no | `.` | Directory, relative to the workspace, for `report.md`/`report.html` |

## Outputs

| Output | Meaning |
|---|---|
| `verdict` | `CLEAN`, `PASSED_WITH_WARNINGS`, `BLOCKED`, `INCOMPLETE`, or `INFRA_FAILURE` |
| `blockers` | Number of effective upgrade blockers (empty on `INFRA_FAILURE`) |
| `warnings` | Number of Warning-severity findings (empty on `INFRA_FAILURE`) |
| `operator-decisions` | Number of findings requiring an operator decision (empty on `INFRA_FAILURE`) |
| `readiness-score` | Upgrade Readiness score, 0–100 (empty on `INFRA_FAILURE`) |
| `can-upgrade-continue` | `"true"`/`"false"` (empty on `INFRA_FAILURE`) |
| `findings-file` | Path to the written `findings.json` on the runner |
| `report-file` | Path to the written `report.html` on the runner, if one was produced |

**Compatibility note:** if you do not set `upgrade-context`, contextual drain,
PDB, webhook-scope, aggregated-API, and EKS control-plane provider precondition
risks are evaluated under the default `unspecified` context. Some findings that
previously failed with blocker behavior may now require operator review instead.
For rollout gates, set the operation explicitly:

```yaml
with:
  upgrade-context: worker-rollout
```

Downstream steps can read these directly:

```yaml
- uses: imneeteeshyadav98/kubepreflight@v0.4.1-ci-integration
  id: kubepreflight
  with:
    target-version: '1.36'
    manifests: './deploy'

- run: echo "score was ${{ steps.kubepreflight.outputs.readiness-score }}"
```

## Exit policy

| Verdict | Job outcome |
|---|---|
| `CLEAN` | Pass |
| `PASSED_WITH_WARNINGS` | Pass, unless `fail-on-warning: 'true'` |
| `BLOCKED` | Always fail |
| `INCOMPLETE` | Always fail — partial evidence is never treated as a pass |
| `INFRA_FAILURE` | Always fail — no report was produced at all (bad kubeconfig, unreachable cluster, invalid inputs); distinct from `INCOMPLETE`, which does produce a report but with partial coverage |

`INFRA_FAILURE` is detected by checking that `findings.json` actually exists
on disk, not by trusting the container's numeric exit code alone —
`kubepreflight`'s own documented exit-code contract uses `1` for both
"warnings only" and ordinary pre-report usage errors, which this action
disambiguates by reading `findings.json`'s own `upgradeReadiness.verdict`
field (the same string that drives the CLI's own exit code) whenever a
report actually exists.

## What you get in the GitHub UI

- **Step Summary**: an Upgrade Readiness scorecard (verdict, score, blocker/warning counts, and a per-category Passed/Warning/Failed table) rendered directly on the job's summary page — no need to open the log.
- **Workflow artifacts**: `findings.json` and `report.html` uploaded as a `kubepreflight-report` artifact on every run, including failed ones (`if: always()`), so a `BLOCKED` PR still has a downloadable report to attach to a change ticket.
- **Job status**: reflects the exit policy above.

## Requirements on the runner

- Docker (present by default on GitHub-hosted `ubuntu-latest` runners)
- `jq` (also present by default on GitHub-hosted `ubuntu-latest` runners)

Self-hosted runners need both installed explicitly.

## Comparing two scans: gate a PR on regressions

A single scan tells you a PR's current state; it can't tell you whether the
PR made things *worse*. `imneeteeshyadav98/kubepreflight/compare` is a
second, sibling composite action for exactly that — it takes two
already-produced `findings.json` files (typically a scan of the PR's base
ref and a scan of its head ref) and evaluates a configurable regression
gate: new Blocker findings, a warning policy, verdict regression, and a
minimum acceptable readiness-score movement. It never touches a cluster or
runs a scan itself — that's still the job of two ordinary `kubepreflight`
steps upstream of it.

### Quick start: gate every PR on new blockers

```yaml
name: kubepreflight-pr-gate
on:
  pull_request:

jobs:
  compare:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          ref: ${{ github.event.pull_request.base.sha }}
          path: base

      - uses: imneeteeshyadav98/kubepreflight@v0.13.0-github-action-comparison
        id: baseline
        with:
          target-version: '1.36'
          manifests: './base/deploy'
          manifests-only: 'true'
          findings-out: 'baseline-findings.json'

      - uses: actions/checkout@v7
        with:
          ref: ${{ github.event.pull_request.head.sha }}
          path: head

      - uses: imneeteeshyadav98/kubepreflight@v0.13.0-github-action-comparison
        id: current
        with:
          target-version: '1.36'
          manifests: './head/deploy'
          manifests-only: 'true'
          findings-out: 'current-findings.json'

      - uses: imneeteeshyadav98/kubepreflight/compare@v0.13.0-github-action-comparison
        with:
          baseline: ${{ steps.baseline.outputs.findings-file }}
          current: ${{ steps.current.outputs.findings-file }}
```

`baseline`/`current` accept the scan action's `findings-file` output
directly, even though it's an absolute runner path rather than a
workspace-relative one like most other inputs — the action normalizes it,
so you don't need to juggle path bookkeeping by hand just to chain the two
actions together. The two scan steps do need distinct `findings-out`
values, though: `findings-out` is always resolved against the job's
`GITHUB_WORKSPACE` root, not against whatever `path:` a nested `checkout`
step used — both scans defaulting to `findings.json` would silently write
to the exact same file, and the second scan would overwrite the first's
output before `compare` ever ran.

This example is manifest-only (no cluster credentials, safe to run on
every PR). For a live-cluster comparison — e.g. nightly, against a real
staging cluster before and after an actual upgrade — point both scan steps
at `provider`/`cluster-name`/`kubeconfig` instead, the same as any live
scan (see the sections above); `compare` itself never needs cluster
access regardless of how its two inputs were produced.

### Inputs

| Input | Required | Default | Meaning |
|---|---|---|---|
| `baseline` | yes | — | Path to the earlier scan's `findings.json` — workspace-relative, or an absolute path such as a chained `findings-file` output |
| `current` | yes | — | Path to the later scan's `findings.json`, same path rules as `baseline` |
| `fail-on-new-blockers` | no | `'true'` | Fail the gate when `current` has a new effective upgrade blocker `baseline` didn't |
| `warning-policy` | no | `'ignore'` | `ignore`, `fail_on_new` (fail only on a *new* warning), or `fail_on_any` (fail if any warning exists, new or pre-existing) |
| `fail-on-verdict-regression` | no | `'true'` | Fail the gate when the overall verdict gets strictly worse (e.g. `CLEAN` → `BLOCKED`), independent of the specific counts driving it |
| `minimum-score-delta` | no | `'0'` | Lowest readiness-score movement (`current` minus `baseline`) that still passes; `0` means the score must not drop at all |
| `comparison-out` | no | `comparison.json` | Path, relative to the workspace, to write the full comparison document |
| `gate-out` | no | `gate.json` | Path, relative to the workspace, to write the gate decision |

### Outputs

| Output | Meaning |
|---|---|
| `decision` | `pass`, `fail`, or `neutral` — see Exit policy below |
| `reasons` | Comma-separated reason codes, e.g. `NEW_BLOCKERS_DETECTED,READINESS_VERDICT_REGRESSED` (empty on `pass`) |
| `new-blockers` | Number of new effective upgrade blockers in `current` not present in `baseline` |
| `new-warnings` | Number of new Warning-severity findings |
| `current-warnings` | Total Warning-severity findings in `current`, new or pre-existing |
| `resolved-findings` | Number of findings present in `baseline` but no longer present in `current` |
| `score-delta` | Readiness-score movement, `current` minus `baseline` (negative means it dropped) |
| `comparison-file` | Path to the written `comparison.json` on the runner |
| `gate-file` | Path to the written `gate.json` on the runner |

### Exit policy

| Decision | Job outcome |
|---|---|
| `pass` | Pass |
| `fail` | Always fail |
| `neutral` | Pass — deliberately never fails a job by itself |

`neutral` means the gate found insufficient evidence to trust a
pass/fail call at all — either scan had incomplete coverage (a partial
Kubernetes/AWS/manifest collection), or the two scans ran against
different target versions (fingerprints are scoped to target version, so
a mismatch would make genuinely-unchanged findings look like a new+resolved
pair). Failing a job on `neutral` would mean blocking a merge for
something the gate couldn't actually confirm regressed — treat it as a
prompt to look at the run manually, not as a merge blocker.

### What you get in the GitHub UI

- **Step Summary**: gate decision and reasons, verdict and readiness-score
  movement, and full tables of every new and resolved finding by severity,
  rule ID, and message.
- **Annotations**: one inline error annotation per newly-introduced
  effective upgrade blocker, linked to its source file when the finding
  came from a scanned manifest. New warnings and resolved findings appear
  in the Step Summary only, not as annotations.
- **Workflow artifacts**: `comparison.json` and `gate.json` uploaded as a
  `kubepreflight-comparison` artifact on every run, including failed ones.
- **Job status**: reflects the exit policy above.

### Requirements on the runner

Same as the scan action: Docker and `jq`, both present by default on
GitHub-hosted `ubuntu-latest` runners.
