# KubePreflight Full Product Audit

Date: 2026-07-29

Scope: read-only audit of the current `master` implementation at commit `40d7d93` (`fix: harden rule evidence, partial reports, and rollback directionality (#232)`). No product code, generated assets, release metadata, tags, branches, or workflows were changed by this documentation update.

## 1. Executive Summary

KubePreflight is broadly safe for public, read-only Kubernetes/EKS upgrade-readiness assessment. The production CLI path uses Kubernetes `get`/`list` style collection and AWS/EKS/EC2 `Describe*`/`List*` APIs, then generates local reports. It does not execute upgrade, rollback, drain, delete, patch, apply, cordon, restart, or remediation operations.

The strongest product areas are deprecated API detection, context-aware upgrade gating, rule-level evidence accounting, partial report preservation, report/Console consistency, v1 schema compatibility, redaction, and rollback provenance controls. PR #232 closed the highest-priority evidence-integrity gaps: every default rule now declares explicit evidence dependencies, rule failures are rendered as partial `INCOMPLETE` reports, and CRD/target-specific webhook rollback findings are target-aware. The largest remaining gaps are now product depth gaps rather than evidence-integrity gaps: direct controller/operator inventory, deeper admission/APIService proof, provider expansion beyond EKS, and fresh real-cluster certification.

Overall recommendation: continue public beta/early production-like testing with explicit positioning: "assessment-only, evidence-qualified, not an automatic upgrade approval engine."

## 2. Audit Method

Inputs reviewed:

- CLI entrypoints: `cmd/kubepreflight`, `internal/cli`.
- Collectors: `internal/collectors/k8s`, `internal/collectors/aws`, `internal/collectors/manifest`, `internal/rollback/eks`.
- Rules: all files under `internal/rules`.
- Report, scoring, gate, compare, redaction, rollback, catalogs, Console, GitHub Action, workflows, scripts, docs, and v1.3.0 certification evidence.

Commands run:

| Command | Result |
| --- | --- |
| `git diff --check` | pass |
| `gofmt -l .` | pass, no output |
| `GOCACHE=/tmp/kubepreflight-go-build-cache go test ./...` | pass after rerun outside sandbox for loopback-listener tests |
| `GOCACHE=/tmp/kubepreflight-go-build-cache go test -race ./...` | pass |
| `go vet ./...` | pass |
| `npm --prefix web test` | first run had one 5s timeout in large comparison test; immediate rerun passed 217/217 |
| `npm --prefix web run build` | pass |
| `scripts/check-console-dist.sh` | pass |
| `scripts/check-api-version-catalog.sh` | pass, 44 entries |
| `scripts/check-compatibility-catalog.sh` | pass |
| `scripts/check-v1-compatibility-contract.sh` | pass, 8 commands and 31 rule IDs |
| `govulncheck ./...` | pass for reachable code after network/cache rerun |

This audit did not run a new real EKS cluster. Real-EKS claims below are based only on repository-retained v1.3.0 certification evidence under `docs/certification/v1.3.0/`.

## 3. Architecture

The product shape is clean:

- `internal/cli`: user commands, flags, exit-code mapping, report write lifecycle.
- `internal/collectors/k8s`: live Kubernetes snapshot collection through client-go and dynamic clients.
- `internal/collectors/aws`: EKS/EC2 enrichment for cluster metadata, add-ons, node groups, network preconditions, and EKS Upgrade Insights.
- `internal/collectors/manifest`: raw YAML and Helm-template manifest scanning.
- `internal/rules`: deterministic rule engine over collected snapshots.
- `internal/findings`: canonical report schema, severity/gate/priority, fingerprints, scoring, coverage.
- `internal/comparison` and `internal/gate`: scan diffing and CI gate decisions.
- `internal/rollback`: rollback eligibility/readiness model and provenance gates.
- `internal/report` and `web`: terminal/Markdown/HTML/Console renderers.

This is an appropriate architecture for an assessment product. The source of truth is the Go report model; renderers consume the same fields rather than recomputing most gate decisions.

## 4. Read-Only Boundary

Confirmed production read-only calls:

- Kubernetes collector uses list/get style calls for Nodes, Pods, PDBs, webhooks, Services, EndpointSlices, CRDs, workloads, PV/PVCs, APIService, deprecated APIs, and the allowlisted CoreDNS ConfigMap.
- AWS collector uses `eks:DescribeCluster`, `eks:ListInsights`, `eks:DescribeInsight`, `eks:ListAddons`, `eks:DescribeAddon`, `eks:DescribeAddonVersions`, `eks:ListNodegroups`, `eks:DescribeNodegroup`, `ec2:DescribeSubnets`, `ec2:DescribeSecurityGroups`, `ec2:DescribeVpcs`.
- Rollback collector uses EKS `DescribeCluster`, `ListInsights`, `DescribeInsight`, `ListUpdates`, `DescribeUpdate`, `DescribeClusterVersions`.
- Report server binds local HTTP for generated reports; this is local presentation, not cluster mutation.

Mutating commands exist in demo, certification setup, remediation examples, and deployment docs, not in production assessment logic. Examples include `demo/live-eks/remediate.sh`, `demo/live-eks/destroy.sh`, case-study seed/remediate scripts, and `deploy/README.md` instructions to install RBAC. These are outside the CLI collector/rule path and should stay clearly labeled.

## 5. CLI Behavior

Actual command behavior sampled with `/tmp/kubepreflight-audit/kubepreflight`:

| Scenario | Command shape | Exit | Output |
| --- | --- | ---: | --- |
| Clean manifest-only scan | `scan --manifests-only --manifests valid --target-version 1.30` | 0 | `findings.json`, no findings |
| Removed API manifest | `scan --manifests-only --manifests testdata/manifest-repo/raw --target-version 1.25 --output all` | 2 | JSON, Markdown, HTML; `API-001` blocker |
| Malformed YAML | `scan --manifests-only --manifests bad --target-version 1.30` | 3 | report written, manifest coverage partial |
| Empty manifest directory | `scan --manifests-only --manifests empty --target-version 1.30` | 0 | clean manifest-only report |
| Missing target | `scan --manifests-only --manifests valid` | 1 | usage error |
| Invalid target | `--target-version nope` | 1 | parse error |
| Missing kubeconfig path | `scan --kubeconfig /tmp/nope` | 4 | infrastructure/config failure |
| Unreachable local API endpoint | fake kubeconfig to `127.0.0.1:9` | 3 | `INCOMPLETE`, Kubernetes coverage partial, report written |
| Output path is a file | `--output-dir /tmp/not-a-dir` | 4 | artifact write/setup failure |
| Compare clean vs blocked with target mismatch | `compare --baseline valid --current removed --gate-out` | 0 | comparison written, gate `neutral`, reason `INSUFFICIENT_EVIDENCE` |
| Rollback unsupported provider | `rollback assess --provider local` | 1 | usage error |
| Rollback EKS with no AWS evidence | `rollback assess --provider eks --cluster-name dummy` | 1 | assessment JSON written, operator decision required |

Exit-code contract is consistent with implementation:

- 0: clean scan or rollback preferred.
- 1: warnings/operator decisions or usage/generic error depending command path.
- 2: effective blockers / do-not-proceed rollback.
- 3: incomplete scan coverage with a report.
- 4: infrastructure/config/report-write failure where no fully trustworthy artifact set should be assumed.

## 6. Rule Coverage

The default registry contains 31 rules. Manifest-only mode runs only `API-001` and `API-002`; v1.3.0 now records the other 29 as `not_applicable`/`not_evaluated`, which fixes the earlier silent absence problem for compare and presentation.

Observed manifest-only clean report:

- 2 rules: `applicable`/`evaluated`.
- 29 rules: `not_applicable`/`not_evaluated`.
- Kubernetes/AWS coverage: `skipped`.
- Manifests coverage: `complete`.

After PR #232, every default rule exposes a rule-owned dependency contract. Required evidence failures now map to `insufficient_evidence`, while successful empty inventories remain `evaluated`. Conditional rules such as `WH-002`, `CRD-002`, and `DRAIN-001` only require deeper evidence after already-collected resources prove it is relevant.

## 7. Context-Aware Gating

The `--upgrade-context` flag supports:

- `audit-only`
- `control-plane-only`
- `worker-rollout`
- `full-platform-upgrade`
- `workload-restart`
- `unspecified`

Context helpers are centralized in `internal/rules/contextual.go`:

- Drain/PDB risks block only worker rollout or full-platform contexts.
- EKS control-plane preconditions block only control-plane-only or full-platform contexts.
- Add-on compatibility uses compatibility-catalog operational impact metadata.
- Current workload health uses operator-decision semantics unless critical infrastructure plus activation context makes a block appropriate.

This is materially better than global blocker semantics and aligns with recent false-positive fixes.

## 8. Scoring And Gate Semantics

`Report.Result()` and `Report.ExitCode()` share `resultAndExitCode()` in `internal/findings/report.go`. Incomplete coverage outranks blockers for the top-level result, while individual blockers remain visible in findings and summary.

`UpgradeReadiness` is derived from findings and the report result. `UpgradeContinue` is false for incomplete scans, blockers, and operator decisions. This is conservative and appropriate for CI and human review.

Readiness score does not penalize unevaluated rules directly. Renderers now qualify scores when coverage is partial, unavailable, or normalized legacy. This is acceptable, but the score must continue to be presented as "based on evaluated findings," not as an absolute probability of safety.

## 9. Report And Schema Contract

Findings schema is `1.1`, additive over `1.0`. Important stable fields remain:

- `ruleId`, `severity`, `confidence`, `message`, `resources`, `evidence`, `remediation`, `priority`, `affectedScope`, `canUpgradeContinue`, `fingerprint`.
- v1.3.0 adds `ruleExecutions` and `ruleExecutionsNormalized`.

The compatibility contract check passes and compare normalizes legacy reports conservatively. Fingerprints are stable across additive metadata because `RuleExecutionRecord` is report-level, not finding-level.

## 10. Compare And CI Gate

Compare now distinguishes not-re-evaluated findings from resolved findings using `RuleExecutions`. Gate output includes evaluation coverage. A clean-vs-blocked sample with mismatched target versions produced a neutral gate with `INSUFFICIENT_EVIDENCE`, which is safer than false pass/fail.

Residual caution: target or context mismatches can still make diffs less meaningful. The tool emits warnings and neutral gate decisions, but docs should keep advising same target/context for remediation verification comparisons.

## 11. Rollback Readiness

Rollback assessment is read-only and EKS-only. It combines:

- EKS rollback/update eligibility.
- EKS Upgrade Insights.
- Optional existing `findings.json`.
- Operational readiness checks for node groups, add-ons, workloads, disruption, reverse compatibility, and coverage.

Confirmed strengths:

- PDB/DRAIN findings no longer force rollback failure unless rollback activation evidence exists.
- `DRAIN-005` is routed through workload health instead of disruption readiness.
- API findings are trusted only when findings target matches rollback target.
- Live-cluster evidence is gated by cluster identity and freshness.
- Stale or wrong-cluster findings become unknown/operator-decision evidence, not hard blockers.

PR #232 changed CRD and target-specific admission-webhook rollback consumption to validate findings against the rollback target. Directionally unproven CRD/webhook evidence becomes unknown/operator-review evidence instead of a confirmed rollback blocker. `WH-002` remains separately preserved as current live admission-backend health risk because it is not merely forward-version compatibility evidence.

## 12. EKS Support

EKS support is real and meaningful:

- Cluster metadata and support type.
- Network/IP preconditions.
- Managed add-on inventory and compatibility.
- Managed node group inventory and health.
- EKS Upgrade Insights.
- Rollback eligibility from EKS update history and cluster versions.

The repo includes real v1.3.0 certification artifacts for full access, reduced IAM, manifests-only, and compare flows. This audit did not recreate that EKS run.

## 13. On-Prem And Non-EKS Clusters

Non-EKS clusters are supported for Kubernetes API based audit-only and cluster-only scans. The tool can evaluate deprecated APIs, webhooks, PDBs, drain risks, node skew, workload health, CoreDNS, CRDs, and APIServices where the Kubernetes collector has permissions.

Non-EKS limitations:

- No provider preconditions equivalent to EKS subnet/security-group checks.
- No AKS/GKE enrichment yet; providers are recognized but not implemented.
- No cloud-specific add-on compatibility beyond EKS-managed and catalog-driven/self-managed signals.
- No infrastructure lifecycle knowledge for on-prem node replacement, control-plane rebuilds, load balancers, CNI dataplane internals, etc.

Correct positioning for on-prem: "audit-only gap identification from Kubernetes evidence," not a complete platform upgrade simulator.

## 14. Security And Redaction

Redaction covers AWS ARNs, account IDs, AWS infrastructure IDs, EKS endpoints, tokens, IPs, hostnames, and local paths across reports, comparisons, rollback assessments, and terminal output without changing fingerprints, scores, gates, recommendations, or exit codes. README warns users to redact before sharing external evidence.

`govulncheck` found no reachable vulnerabilities in code. It did report one vulnerability in required modules that does not appear reachable.

GitHub workflows and release workflow include security-related checks and image scan/SARIF flows. No telemetry phone-home behavior was found in the OSS core.

## 15. Console

The Console is feature-complete for imported reports, compare state, rule-execution coverage, priority sorting, action summaries, EKS inventory, and rollback schemas. Web tests passed on rerun and build output is up to date.

Residual risk: some schema/presentation logic is duplicated in TypeScript (`web/src/lib/*`) and Go. Tests reduce drift risk, but every schema/gate addition should include paired Go and Console tests.

## 16. Documentation And Claims

README and docs now mostly match implementation:

- v1.3.0 evidence integrity and evaluation semantics are described.
- v1.1.0 context-aware gating is described.
- v1.2.1 artifact write exit code behavior is described.
- Read-only permissions are listed.
- Manifests-only mode is explained.

One positioning rule should remain strict: avoid saying "all categories passed" without displaying or linking evaluation coverage. A manifest-only clean scan is clean only for manifest API checks, not for the whole cluster.

## 17. Known Mutating Surfaces Outside Product

These are expected but should stay clearly outside the assessment boundary:

- Demo/case-study cluster setup and teardown scripts.
- Demo remediation scripts.
- Certification cluster creation/deletion and IAM setup.
- RBAC/IAM deployment manifests users apply manually.
- Remediation text that suggests `kubectl patch`, `apply`, `scale`, or delete as operator actions.

None of these are invoked by `kubepreflight scan`, `plan`, `compare`, or `rollback`.

## 18. Findings

### AUD-001 - P2 - Rule errors now produce user-visible partial reports

Status: Resolved in `40d7d93` / PR #232.

Confidence: Confirmed fixed by post-merge tests.

Affected files: `internal/rules/rule.go`, `internal/cli/scan.go`, `internal/cli/plan.go`, `internal/cli/rule_error_scope_test.go`.

Previous behavior: `RunAllWithExecutions` could create a `State: failed` record for a rule error, but scan/plan returned immediately on `err` before constructing a report. JSON, Markdown, HTML, terminal, and Console never received the partial findings or failed execution record.

Current behavior: rule errors no longer discard completed findings and execution records. Scan and plan preserve partial reports, failed rules appear in `ruleExecutions`, report result is `INCOMPLETE`, and a successfully written incomplete report returns exit code `3`.

Evidence: `internal/cli/rule_error_scope_test.go` covers incomplete report writing and exit-code precedence; `internal/rules/execution_test.go` covers failed execution records and continuation after independent rule failures.

Operational impact: Closed. Operators now receive a partial report showing which checks completed and which rule failed.

Fix implemented: partial-report-on-rule-error contract with explicit `failed` rule records, sanitized user-visible failure reasons, and exit code precedence where artifact write failures still return exit code `4`.

Regression coverage: keep tests that simulate one failing rule after successful rules and assert rendered JSON/Markdown/HTML/terminal behavior remains incomplete rather than abort-only.

### AUD-002 - P2 - Insufficient-evidence accounting covers default rules

Status: Resolved in `40d7d93` / PR #232.

Confidence: Confirmed fixed by post-merge tests.

Affected files: `internal/rules/execution.go`, `internal/rules/*`, `internal/report/evaluation_coverage.go`.

Previous behavior: Only six silent-skip rules were mapped to collector error keys for `insufficient_evidence`; `ADDON-002` emitted its own uncertainty finding. Other rules could show `evaluated` when their relevant collector data failed, because the registry could not know their evidence dependencies.

Current behavior: every default rule declares explicit evidence dependencies through rule-owned metadata. Applicable rules with missing required evidence report `insufficient_evidence`. Rules outside a scan mode report `not_applicable`/`not_evaluated`.

Evidence: `internal/rules/execution_test.go` includes dependency-contract completeness checks, conditional dependency tests for `WH-002`/`CRD-002`/`DRAIN-001`, and relevant missing-evidence tests.

Operational impact: Closed for the default registry. Per-rule execution state now distinguishes evaluated, insufficient evidence, failed, and not applicable.

Fix implemented: `DependencyRule` / `ContextDependencyRule` contracts plus invariant tests that every default rule appears exactly once and declares dependency metadata.

Regression coverage: for each collector key failure, assert every dependent rule transitions to `insufficient_evidence` or emits an intentional uncertainty finding.

### AUD-003 - P2 - CRD/Webhook rollback directionality is target-aware

Status: Resolved in `40d7d93` / PR #232.

Confidence: Confirmed fixed by post-merge tests.

Affected files: `internal/rollback/operational.go`, `internal/rollback/operational_test.go`.

Previous behavior: API findings were target-gated for rollback. CRD/WH findings were identity/freshness-gated, then consumed as rollback reverse-compatibility evidence without checking whether their forward target semantics applied to the rollback target.

Current behavior: CRD and target-specific webhook findings are validated against the rollback target. Target mismatch or unknown provenance becomes unknown/operator-review evidence instead of a confirmed rollback blocker. `WH-002` current backend failure remains visible independently of target mismatch.

Evidence: `internal/rollback/operational_test.go` covers CRD/webhook target mismatch behavior and `WH-002` current-health preservation.

Operational impact: Closed. A forward upgrade finding cannot create a confirmed CRD/webhook rollback blocker when its target relevance is not proven.

Fix implemented: CRD/webhook rollback target validation similar to API target validation, with separate current-health handling for `WH-002`.

Regression coverage: forward target 1.36 findings supplied to rollback target 1.34 must not hard-fail reverse compatibility unless the reverse impact is recomputed or independently evidenced.

### AUD-004 - P2 - No direct custom controller/operator compatibility inventory

Confidence: Design concern.

Affected files: `internal/rules`, `internal/compatcatalog`, `internal/collectors/k8s`.

Observed behavior: Controller risk is inferred through webhooks, CRDs, APIService health, workloads, and add-on catalogs. There is no generic inventory of controller images, operator versions, leader-election leases, controller-runtime compatibility, or installed operator packages.

Expected behavior: For production upgrade readiness, major controllers/operators should be assessed directly where possible.

Evidence: Rule inventory contains no generic controller/operator version rule beyond cataloged add-ons and workload health.

Operational impact: A controller can be running and healthy but still incompatible with the target Kubernetes API behavior.

Recommended fix: Add a later controller inventory capability, starting with known namespaces/deployments and opt-in catalog mapping.

Suggested tests: Fixture deployments for cert-manager, ingress-nginx, external-dns, metrics-server, and unknown custom controllers.

### AUD-005 - P3 - Admission webhook checks are static/observed health checks, not admission simulations

Confidence: Confirmed.

Affected files: `internal/rules/wh001.go`, `wh002.go`, `wh004.go`, `wh005.go`.

Observed behavior: Webhook rules inspect failure policy, matching scope, client config, service references, ports, and EndpointSlice readiness. They do not perform dry-run API writes through admission, validate TLS handshake behavior, or check certificate expiry/CA bundle correctness end to end.

Expected behavior: Keep current rules, but position them as strong preflight indicators, not proof that every admission path succeeds.

Evidence: No dynamic admission request execution exists in production CLI path, consistent with read-only/no-write constraints.

Operational impact: A webhook can look structurally healthy while failing TLS or request handling for some admission paths.

Recommended fix: Consider an optional, explicitly documented, non-default dry-run admission probe mode only if the project later allows controlled API writes. Otherwise add static TLS/cert linting where possible.

Suggested tests: CA bundle invalid/expired fixtures and webhook service TLS mismatch fixtures if static inspection is added.

### AUD-006 - P3 - APIService checks rely on availability status, not request-level proof

Confidence: Confirmed.

Affected files: `internal/rules/apiservice001.go`, `internal/collectors/k8s/collector.go`.

Observed behavior: `APISERVICE-001` flags unavailable aggregated APIs from APIService availability evidence. It does not query each aggregated API endpoint or validate serving cert/TLS behavior.

Expected behavior: Keep as observed Kubernetes status unless stronger read-only probes are explicitly scoped.

Evidence: Rule message says this does not by itself prove the Kubernetes version upgrade will fail and sets `operator_decision`.

Operational impact: Some aggregated API failures may be missed if status is stale or incomplete.

Recommended fix: Add optional discovery/request probes where safe and read-only.

Suggested tests: APIService status stale vs unavailable vs missing-service fixtures.

### AUD-007 - P3 - Manifests-only clean output is scoped to manifest API checks

Status: Resolved in `40d7d93` / PR #232.

Confidence: Confirmed.

Affected files: `internal/rules/defaults.go`, `internal/report/*`, `web/src`.

Previous behavior: v1.3.0 correctly recorded 29 non-applicable rules in manifest-only scans, but summary categories could still read "Passed" unless the reader noticed evaluation coverage.

Current behavior: terminal, Markdown, HTML, and Console clean states now explicitly say manifest API checks are clean and that live cluster/AWS/scheduling/disruption/add-on/node/CRD/webhook checks were not evaluated in manifest-only mode.

Evidence: Clean manifest-only sample had 2 evaluated rules and 29 not-applicable/not-evaluated records.

Operational impact: Closed for the clean manifest-only path. The output no longer implies full cluster readiness.

Fix implemented: shared manifest-only clean notice plus renderer and Console tests.

Regression coverage: snapshot and Console tests cover manifest-only summary labels in terminal, Markdown, HTML, and Console.

### AUD-008 - P3 - Console schema logic can drift from Go logic

Confidence: Design concern.

Affected files: `web/src/lib/*`, `internal/findings`, `internal/comparison`, `internal/gate`, `internal/rollback`.

Observed behavior: TypeScript validators and presentation helpers mirror Go schema and semantics. Tests are strong, but this remains a duplicate implementation.

Expected behavior: Schema changes should be generated or contract-tested across Go and TS.

Evidence: Web schema tests exist for findings, comparison, plan, rollback; v1 contract script checks Go side.

Operational impact: A future additive field or enum could render differently in Console than terminal/Markdown/HTML.

Recommended fix: Add JSON schema generation or golden cross-render fixtures consumed by both Go and TS.

Suggested tests: One canonical fixture per schema version loaded by Go tests and Vitest.

### AUD-009 - P4 - Demo and certification mutation boundaries require continued labeling

Confidence: Confirmed.

Affected files: `demo/*`, `scripts/case-study/*`, `docs/certification/v1.3.0/*`, `deploy/*`.

Observed behavior: Non-product scripts apply/delete/scale resources for demos, remediation examples, certification, or cleanup.

Expected behavior: These should remain clearly marked as setup/remediation/cleanup, never as part of `kubepreflight scan`.

Evidence: Source search found mutations only in helper/docs/workflow contexts, not production collectors.

Operational impact: External reviewers may confuse demo mutation with product behavior if docs are skimmed.

Recommended fix: Keep "KubePreflight itself is read-only; this script creates/removes demo fixtures" headers in every mutating helper.

Suggested tests: Keep `scripts/live-eks/verify-read-only.sh` and add docs linting for demo disclaimers if needed.

## 19. Release Readiness Assessment

Current `master` is technically healthy for a documentation/audit checkpoint:

- Go, race, vet: pass.
- Web tests/build: pass after one timeout rerun.
- Catalog and v1 compatibility checks: pass.
- Reachable vulnerability scan: pass.
- Git status remains clean for tracked files; pre-existing untracked `CONTRIBUTING.md` and `docs/audits/` are intentionally handled outside the implementation PR.

This audit should not be treated as a new release certification because no fresh real EKS cluster was created in this session.

## 20. Decision

Ship posture: suitable for public testing and production-like read-only assessments with evidence-qualified language.

Do not position as:

- Automatic upgrade approval.
- Complete on-prem platform lifecycle simulator.
- Full controller/operator compatibility scanner.
- Rollback executor.

Position as:

- Read-only upgrade readiness audit.
- Context-aware risk classification.
- Evidence-aware compare and CI gate.
- EKS-enriched where AWS evidence is available.
- Useful for on-prem gap identification from Kubernetes API evidence.

## 21. Follow-Up Artifacts

Companion documents:

- `docs/audits/kubepreflight-capability-matrix.md`
- `docs/audits/kubepreflight-rule-inventory.md`
- `docs/audits/kubepreflight-next-roadmap.md`
