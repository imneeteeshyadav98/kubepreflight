# Post-PR #232 Local Certification

Date: 2026-07-29

Source commit:

```text
40d7d93857984b28d58d85c8a4e5e8a2a251b4b3
```

This directory preserves fresh local binary evidence after PR #232
(`fix: harden rule evidence, partial reports, and rollback directionality`)
merged to `master`.

## Scope

Covered:

- local binary built from current `master`
- manifest-only clean scan
- manifest-only removed-API blocker scan
- unreachable Kubernetes API partial-evidence scan
- rollback assessment provenance-gate behavior without a matching live EKS target

Not covered:

- fresh real-cluster Kubernetes validation
- reduced-RBAC service-account validation
- reduced-IAM EKS validation
- fresh real-EKS certification

Those require an available non-production Kubernetes/EKS target. At the time
this evidence was captured, no kubeconfig current context was set and the
configured AWS account had no EKS clusters in the region sweep.

## Binary

The local binary was built with:

```bash
GOCACHE=/tmp/kubepreflight-go-build-cache \
  go build -o /tmp/kubepreflight-post232/kubepreflight ./cmd/kubepreflight
```

Checksum:

```text
d2884183ae55759170d61cf5fd02640d390055528fc914ed9ce176670f7571b0
```

The binary reports `KubePreflight dev` because this was a local build without
release ldflags.

## Results

| Scenario | Exit | Key assertion |
| --- | ---: | --- |
| `manifest-clean` | 0 | Manifest API checks clean; 2 rules evaluated and 29 rules not applicable |
| `manifest-blocked` | 2 | Removed API manifest produces one blocker |
| `unreachable-cluster` | 3 | Kubernetes coverage is partial; applicable rules with required missing evidence report `insufficient_evidence` |
| `rollback-no-aws` | 1 | Rollback assessment is operator decision / insufficient evidence, not a false rollback pass |

## Evidence Layout

Each scenario directory contains:

- `command.txt`
- `exit-code.txt`
- `stdout.txt`
- `stderr.txt`
- JSON report output
- Markdown/HTML report output where requested

`environment.txt` records the source commit, branch, binary checksum, and local
binary version output.
