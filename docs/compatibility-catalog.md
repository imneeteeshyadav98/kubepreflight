# Compatibility Catalog

The compatibility catalog is the foundation for `v0.10.0-compatibility-catalog`.
It records reviewed add-on compatibility facts in a small versioned model so
later rules can distinguish:

- known compatible
- known incompatible
- compatible but upgrade recommended
- unknown or unverifiable

The first slice added the model, embedded seed catalog, validation,
deterministic lookup, and version-status helpers. EKS managed add-ons now use
the catalog for deterministic decisions where reviewed entries exist.

For EKS managed add-ons covered by the catalog:

- installed versions below the known minimum produce `ADDON-001` Blocker
  findings
- parseable versions that meet the minimum but are below the recommendation
  produce `ADDON-002` Warning findings
- known compatible recommended versions produce no compatibility finding
- missing catalog targets, malformed versions, custom builds, and other
  unknowns remain `ADDON-002` Warning findings

The catalog currently affects EKS managed add-ons only. Live workload add-ons
such as metrics-server, ingress controllers, cert-manager, and external-dns
remain conservative unverifiable warnings until a later slice wires them into
deterministic catalog matching.

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

- Amazon VPC CNI
- kube-proxy
- CoreDNS
- EBS CSI driver
- EFS CSI driver
- metrics-server

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
