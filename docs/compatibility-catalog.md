# Compatibility Catalog

The compatibility catalog is the foundation for `v0.10.0-compatibility-catalog`.
It records reviewed add-on compatibility facts in a small versioned model so
later rules can distinguish:

- known compatible
- known incompatible
- compatible but upgrade recommended
- unknown or unverifiable

The first slice added the model, embedded seed catalog, validation,
deterministic lookup, and version-status helpers, and wired EKS managed
add-ons into it. A second slice extended the same catalog to live workload
add-ons discovered from cluster state directly: metrics-server, ingress-nginx,
AWS Load Balancer Controller, cert-manager, and external-dns.

For any add-on covered by the catalog (EKS managed or live workload):

- installed versions below the known minimum produce `ADDON-001` Blocker
  findings
- parseable versions that meet the minimum but are below the recommendation
  produce `ADDON-002` Warning findings
- known compatible recommended versions produce no compatibility finding
- missing catalog targets, malformed versions, custom builds, and other
  unknowns remain `ADDON-002` Warning findings

A given workload never produces both an `ADDON-001` and an `ADDON-002`
finding: `ADDON-001` owns the known-incompatible status exclusively,
`ADDON-002` explicitly skips it.

### Live workload add-ons

Live workload add-ons are identified from Deployment/DaemonSet container
images, not names or labels — `registry.k8s.io/ingress-nginx/controller`
identifies ingress-nginx regardless of what the Deployment or its namespace
happens to be named, and a workload whose name merely mentions an add-on
(e.g. a test harness called `my-ingress-nginx-test`) never matches without
the real image. This is also what keeps cert-manager's controller,
cainjector, and webhook Deployments from being conflated: each ships under a
distinct image repository, and only the controller's is catalog-tracked.

AWS Load Balancer Controller is catalogued under the `eks` provider (it only
makes sense on EKS); its catalog entry is only applied when the scan has
confirmed AWS/EKS enrichment (`--provider=eks`) — a cluster-only scan that
happens to have an ALB Controller-shaped Deployment installed falls back to
the ordinary unverifiable warning instead of trusting EKS-specific
compatibility data on an unconfirmed cluster type. Ingress controllers
without a catalog entry (traefik, haproxy-ingress, kong-ingress) keep the
original conservative unverifiable-warning behavior.

## Model

Each catalog entry includes:

- Kubernetes target version
- provider
- add-on name
- minimum compatible version
- recommended version
- source
- reference
- last verified date
- confidence

The current schema version is:

```text
compatcatalog.kubepreflight.io/v1
```

## Initial Coverage

The seed catalog contains reviewed entries for:

EKS managed add-ons (provider `eks`):

- Amazon VPC CNI
- kube-proxy
- CoreDNS
- EBS CSI driver
- EFS CSI driver
- AWS Load Balancer Controller

Live workload add-ons (provider `kubernetes`, applies to any cluster type):

- metrics-server
- ingress-nginx
- cert-manager
- external-dns

## Validation Rules

Catalog loading rejects:

- unsupported schema versions
- malformed JSON
- missing provider, add-on, source, reference, confidence, or verified date
- malformed Kubernetes target versions
- unparseable minimum or recommended versions
- minimum versions greater than recommended versions
- duplicate entries for the same provider, add-on, and Kubernetes target

Lookup normalizes provider, add-on name, and Kubernetes patch versions. Missing
entries remain unknown; callers must not infer compatibility from absence.

## Source Policy

Catalog entries must be reviewed before they affect findings. Do not
automatically scrape external sources into the production catalog without human
review. Every entry must keep source attribution and a last verified date.
