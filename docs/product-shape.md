# Product shape

KubePreflight is **CLI-first**, not CLI-only. The v0.1 core is a read-only
upgrade-readiness engine. The optional local Console reads the CLI's
`findings.json` artifact for demos, review, and evidence exploration without
connecting to Kubernetes or AWS.

## Delivery phases

1. **CLI engine** — deterministic collection, correlation, findings, reports,
   and CI-friendly exit codes.
2. **Local Console** — static browser viewer for `findings.json`; no backend,
   account, database, telemetry, or upload service.
3. **Self-hosted team mode** — scan history, multiple clusters, waivers,
   ownership, and OIDC only after UI usefulness is validated.
4. **Hosted SaaS / fleet governance** — organizations, RBAC, integrations,
   governance, and billing only after pilot demand establishes the need.

The data boundary stays artifact-first: future hosted modes should ingest
`findings.json` rather than require broad, persistent cluster credentials.

## Current non-goals

- multi-tenant SaaS infrastructure;
- billing or user management;
- direct/cloud-hosted cluster scanning;
- an in-cluster agent;
- auto-remediation.

## Validation gate

Console availability does not remove the product-validation gate. Self-hosted
team mode and hosted SaaS remain deferred until discovery calls and real pilot
commitments show demand for history, waivers, ownership, and governance.
