# Live EKS demo (recording source)

A reproducible, end-to-end sequence used to produce the "See it in action"
demo GIFs: create a real, throwaway EKS cluster, seed known blockers, run a
real scan, remediate, run a second real scan, compare, verify the captured
evidence is safe, then destroy everything.

Distinct from [`demo/eks/`](../eks/README.md) (a manual single-scan
validation walkthrough, no remediation/comparison phase) and
[`demo/eks-case-study/`](../eks-case-study/README.md) (pinned to the
historical `1.31 -> 1.32` real-upgrade case study, with a real
`eksctl upgrade cluster` step and rollback assessment). This demo
deliberately does **not** perform a real control-plane upgrade or rollback
— see "Scope" below.

## Cost and safety warning

**This creates real, billable AWS resources** (an EKS control plane and one
EC2 node, roughly 15-20 minutes to provision). Use a sandbox/non-production
AWS account, keep the cluster as small as `cluster.yaml` already is, and
**run `destroy.sh` as soon as recording is done** — it is not optional.
`create.sh` refuses to run if a cluster with this name already exists, so a
leftover cluster from a previous run is never silently reused.

## Scope

This demo is scan → remediate → scan → compare only. It does not run a real
`eksctl upgrade cluster` or rollback assessment — mixing a live control-plane
upgrade into the same recording session adds real risk and real time for a
separate story (upgrade/rollback) this demo isn't telling. See
`demo/eks-case-study/` for the real-upgrade walkthrough.

## Prerequisites

- `aws` CLI, configured with credentials that can create/delete an EKS
  cluster and its node group
- [`eksctl`](https://eksctl.io/)
- `kubectl`
- A local `kubepreflight` build on `PATH` (`go build -o kubepreflight
  ./cmd/kubepreflight`), or set `KUBEPREFLIGHT_BIN` to its path
- `AWS_PROFILE`/`AWS_REGION` exported to whatever sandbox account/region
  you're using (this guide assumes `us-east-1`, matching `cluster.yaml`)

## Steps

```bash
# 1. Create the cluster (~15-20 min). Confirms account identity and that
#    no cluster with this name already exists before creating anything.
./demo/live-eks/create.sh

# 2. Seed the "before" fixtures (reuses demo/eks/manifests/ and
#    demo/eks-case-study/manifests/ — already-proven, not reinvented).
./demo/live-eks/apply-before.sh

# 3. Real scan against the live cluster. Expected: BLOCKED, firing
#    API-001, PDB-001, PDB-002, WH-001, WH-002, WORKLOAD-001.
./demo/live-eks/scan.sh before

# 4. Remediate the PDB pair and the fail-closed webhook. Leaves
#    WORKLOAD-001 and the scan-only API-001 manifest untouched on purpose
#    (a demo where everything resolves isn't representative).
./demo/live-eks/remediate.sh

# 5. Second real scan. Expected: still BLOCKED (API-001/WORKLOAD-001
#    remain), but PDB-001/PDB-002/WH-001/WH-002 no longer fire.
./demo/live-eks/scan.sh after

# 6. Real comparison between the two scans.
./demo/live-eks/compare.sh

# 7. Verify the captured evidence before recording/committing anything:
#    expected finding IDs present, no account ID/ARN/IP/private hostname
#    anywhere in the evidence, before/after are the same cluster, reports
#    are non-trivial, gate decision is internally consistent.
./demo/live-eks/verify-expected-output.sh

# 8. Record terminal + report/Console screenshots from
#    demo/live-eks/evidence/{before,after,compare}/ now, before teardown.

# 9. Destroy everything (mandatory). Removes any LoadBalancer Services
#    first, then the seeded fixtures, then the cluster itself, then
#    verifies the cluster is actually gone.
./demo/live-eks/destroy.sh
```

## Expected results

| Phase | Result |
|---|---|
| `before` | `BLOCKED`, firing `API-001`, `PDB-001`, `PDB-002`, `WH-001`, `WH-002`, `WORKLOAD-001` |
| `after` | `BLOCKED`, firing `API-001`, `WORKLOAD-001` only — `PDB-001`, `PDB-002`, `WH-001`, `WH-002` resolved |
| `compare` gate | `pass`, `newBlockers: 0`, `resolvedFindings > 0` |

## Evidence directory

`demo/live-eks/evidence/` is gitignored — it's a fresh, real capture every
time this is run, not a frozen fixture (same reasoning as
`demo/README.md`'s "Output isn't committed" section). Only the GIFs
produced from a run, not the raw evidence itself, get committed elsewhere
in this repo.
