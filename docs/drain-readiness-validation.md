# Drain Readiness Validation

This note preserves the real-cluster validation evidence for the
`v0.8.0-drain-readiness` milestone.

## Milestone

- Tag: `v0.8.0-drain-readiness`
- Validated master commit: `4969f8d`
- Scope: DRAIN-001 through DRAIN-005, including the Drain Readiness
  scorecard category.

## Real Drain Smoke Result

KubePreflight predicted workloads that were likely to become unschedulable
during node evacuation. After a real drain, those same workloads became
Pending with the predicted scheduling reasons.

The important product finding was that `kubectl drain --dry-run=server` did
not surface the warning, while KubePreflight did. That makes the drain
readiness checks useful as a preflight signal instead of only a post-drain
diagnostic.

## Expected Signals

- Risky workloads are identified before drain begins.
- Scheduling reasons in findings match the later Pending pod conditions.
- Drain readiness findings do not depend on `kubectl drain --dry-run=server`
  warnings.
- The scorecard reports a dedicated Drain Readiness category.
- Blocker and warning behavior remains tied to observed facts rather than
  speculative failure.

## Release Artifact

After the remote tag is pushed and the release workflow passes, the expected
image is:

```text
ghcr.io/imneeteeshyadav98/kubepreflight:0.8.0-drain-readiness
```
