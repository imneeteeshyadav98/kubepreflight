# Real EKS case study: 1.31 → 1.32

A reproducible, end-to-end walkthrough of the full KubePreflight product
story on one real, throwaway EKS cluster: pre-upgrade readiness, manual
remediation, a real control-plane upgrade, post-upgrade rollback
assessment, and CI regression gating — with every step's evidence kept.
Full plan, rationale, and success criteria:
[`docs/case-studies/eks-1.31-to-1.32.md`](../../docs/case-studies/eks-1.31-to-1.32.md).

This is validation, not a demo of a single feature — see the design doc's
"Boundary" section before fixing anything you find along the way.

## Cost and safety warning

**This creates real, billable AWS resources** (an EKS control plane and one
EC2 node). Use a sandbox/non-production AWS account, keep the cluster as
small as `cluster.yaml` already is, and **delete it as soon as the case
study is captured** — the cleanup step below is not optional.

## Prerequisites

- `aws` CLI, configured with credentials that can create/upgrade/delete an
  EKS cluster and its node group
- [`eksctl`](https://eksctl.io/)
- `kubectl`
- A local `kubepreflight` build on `PATH` (`go build -o kubepreflight
  ./cmd/kubepreflight`), or set `KUBEPREFLIGHT_BIN` to its path
- `AWS_PROFILE` and `AWS_REGION` exported to whatever sandbox account/region
  you're using (this guide assumes `us-east-1`, matching `cluster.yaml`)

## Step 1 — verify your account

```bash
aws sts get-caller-identity
aws eks list-clusters --region us-east-1
```

Confirm you're pointed at the sandbox account you expect, and that nothing
named `kubepreflight-case-study` already exists — if it does, delete it
first, don't reuse a cluster from a previous run.

## Step 2 — create the cluster

```bash
eksctl create cluster -f demo/eks-case-study/cluster.yaml
```

Takes roughly 15–20 minutes. Creates a single-node-group EKS cluster
pinned to version `1.31`.

## Step 3 — clean baseline scan

```bash
scripts/case-study/02-scan.sh clean-baseline
```

A freshly created cluster with nothing seeded yet should come back
`CLEAN`. This isn't one of the case-study document's numbered evidence
phases — it's a sanity check that the cluster itself is healthy before any
fixtures go in.

## Step 4 — seed the fixtures

```bash
scripts/case-study/01-seed-fixtures.sh
```

Applies the PDB risk pair and the unhealthy workload, then the fail-closed
webhook last (see that script's own comments on ordering). The scan-only
deprecated-API manifest (`demo/eks/manifests/old-api.yaml`) is never
applied to the cluster — it's picked up by `--manifests` in the scan step
instead.

## Step 5 — "before" evidence

```bash
scripts/case-study/02-scan.sh before
```

Expected: `BLOCKED`, firing `API-001`, `PDB-001`, `PDB-002`, `WH-001`,
`WH-002`, `WORKLOAD-001`. Writes `findings.json`/`report.md`/`report.html`
to `demo/eks-case-study/evidence/before/` — this is the case-study
document's section 3 evidence.

## Step 6 — remediate, then "after-remediation" evidence

```bash
scripts/case-study/03-remediate.sh
scripts/case-study/02-scan.sh after-remediation
scripts/case-study/06-compare.sh before after-remediation resolved-findings
```

Fixes the PDB risk and the webhook; leaves the unhealthy workload and the
scan-only API-001 manifest alone (see `03-remediate.sh`'s own comment for
why). The compare step's `gate.json` and `comparison.md` are this phase's
"resolved findings / comparison summary / score movement / CI gate
pass-fail" evidence (case-study document section 4).

## Step 7 — upgrade the control plane

```bash
scripts/case-study/04-upgrade.sh
```

Real `eksctl upgrade cluster --approve`, 1.31 → 1.32. Takes several
minutes; refreshes kubeconfig automatically when done.

## Step 8 — "after-upgrade" evidence and rollback assessment

```bash
scripts/case-study/02-scan.sh after-upgrade
scripts/case-study/05-rollback-assess.sh
scripts/case-study/06-compare.sh after-remediation after-upgrade upgrade-regression
```

The last command is the case study's central CI-gating demonstration:
does the real upgrade itself introduce any regression versus the last
known-good, pre-upgrade state? This is also the pair of `findings.json`
files the milestone's Final PR runs through the **hosted** `compare`
composite Action on a real GitHub Actions runner — not just this local
CLI invocation.

## Step 9 — cleanup (mandatory)

```bash
demo/eks-case-study/cleanup.sh
```

Deletes the webhook, both seeded namespaces, and the EKS cluster itself
(`eksctl delete cluster --wait`, several minutes).

## Step 10 — verify no clusters remain

```bash
aws eks list-clusters --region us-east-1
```

Should return an empty `clusters` list. If it doesn't, something didn't
delete cleanly — investigate before walking away, since a lingering
cluster keeps billing.

## Expected results

| Step | Phase | Result |
|---|---|---|
| 3 | clean baseline | `CLEAN` |
| 5 | before | `BLOCKED` — `API-001`, `PDB-001`, `PDB-002`, `WH-001`, `WH-002`, `WORKLOAD-001` |
| 6 | after-remediation | `BLOCKED` — `API-001`, `WORKLOAD-001` only; `resolved-findings` compare shows 4 resolved |
| 8 | after-upgrade | Same as after-remediation, plus whatever the real upgrade itself surfaces — this is the finding, not a foregone conclusion |

These are predictions from the design doc, not captured results — this
README's job is reproducibility, not the write-up. The actual captured
numbers, screenshots, and narrative live in
`docs/case-studies/eks-1.31-to-1.32.md` once a later PR in this milestone
runs all of the above for real.

## Evidence layout

```text
demo/eks-case-study/evidence/
  before/              findings.json, report.md, report.html
  after-remediation/   findings.json, report.md, report.html
  after-upgrade/       findings.json, report.md, report.html
  rollback/            rollback-assessment.json, rollback-report.md/.html
  compare/
    resolved-findings/     comparison.json, comparison.md, gate.json
    upgrade-regression/    comparison.json, comparison.md, gate.json
```

Not gitignored — this is captured evidence meant to be committed, not
scratch output (unlike `demo/README.md`'s `kind`-based demo, whose output
intentionally isn't committed because it goes stale every milestone; a
one-time real case study is exactly the kind of artifact worth freezing).

## Notes

- The webhook and PDB fixtures are `demo/eks/manifests/`'s existing,
  already-proven files, reused directly — see that directory's own
  comments for exactly how each is scoped to avoid affecting anything
  outside its own namespace.
- `CLUSTER_NAME`/`REGION` default to `kubepreflight-case-study`/`us-east-1`
  and can be overridden via environment variable in every script here, the
  same convention `demo/eks/cleanup.sh` already uses.
- This case study never sets or assumes an AWS profile for you — export
  `AWS_PROFILE` yourself before running any of the commands above.
