# Confirmed Defects — Post-#232 Real-Environment Certification

Four real, confirmed defects were found during this certification. The original certification commit was evidence-only; the statuses below track the follow-up local fix state. This file is the authoritative index; see the linked lane reports for full derivation and evidence.

---

## RED-TERMINAL-001 — Terminal output redaction is broken

**Severity:** P1 / High
**Status:** fixed locally, real-binary verified; fresh real-EKS re-certification pending
**Evidence:** `07-redaction/report.md`, `02-reduced-iam-eks/report.md`

`scan --help`'s `--redact-sensitive-identifiers` description explicitly names "terminal" as a covered output format, alongside `findings.json`/`report.md`/`report.html`. It is not. A clean per-profile sweep across all 6 Lane 2 IAM profiles showed **0 raw account-ID/ARN matches in every `findings.json` and `report.md`**, and **2–7 raw matches in every corresponding `stdout.txt`**, for the identical run, with the identical flag passed.

This is not cosmetic. Terminal output is the one format users routinely copy into tickets, Slack, GitHub issues, and CI logs — exactly the documented use case for `--terminal-output full`. Clean file-output redaction can create false confidence that the terminal stream is equally safe, when it is not.

**Acceptance criteria for the fix:**
- Terminal output uses the same effective redaction policy as JSON/Markdown/HTML.
- ANSI formatting does not bypass pattern detection.
- Both stdout and stderr are deliberately assessed, not assumed.
- `scan`, `plan`, `compare`, `rollback plan`, and `rollback assess` are all covered.
- Exit codes, fingerprints, scores, gate decisions, and rule-execution states remain unchanged by the fix (as they already correctly do for file outputs).
- Real-binary and real-EKS regression evidence retained for the fix, not just unit tests.

---

## RED-CLOUD-ID-002 — No redaction pattern for AWS infrastructure IDs

**Severity:** P2 / Medium
**Status:** fixed locally, real-binary verified; fresh real-EKS re-certification pending
**Evidence:** `07-redaction/report.md`

`internal/redact/redact.go` has exactly three patterns (ARN, EC2-internal-hostname, 12-digit account ID). There is no pattern for VPC IDs, subnet IDs, security-group IDs, instance IDs, or volume IDs. Real, raw VPC/SG identifiers were confirmed present in retained evidence (now scrubbed) despite `--redact-sensitive-identifiers` being passed. Whether every one of these identifiers is sensitive depends on sharing context, but the tool promises "sensitive-identifier redaction" broadly, and AWS infrastructure IDs can expose account topology and are cross-referenceable with other leaked data.

Should be fixed in the same PR as RED-TERMINAL-001 (same root module, same review), but the two are independently tested and independently confirmed — treat as two defects, not one.

---

## RBAC-DOC-001 — Documented minimal RBAC can never reach clean coverage

**Severity:** P2 / Medium
**Status:** fixed locally, real-cluster verified on Kind
**Evidence:** `01-reduced-rbac/report.md`

`deploy/clusterrole.yaml`, applied verbatim with no modification, produces a permanent `INCOMPLETE`/exit-3 report regardless of actual cluster health: `DRAIN-002` needs `persistentvolumes`/`persistentvolumeclaims` the doc never grants; `API-001`/`API-002`'s deprecated-API sweep needs `extensions/v1beta1 podsecuritypolicies` and `storage.k8s.io/v1beta1 csistoragecapacities`, neither granted. No finding is fabricated or hidden — this is a coverage/trust-signal problem, not a correctness problem — but it means the documented "copy-pasteable RBAC" is not actually sufficient for the coverage it implies, and CI gates keyed on exit code 3 would fire permanently for a compliant deployment.

This is a product-contract question, not merely a certification oddity. Three valid resolution directions, **not to be resolved by silently granting more permissions without reviewing scope**:
1. Expand the documented RBAC to genuinely support complete coverage.
2. Deliberately keep it reduced and explicitly document it as reduced-coverage.
3. Split into a recommended complete-read RBAC and a reduced-privilege RBAC with an explicit coverage matrix showing exactly what each variant can and cannot evaluate.

---

## RBAC-STALE-003 — Stale test tooling / doc comment (informational, lowest priority)

**Severity:** P3 / Low
**Status:** fixed locally, test/script verified
**Evidence:** `01-reduced-rbac/report.md`

`scripts/certification/assert-findings.sh`'s `insufficient_evidence_capable_rule_ids` allowlist (6 rules) and a comment in `internal/report/evaluation_coverage.go` referencing a removed symbol (`ruleErrorsMapKeys`) both understate current behavior — all 31 rules now support per-rule evidence-dependency tracking, confirmed both on Kind (K8s-plane rules) and real EKS (AWS-plane rules, Lane 2 IAM-A). The actual product behavior is correct and more complete than the stale references describe. Fix opportunistically alongside certification-document cleanup; do not let it distract from RED-TERMINAL-001.

---

## Findings explicitly flagged as needing further investigation, not fully explained

These are not classified as defects — they may be entirely correct, deliberate behavior — but the exact causal mechanism was not traced to source in this session and should not be treated as fully understood:

- **Lane 1 Profile B** (PDB access denied only): `API-001`, `API-002`, and `DRAIN-001` also went `insufficient_evidence`, not just `PDB-001`/`PDB-002`. A plausible explanation (the old `policy/v1beta1 PodDisruptionBudget` deprecated-API check shares the same underlying resource type, and DRAIN-001 may itself consult PDB evidence) is offered in `01-reduced-rbac/report.md`, but the exact dependency chain was not confirmed against `execution.go`'s `EvidenceDependencies()` for `DRAIN001`/`API001`/`API002` line-by-line the way it was for the PDB/webhook/CRD rules. Needs a source-level pass before being cited as fully understood.
- **Lane 1 Profiles C/D conditional-dependency behavior** (`WH-002`/`CRD-002` correctly stayed `evaluated` when Services/EndpointSlices were denied): confirmed correct *for the specific case tested* — an empty webhook/CRD inventory with no live object actually requiring that backend evidence. This does **not** certify the activated dependency path (a cluster with an actual Service-referencing webhook or a webhook-conversion CRD present, where `WH-002`/`CRD-002` *should* correctly go `insufficient_evidence` when Services/EndpointSlices are denied). That path was not exercised — no such live object existed on either the Kind or EKS clusters used in this certification.
