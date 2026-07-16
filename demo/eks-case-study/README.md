# Real EKS case study — fixtures

Environment and fixture design for the `v0.14.0-real-eks-case-study`
milestone. Full plan, methodology, and evidence checklist:
[`docs/case-studies/eks-1.31-to-1.32.md`](../../docs/case-studies/eks-1.31-to-1.32.md).

This directory currently holds only the cluster config and the one new
fixture this case study needs beyond what `demo/eks/` already proves:

- `cluster.yaml` — eksctl config, pinned to EKS 1.31 (upgraded to 1.32
  during case-study execution).
- `manifests/unhealthy-workload.yaml` — a pre-existing broken pod
  (`WORKLOAD-001`), the one failure mode `demo/eks/manifests/` doesn't
  already cover.

Everything else — the PDB risk pair (`PDB-001`/`PDB-002`), the fail-closed
webhook (`WH-001`/`WH-002`), and the scan-only deprecated API manifest
(`API-001`) — is reused directly from `../eks/manifests/`, unmodified. No
reason to fork proven, already-documented fixtures.

Reproducible apply/scan/upgrade/rollback/compare scripts and a `cleanup.sh`
land in the next PR of this milestone (`scripts/case-study/`) — this PR is
design only, nothing here has been run against a real cluster yet.
