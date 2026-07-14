# Compatibility Catalog

The compatibility catalog is the foundation for `v0.10.0-compatibility-catalog`.
It records reviewed add-on compatibility facts in a small versioned model so
later rules can distinguish:

- known compatible
- known incompatible
- compatible but upgrade recommended
- unknown or unverifiable

PR 1 adds the model, embedded seed catalog, validation, deterministic lookup,
and version-status helpers. It does not change `ADDON-001` or `ADDON-002`
behavior yet; PR 2 wires EKS managed add-ons into the catalog.

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
