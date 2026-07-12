# Drain Readiness Validation

This note preserves the real-cluster validation evidence for the
`v0.8.0-drain-readiness` milestone. The release tag remains locked to the
validated commit; this document is a post-release evidence record.

## Milestone

- Tag: `v0.8.0-drain-readiness`
- Validated commit: `4969f8d`
- Scope: DRAIN-001 through DRAIN-005, including the Drain Readiness scorecard
  category.

## Environment

Validation was run against a multi-node Kubernetes cluster so a real
non-dry-run node drain could exercise scheduler behavior after eviction. The
cluster contained purpose-built risky workload fixtures alongside ordinary
workloads.

The validation compared three signals:

- KubePreflight findings before the drain.
- `kubectl drain --dry-run=server` behavior before the drain.
- Pod status and scheduler reasons after the real drain.

## Risky Workload Fixtures

The fixture set included workloads designed to stress drain-readiness checks:

- Workloads constrained to a limited node set through hard scheduling rules.
- Workloads with insufficient spare schedulable capacity after one node is
  removed from service.
- Workloads whose evacuation risk is visible before the drain but only becomes
  operationally obvious after pods are evicted.

## KubePreflight Predictions

Before draining, KubePreflight reported drain-readiness findings for the risky
workloads. The findings identified the workloads that were likely to fail
rescheduling and included the expected scheduling constraints or capacity
reasons.

Expected pre-drain signals:

- Risky workload fixtures produced Drain Readiness findings.
- Findings were attached to the affected workload resources.
- Evidence included scheduler-relevant constraints or capacity context.
- The Drain Readiness scorecard category reflected the findings.

## Real Drain Result

After a real non-dry-run node drain, the same risky workloads became Pending.
Their pod conditions matched the reasons KubePreflight predicted before the
drain.

Observed Pending reason patterns included scheduler messages equivalent to:

```text
node(s) didn't match Pod's node affinity/selector
Insufficient cpu
Insufficient memory
```

The important validation result is that KubePreflight predicted the risky
workloads before eviction, while the later Pending pods confirmed the same
scheduling constraints after the drain.

## Dry-Run Comparison

`kubectl drain --dry-run=server` did not warn about the same rescheduling
failure modes. That difference is the product value of the drain-readiness
checks: KubePreflight can surface scheduler and spare-capacity risks before an
operator starts a drain, instead of relying on post-eviction Pending pods as
the first clear signal.

## Cleanup

After validation:

- Drained nodes were uncordoned where appropriate.
- Risky workload fixtures were removed.
- Any intentionally constrained fixture resources were deleted.
- The cluster was checked for leftover Pending pods from the validation run.

## Limitations

This validation demonstrates that the drain-readiness checks can predict real
rescheduling failures for the covered fixture classes. It does not guarantee
that every future scheduler plugin, autoscaler behavior, topology policy, or
provider-specific disruption condition will be predicted.

The checks should be treated as a preflight risk signal that improves operator
visibility before drain, not as a formal proof that every node evacuation will
succeed.
