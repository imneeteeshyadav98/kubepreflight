# Provider roadmap

KubePreflight's checks split into two groups:

1. **Kubernetes-plane checks** — run against the live API server and/or
   rendered manifests. These never touch a cloud provider API and already
   work identically on any Kubernetes cluster, regardless of who's hosting
   it.
2. **Cloud-plane checks** — call a specific cloud provider's read-only API
   to enrich the scan with facts the Kubernetes API can't see (upgrade
   insights, add-on compatibility, subnet/IP headroom, and similar).
   These are necessarily provider-specific.

This split is why `--provider` is optional: omit it (or use
`--provider=cluster-only`, implicit) and KubePreflight runs the
Kubernetes-plane checks against any cluster. Pass `--provider=eks` (today)
or `--provider=aks`/`--provider=gke` (planned) to additionally pull in that
cloud's enrichment checks.

## Kubernetes-plane checks — already portable, provider-independent

These run the same way no matter which cloud (or no cloud) hosts the
cluster:

| ID | Check |
| --- | --- |
| API-001 | Deprecated/removed APIs vs target version |
| PDB-001 | `disruptionsAllowed=0` on critical path |
| PDB-002 | Overlapping PDBs (incl. CoreDNS duplicate-PDB case) |
| WH-001 | Broad/catch-all fail-closed webhooks |
| WH-002 | Fail-closed webhook, no ready endpoints |
| NODE-001 | kubelet skew outside supported policy |

## EKS — current, validated

Status: **implemented and validated against a real EKS cluster** (see
[Validated on real EKS](../README.md#validated-on-real-eks) in the root
README).

`--provider eks --cluster-name <name>` adds:

| ID | Check | Data source |
| --- | --- | --- |
| EKS-INSIGHT-001/002/003 | EKS Upgrade Insights ingestion (warning/info AWS-native signal, 24-hour refresh caveat) | `eks:ListInsights`/`DescribeInsight` |
| ADDON-001 | Catalog-known EKS add-on incompatible with target version | EKS add-on inventory + compatibility catalog |
| ADDON-002 | High-impact add-on compatibility unknown or upgrade recommended (VPC CNI, kube-proxy, CoreDNS, EBS/EFS CSI, metrics-server, ingress controllers, cert-manager, external-dns) | EKS add-on inventory + compatibility catalog / live workload inventory |
| NODE-002 | Control-plane subnet IP headroom | `ec2:DescribeSubnets` |
| NET-002 | Cluster's security group or VPC no longer exists | `ec2:DescribeSecurityGroups`/`DescribeVpcs` |

## AKS — planned

Status: **`--provider aks` is recognized by the CLI and validates its
required flags (`--cluster-name`, `--resource-group`; `--subscription-id`
optional), but enrichment collection is not implemented yet.** Selecting it
today fails fast with a clear error before any Azure API call or cluster
access is attempted.

Planned enrichment, once implemented:

- Cluster and node-pool Kubernetes version discovery (AKS API).
- Available upgrade paths per node pool.
- Azure CNI subnet/IP headroom (mirrors NODE-002's EKS subnet-exhaustion
  check, adapted to Azure CNI's subnet model).
- Addon profile compatibility with the target version (AKS-managed
  add-ons, similar in spirit to ADDON-001).

Candidate rule IDs (final IDs decided when the checks are actually built):
`AKS-API-001`, `AKS-NODE-001`, `AKS-NET-001`, `AKS-ADDON-001`, `AKS-UPG-001`.

## GKE — planned

Status: **`--provider gke` is recognized by the CLI and validates its
required flags (`--cluster-name`, `--project`, `--location`), but
enrichment collection is not implemented yet.** Selecting it today fails
fast with a clear error before any GCP API call or cluster access is
attempted.

Planned enrichment, once implemented:

- Cluster and node-pool Kubernetes version discovery (GKE API).
- Release channel awareness (Rapid/Regular/Stable) and its effect on
  available upgrade targets.
- GKE deprecation insights/recommendations (GKE's own equivalent of EKS
  Upgrade Insights).
- VPC-native secondary IP range headroom (mirrors NODE-002's subnet
  headroom check, adapted to GKE's alias-IP/secondary-range model).
- Autopilot vs. Standard mode awareness, since some checks (e.g. node
  pool-level settings) don't apply the same way under Autopilot.

Candidate rule IDs (final IDs decided when the checks are actually built):
`GKE-API-001`, `GKE-NODE-001`, `GKE-RC-001`, `GKE-NET-001`, `GKE-MODE-001`.

## Design notes for implementers

- `internal/providers` holds the `Provider` interface and one subpackage
  per provider (`eks`, `aks`, `gke`). Phase 1 (this doc's current state)
  only defines `Provider.Name() string` plus a per-provider `Config` with
  `Validate()` — no `Discover*` methods yet, since those return types
  should be designed against real AKS/GKE API response shapes rather than
  guessed in advance.
- When AKS or GKE enrichment is actually built, follow the same pattern
  `internal/collectors/aws` already established: a read-only collector
  producing plain Go structs, consumed by rules that gracefully no-op when
  that provider's snapshot isn't present (see `rules.ScanContext.AWS` and
  its four consuming rules for the reference shape).
- No cloud SDK dependency should be added until a phase actually implements
  that provider's collector.
