# Demo cluster

Seeds a local [kind](https://kind.sigs.k8s.io/) cluster with 7 of the 10 locked-MVP failure modes so `kubepreflight scan` has something real to find. The remaining 3 (EKS Upgrade Insights ingestion — EKS-INSIGHT-001/002/003, ADDON-001, NODE-002) are AWS/EKS-API-only and cannot be reproduced on a non-EKS cluster — those are covered by the mocked fixtures in `internal/rules/*_test.go` and `internal/collectors/aws/collector_test.go` instead.

## Why an older Kubernetes version

API-001 (deprecated API usage) needs a `PodSecurityPolicy` object to actually exist, and `policy/v1beta1` PodSecurityPolicy was removed from the API server entirely in Kubernetes 1.25 — a cluster already past that version will reject creating one outright. The cluster is pinned to `kindest/node:v1.24.15` specifically so this object can exist, which also means NODE-001 fires "for free": scanning with `--target-version 1.34` against a 1.24 cluster is a 10-minor-version kubelet skew, well past the supported n-3 window.

## Reproducing it

```bash
kind create cluster --name kubepreflight-demo --image kindest/node:v1.24.15
kubectl apply -f demo/00-namespace.yaml
kubectl apply -f demo/01-psp.yaml
kubectl apply -f demo/02-pdb001.yaml
kubectl apply -f demo/03-pdb002.yaml
kubectl wait --for=condition=Available deployment/singleton-app -n demo --timeout=90s
kubectl wait --for=condition=Available deployment/shared-app -n demo --timeout=90s

# COREDNS-001: strip the `ready` plugin from the real Corefile
kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}' \
  | grep -v '^\s*ready\s*$' > /tmp/corefile-no-ready.txt
kubectl create configmap coredns -n kube-system \
  --from-file=Corefile=/tmp/corefile-no-ready.txt \
  --dry-run=client -o yaml | kubectl apply -f -

# Apply LAST — see the warning comment in 99-webhook.yaml before running this
kubectl apply -f demo/99-webhook.yaml

kubepreflight scan --target-version 1.34 --output all
```

## `demo/99-webhook.yaml` is deliberately dangerous — read this first

It's a catch-all (`apiGroups: ["*"]`, `resources: ["*"]`), fail-closed webhook pointed at a Service with zero ready endpoints — this is intentionally the exact WH-001/WH-002 failure mode, which per the deep dive's own description of webhook blast radius is "the #1 silent upgrade killer." Two things keep it from actually bricking the demo cluster's own control loops:

- A `namespaceSelector` excludes `kube-system`/`kube-node-lease`/`kube-public`/`local-path-storage`, so kubelet heartbeats, lease renewals, and system controllers keep working. WH-001's detection only checks `apiGroups`/`resources`, not `namespaceSelector`, so this doesn't affect whether the check fires.
- `operations` is `["CREATE", "UPDATE"]`, not `"*"`.

**Apply it last, after everything else above.** Once applied, `kubectl` writes to the `default`/`demo` namespaces stop working — including deleting the webhook itself (the classic self-dependency deadlock from deep dive Section 5.2). Recovery is `kind delete cluster --name kubepreflight-demo`, never `kubectl delete`.

## Running via Docker instead of a local build

`docker compose up` works against this demo cluster too, on Linux:

```bash
docker build -t kubepreflight:local .
docker compose up
```

This mounts `~/.kube` read-only and writes `findings.json` to `./out`, using whatever context is currently active — so it'll target `kind-kubepreflight-demo` right after the `kind create cluster` step above, no extra flags needed. `docker-compose.yml` sets `network_mode: host`, which is required because kind binds its API server to `127.0.0.1` on the host — without host networking, every collector call fails with `connection refused` (confirmed by actually hitting this against a live cluster, not just inferred). This is Linux-only; see the top-level README's Install section for the macOS/Windows caveat.

For `--output all` or other flags, override the default command:

```bash
docker compose run --rm kubepreflight scan \
  --context kind-kubepreflight-demo \
  --target-version 1.34 \
  --output all \
  --findings-out /work/findings.json
```

## `sample-output/`

Captured output from an actual run against this demo cluster: `terminal-output.txt`, `findings.json`, `report.md`, `report.html`. Regenerate by following the steps above.
