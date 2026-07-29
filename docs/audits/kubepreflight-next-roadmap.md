# KubePreflight Next Roadmap

Date: 2026-07-29

This roadmap is based on the read-only full-product audit after PR #232 (`40d7d93`). The highest-priority evidence-integrity and rollback-directionality gaps are now closed, so the next work should focus on documentation sync and fresh certification evidence rather than more rule-execution semantics churn.

## Priority 0 - Keep Current Safety Boundary

Do not add automatic remediation, upgrades, rollback execution, drain execution, or live admission write probes by default. The product's trust comes from being read-only and evidence-qualified.

## Priority 1 - Audit Documentation Sync

Problem: pre-#232 audit documents described rule dependency accounting, partial reports, CRD/webhook rollback directionality, and manifest-only clarity as open gaps.

Recommended PR sequence:

1. Update audit docs to use `40d7d93` as source of truth.
2. Mark PR #232-fixed items as resolved.
3. Keep remaining limitations explicit: direct controller/operator inventory, admission simulation, APIService request proof, provider expansion, and fresh real-EKS certification.
4. Keep `CONTRIBUTING.md` and audit docs isolated from runtime changes.

Acceptance criteria:

- Documentation no longer claims rule failures abort before report creation.
- Documentation no longer claims insufficient-evidence accounting is six-rule-only.
- Documentation no longer claims CRD/webhook rollback directionality is open.
- Documentation no longer claims manifest-only clean output can imply full cluster readiness.

## Priority 2 - Fresh Real-Binary Scenario Certification

Problem: PR #232 is locally and CI validated, but it should also be certified through fresh binary runs with preserved command/output evidence.

Recommended PR sequence:

1. Build the binary from current `master`.
2. Run manifest-only clean and blocker scenarios.
3. Run partial-evidence scenarios and confirm `INCOMPLETE` / exit `3`.
4. Run rollback-directionality fixtures and confirm target mismatch is unknown/operator review rather than false blocker.
5. Preserve commands, versions, checksums, JSON, Markdown, HTML, and assertions under a certification directory.

Acceptance criteria:

- Binary checksum recorded.
- Command log records exit codes.
- Outputs are sanitized where sharing outside the local machine.
- Assertions explicitly cover PR #232 behavior.

## Priority 3 - Reduced-RBAC Kubernetes Validation

Problem: reduced Kubernetes permissions are the most likely way users encounter partial evidence. The merged dependency model should be validated against an actual restricted service account.

Recommended PR sequence:

1. Create/read from a reduced-RBAC kubeconfig in a non-production cluster.
2. Run scan with full intended target/context.
3. Confirm denied collector calls produce partial coverage and per-rule `insufficient_evidence`.
4. Confirm no denied evidence path becomes false clean.
5. Preserve sanitized reports and assertion notes.

Acceptance criteria:

- Reduced permissions produce `INCOMPLETE` where required evidence is missing.
- Successful empty lists remain `evaluated`.
- No mutating Kubernetes calls are required.

## Priority 4 - Reduced-IAM EKS Validation

Problem: reduced AWS permissions are common in enterprises. EKS/AWS evidence failures should become partial/unknown/operator-review outcomes where appropriate, not false blockers or false clean results.

Recommended PR sequence:

1. Run with a deliberately reduced IAM policy in a non-production EKS account.
2. Confirm EKS/EC2 `AccessDenied` paths are represented in coverage and rule executions.
3. Confirm context-aware EKS preconditions preserve blocker behavior only when evidence is actually available and applicable.
4. Preserve sanitized reports and assertion notes.

Acceptance criteria:

- AWS collector failures are visible.
- Reduced-IAM evidence gaps do not produce false readiness.
- Read-only AWS action boundary remains `Describe*`/`List*`.

## Priority 5 - Fresh Real-EKS Certification

Problem: repository-retained v1.3.0 evidence exists, but PR #232 behavior needs fresh real-EKS certification before a release candidate is classified with full confidence.

Recommended PR sequence:

1. Run full-access EKS scan.
2. Run reduced-IAM scan.
3. Run rollback assessment using fresh findings.
4. Validate partial report behavior and CRD/webhook rollback directionality with real evidence or controlled fixtures.
5. Preserve sanitized artifacts, checksums, command logs, cluster/environment metadata, and cleanup evidence.

Acceptance criteria:

- Full-access and reduced-IAM behavior match the merged contract.
- Rollback directionality does not create false blockers.
- Cleanup is complete.
- Evidence is safe to publish.

## Priority 6 - Controller And Operator Inventory

Problem: Controller compatibility is inferred indirectly through add-on catalogs, webhook health, CRDs, APIService, and workload health.

Recommended scope:

- Start with inventory only, not blocking rules.
- Detect common controllers by namespace/name/image labels.
- Add catalog entries for cert-manager, ingress-nginx, external-dns, metrics-server, AWS Load Balancer Controller, Cluster Autoscaler/Karpenter where evidence is reliable.
- Use `operator_decision` for unknown versions.

Acceptance criteria:

- Unknown custom controllers are surfaced as inventory, not blockers.
- Known controller versions can emit warning/operator-decision findings.
- No image registry calls are required by default.

## Priority 7 - Admission And APIService Depth

Problem: Current webhook and APIService checks are strong static/status checks but not end-to-end request proof.

Read-only improvements:

- Static CA bundle parse and expiry checks.
- Service port/targetPort consistency checks where inferable.
- APIService condition age/staleness display.
- EndpointSlice topology and readiness detail.

Optional future non-default mode:

- Explicit opt-in dry-run admission probes, documented as API writes/dry-run requests, not part of default read-only mode.

Acceptance criteria:

- Default scan remains mutation-free.
- Any opt-in write/dry-run mode is impossible to trigger accidentally.

## Priority 8 - On-Prem Audit-Only Positioning

Problem: On-prem users can benefit from Kubernetes evidence but should not expect EKS-like platform lifecycle simulation.

Recommended work:

- Add a docs page: "Using KubePreflight for on-prem and self-managed clusters."
- Include examples with `--upgrade-context audit-only`.
- List supported Kubernetes API checks and unsupported infrastructure checks.
- Add a sample report interpretation section for missing cloud enrichment.

Acceptance criteria:

- On-prem support is framed as audit/gap identification.
- No provider-specific claims are made without provider evidence.

## Priority 9 - AKS/GKE Provider Roadmap

Recommended order:

1. Provider inventory only.
2. Control-plane version/status.
3. Node pool inventory and surge/headroom.
4. Managed add-on inventory.
5. Provider-native upgrade insights if APIs exist.
6. Context-aware precondition gates.

Acceptance criteria:

- AKS/GKE commands never silently run as if fully supported.
- Provider gaps show as skipped/partial/evidence unavailable, not clean.

## Priority 10 - Report Schema Contract Automation

Problem: Go and TypeScript schema logic can drift.

Recommended work:

- Generate JSON Schema from Go models or maintain checked-in schemas.
- Add canonical fixtures for findings v1.0, findings v1.1, comparison, gate, plan, rollback.
- Load the same fixtures in Go tests and Vitest.

Acceptance criteria:

- Adding an enum or field fails tests unless both Go and Console accept/render it.
- Legacy v1.0 normalization remains conservative.

## Priority 11 - External Tester Loop

Recommended work:

- Keep the First External Test Report issue template active.
- Add a "false positive" triage label and lightweight issue query.
- For every real user false blocker, create a traceable issue before fixing.
- Maintain release-note lines that distinguish false-blocker fixes from new checks.

Acceptance criteria:

- Every external report maps to a rule ID, environment, context, and evidence plane.
- False positives produce tests before or with fixes.

## Suggested Next Three PRs

1. `docs: sync audit notes after rule evidence hardening`
2. `test: certify v1.3.0 post-232 binary scenarios`
3. `test: add reduced-rbac and reduced-iam certification evidence`

Keep each PR isolated. Avoid adding new rule-execution semantics unless a regression appears. Provider expansion, Console redesign, and new rules should remain separate from certification/documentation work.
