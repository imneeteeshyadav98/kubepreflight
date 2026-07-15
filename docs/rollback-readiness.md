# Rollback Readiness

Status: planned for `v0.12.0`.

KubePreflight does not replace Amazon EKS rollback operations or AWS-native
rollback readiness insights. The rollback workflow is assessment-only: it
combines AWS eligibility evidence, AWS rollback/upgrade insights, Kubernetes
live evidence, and existing KubePreflight checks to help an operator decide
whether rollback is available, operationally ready, and preferable to
fix-forward.

The tool must not invoke rollback, update node groups, downgrade add-ons, call
`--force`, modify PDBs, or remove disruption annotations.

## Assessment Modes

`rollback plan` is the pre-upgrade posture assessment. It answers whether an
operational rollback path is likely to remain open if the upgrade introduces a
regression.

`rollback assess` is the post-upgrade readiness assessment. It evaluates the
current cluster state against EKS rollback eligibility, evidence freshness, and
operational risk before an operator decides whether to roll back or fix forward.

## Decision Layers

Rollback output keeps three decisions separate so the report cannot contradict
itself.

Eligibility:

- `eligible`
- `unavailable`
- `unknown`

Readiness:

- `ready`
- `blocked`
- `high_risk`
- `insufficient_evidence`

Recommendation:

- `rollback_preferred`
- `fix_forward_preferred`
- `operator_decision_required`
- `do_not_proceed`

KubePreflight should avoid saying rollback is "safe". Preferred wording is that
rollback is eligible and no blocking risks were detected based on currently
available evidence.

## Schema

Rollback assessments use:

```text
kubepreflight.io/rollback-assessment/v1alpha1
```

The initial model records:

- cluster identity and current/rollback target versions
- eligibility status, rollback window, source, and reason codes
- readiness status with blocker, warning, and unknown counts
- recommendation decision, confidence, and reason codes
- evidence freshness and completeness
- per-check status, evidence, and reason codes

Reason codes are deterministic constants rather than free-form classifier
output. This keeps the first implementation rules-based and reviewable.

## Scope Boundary

`v0.12.0` remains read-only. Recommended operational steps may appear in reports
or action plans, but execution remains outside KubePreflight.
