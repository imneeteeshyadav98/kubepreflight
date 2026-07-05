# Real EKS demo

A reproducible, end-to-end walkthrough of KubePreflight against a real,
throwaway EKS cluster: create it, run a clean scan, seed known upgrade
blockers, run a worst-case scan, then delete everything.

## Cost and safety warning

**This creates real, billable AWS resources** (an EKS control plane and one
EC2 node). Use a sandbox/non-production AWS account, keep the cluster as
small as `eks-demo.yaml` already is, and **delete it as soon as you're done
testing** — Step 7 below is not optional.

## Prerequisites

- `aws` CLI, configured with credentials that can create/delete an EKS
  cluster and its node group
- [`eksctl`](https://eksctl.io/)
- `kubectl`
- `AWS_PROFILE` and `AWS_REGION` exported to whatever sandbox account/region
  you're using (this guide assumes `us-east-1`, matching `eks-demo.yaml`)

## Step 1 — verify your account

Confirm you're pointed at the sandbox account you expect, and that nothing
named `kubepreflight-demo` already exists (if it does, delete it first —
don't reuse a cluster from a previous run):

```bash
aws sts get-caller-identity
aws eks list-clusters --region us-east-1
```

## Step 2 — create the cluster

```bash
eksctl create cluster -f demo/eks/eks-demo.yaml
```

Takes roughly 15–20 minutes (control plane + one managed node group).

## Step 3 — update kubeconfig

`eksctl` does this automatically at the end of Step 2, but if you need to
point `kubectl` at it again later:

```bash
aws eks update-kubeconfig --name kubepreflight-demo --region us-east-1
```

## Step 4 — run a clean scan

```bash
./kubepreflight scan \
  --provider eks \
  --cluster-name kubepreflight-demo \
  --target-version 1.36 \
  --serve-report always
```

A freshly created cluster with nothing seeded yet should come back
`Result: CLEAN` with `AWS enrichment: true`. Stop the server (Ctrl+C) once
you've looked at the report/Console.

## Step 5 — seed worst-case resources

```bash
kubectl apply -f demo/eks/manifests/pdb-lab.yaml
kubectl wait --for=condition=available deploy/critical-app -n preflight-lab --timeout=120s
kubectl apply -f demo/eks/manifests/broken-webhook.yaml
```

`pdb-lab.yaml` must be applied first — it creates the `preflight-lab`
namespace and label that `broken-webhook.yaml`'s `namespaceSelector`
depends on.

## Step 6 — run the worst-case scan

`demo/eks/manifests/old-api.yaml` is **scan-only — never apply it to the
cluster** (see the warning comment in that file). It's included here so
`--manifests` picks it up and fires `API-001` alongside the live-cluster
findings:

```bash
./kubepreflight scan \
  --provider eks \
  --cluster-name kubepreflight-demo \
  --target-version 1.36 \
  --manifests demo/eks/manifests \
  --serve-report always
```

### Expected result

| Step | Result |
|---|---|
| Step 4 (clean) | `CLEAN`, `AWS enrichment: true` |
| Step 6 (seeded) | `BLOCKED`, firing `API-001`, `PDB-001`, `PDB-002`, `WH-001`, `WH-002` |

Counts should match exactly across `findings.json`, `report.html`, and the
Console.

## Step 7 — cleanup (mandatory)

```bash
./demo/eks/cleanup.sh
```

Deletes the webhook, the `preflight-lab` namespace, and the EKS cluster
itself (`eksctl delete cluster --wait`, several minutes).

## Step 8 — verify no clusters remain

```bash
aws eks list-clusters --region us-east-1
```

Should return an empty `clusters` list. If it doesn't, something didn't
delete cleanly — investigate before walking away, since a lingering
cluster keeps billing.

## Notes

- The webhook in `broken-webhook.yaml` is intentionally broken (fail-closed,
  zero ready backend endpoints) — that's the point, it's what triggers
  `WH-001`/`WH-002`. Its `namespaceSelector` scopes it to the
  `preflight-lab` namespace only, so it never affects `kube-system` or
  anything else in the cluster. It is therefore not labeled a global API
  write blocker; that label is reserved for selector-free catch-all write
  scope.
- `CLUSTER_NAME`/`REGION` in `cleanup.sh` default to `kubepreflight-demo`/
  `us-east-1` and can be overridden via environment variables if you
  changed them in `eks-demo.yaml`.
- This demo never sets or assumes an AWS profile for you — export
  `AWS_PROFILE` yourself before running any of the commands above.
