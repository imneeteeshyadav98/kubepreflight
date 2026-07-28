# KubePreflight v1.3.0 — real-EKS certification plan (PR 8)

Status: **Stage 1 (planning only) — no AWS access used to produce this
document.** This directory is the certification plan for PR 8 of the
v1.3.0 sequence, the final PR in the merged 7-PR run (#221–#227, landed
on `master` at `0442ad4`) that added native per-rule execution records,
legacy normalization, Terminal/Markdown/HTML/Console coverage rendering,
the `not_re_evaluated` comparison bucket, gate/score coverage
presentation, and the findings schema `1.0` → `1.1` bump with a hardened
`internal/v1compat` compatibility contract.

PR 8 does not add product code. It certifies that merged behavior against
a real, disposable EKS cluster across three evidence modes — full access,
reduced IAM access, and manifests-only — the same way the (unversioned,
committed) `docs/case-studies/eks-1.31-to-1.32.md` case study already
certified the pre-v1.3.0 product against a real cluster. This plan reuses
that case study's cluster shape, scripts, and safety conventions rather
than inventing new ones — see "Prior art" below.

## Prior art this plan reuses

- [`docs/case-studies/eks-1.31-to-1.32.md`](../../case-studies/eks-1.31-to-1.32.md)
  — the one real, executed, disposable-cluster certification already in
  this repo (executed 2026-07-16, cluster `kubepreflight-case-study`,
  `us-east-1`, deleted after evidence capture). No repo-internal document
  specifically named "v1.2.0 certification" exists; `docs/releases/
  v1.2.1-evidence.md` references a `v1.2.0_release_lock_validation.md`
  and a certification `final.md`, but both live in an external scratchpad
  directory, not in this repository, so they are not available to cite
  here. This case study is the closest and most concrete in-repo
  equivalent, and its cost/safety footprint is treated as the baseline
  for this plan's spend estimate below.
- [`demo/eks-case-study/`](../../../demo/eks-case-study/) — the cluster
  definition (`cluster.yaml`), fixtures (`manifests/`), teardown script
  (`cleanup.sh`), and already-committed evidence tree from that case
  study. Its evidence layout (`evidence/<phase>/findings.json,report.md,
  report.html`, `evidence/compare/<label>/`, `evidence/rollback/`) is the
  direct model for this PR's own evidence tree under `full-access/`,
  `reduced-iam/`, `manifests-only/`, and `comparisons/`.
- [`scripts/case-study/*.sh`](../../../scripts/case-study/) — six scripts
  (`01-seed-fixtures.sh` … `06-compare.sh`) that already encode the safe
  patterns this plan reuses: `CLUSTER_NAME`/`REGION`/`KUBEPREFLIGHT_BIN`
  overridable via environment variable, `--ignore-not-found`/idempotent
  deletes, one dedicated fixture directory per phase, explicit `--manifests`
  pointed at a single file (never a whole directory that would double-count
  live-applied fixtures — see `02-scan.sh`'s own comment on this).
- `internal/redact/` — the existing `--redact-sensitive-identifiers` flag
  and its ARN/hostname/account-ID regexes, audited below for exactly what
  it does and does not cover.

## Objectives

Certify, across all three modes, that the following merged v1.3.0
behavior holds against real collected evidence, not only synthetic test
fixtures:

1. `schemaVersion == "1.1"` on every native scan output.
2. Exactly 31 `RuleExecutionRecord` entries in `ruleExecutions` on every
   scan output, in every mode — confirmed from `rules.AllRuleIDs()`
   (`internal/rules/defaults.go`), which registers 31 rules:
   `API-001, API-002, WH-001, WH-002, WH-004, WH-005, DRAIN-001..005,
   PDB-001, PDB-002, NODE-001, NODE-002, NODE-003, NET-002, WORKLOAD-001,
   ADDON-001, ADDON-002, EKS-NG-001..004, EKS-INSIGHT-001..003, COREDNS-001,
   CRD-001, CRD-002, APISERVICE-001`.
3. Accurate `applicability`/`state` values per PR 1's contract
   (`internal/rules/rule.go`'s `RunAllWithExecutions`).
4. Native vs. normalized-legacy metadata distinction
   (`RuleExecutionsNormalized`), per PR 2.
5. Evaluation-coverage presentation across Terminal, Markdown, HTML, and
   Console (PR 3/PR 4).
6. Gate/score qualification text and additive-only coverage presentation
   in `compare --gate-out` (PR 6).
7. `not_re_evaluated` comparison semantics (PR 5), proven with a real
   full-access baseline vs. a real reduced-access-or-manifests-only
   current report — not only `internal/comparison`'s unit tests.
8. Output parity across Terminal/JSON/Markdown/HTML/Console for the same
   underlying report.
9. Redaction/evidence-sanitization safety for anything committed as
   public evidence.
10. Backward compatibility with a genuine schema `1.0` document (PR 7's
    fix): both `compare` and `rollback assess --findings` must load it.

See [`expected-results.md`](expected-results.md) for the exact,
field-level assertions this maps to per mode, and
[`commands.md`](commands.md) for every command that produces the
evidence those assertions check.

## The three evidence modes

| Mode | What's exercised | AWS/cluster access needed |
|---|---|---|
| **Full access** | Every plane (Kubernetes + AWS + manifests), full IAM | Full read-only IAM policy (below) + cluster read RBAC |
| **Reduced IAM** | Same cluster, an IAM identity missing several `eks:`/`ec2:` read actions the AWS collector calls | Reduced read-only IAM policy (below) + same cluster RBAC |
| **Manifests-only** | `--manifests-only`, zero cluster/AWS access at all | None — this mode never loads a kubeconfig or AWS credentials (`internal/cli/scan.go`'s `manifestsOnly` guard) |

Only **one** EKS cluster needs to exist for this whole certification:
full-access and reduced-IAM both scan the same live cluster, using two
different local AWS credential profiles/roles. Manifests-only needs no
cluster or AWS access at all and can be exercised (and was, for this
plan's own tooling validation — see `validation-summary.md`) with zero
AWS setup, against this repo's own `demo/eks/manifests/old-api.yaml`.

## Environment prerequisites, IAM policies, and sanitization plan

See [`environment.md`](environment.md) for:

- AWS account/region/cluster prerequisites.
- The exact full-access and reduced-IAM read-only IAM policies, derived
  from `internal/collectors/aws/collector.go` and
  `internal/rollback/eks/collector.go`'s actual AWS SDK calls (not
  guessed).
- The sanitization plan: what `--redact-sensitive-identifiers` covers
  (`internal/redact/redact.go`'s regexes), what it does **not** cover,
  and the additional automated check
  (`scripts/certification/check-evidence-sanitized.sh`) that closes that
  gap for committed evidence.

## Command inventory and mutation classification

See [`commands.md`](commands.md) for the full command list — every
`kubepreflight` invocation across all three modes, and every
`aws`/`eksctl`/`kubectl` command needed to stand up, seed, and tear down
the cluster — each one tagged `READ-ONLY` or `MUTATING`.

## Estimated AWS spend

Basis: `demo/eks-case-study/cluster.yaml`'s already-executed, minimal
footprint — one EKS control plane, one `t3.small` managed node, no NAT
gateway, no load balancer, no EIP — is the direct model for this PR's
cluster, reused unchanged rather than resized. No in-repo document
records that case study's actual dollar cost (see "Prior art" above), so
this estimate is derived conservatively from list pricing for that exact
shape, not a real historical figure:

| Item | Rate (indicative, `us-east-1`) | Duration | Estimated cost |
|---|---|---|---|
| EKS control plane (standard support) | ~$0.10/hour | ~2–3 hours (create + three-mode scan/compare/rollback pass + teardown, generously bounded) | ~$0.20–$0.30 |
| 1x `t3.small` on-demand node | ~$0.02/hour | ~2–3 hours | ~$0.04–$0.06 |
| 1x 20 GiB gp3 EBS volume | ~$0.08/GiB-month, prorated to a few hours | — | ~$0.01 |
| NAT gateway / load balancer / EIP | $0 — explicitly disabled (`cluster.yaml`'s `vpc.nat.gateway: Disable`, no Service/Ingress created) | — | $0 |
| AWS API calls this PR makes (`Describe*`/`List*` only) | Free — read-only EC2/EKS API calls carry no charge | — | $0 |
| Manifests-only mode | No AWS resources touched at all | — | $0 |

**Total estimate: under $1**, matching v1.2.0-era case study's own
"cheap, disposable cluster" design intent
(`demo/eks-case-study/cluster.yaml`'s own header comment). The dominant
cost driver is wall-clock cluster lifetime, not resource count — keeping
cluster creation → all three modes' evidence capture → teardown to a
single, unhurried session (the case study document reports cluster
create/upgrade/delete each take single-digit minutes) is what keeps this
near the low end of the range.

## Evidence directory structure

```text
docs/certification/v1.3.0/
├── README.md               (this file)
├── approval-record.md       (Stage 2 human approval checklist, unchecked)
├── environment.md           (prerequisites, IAM policies, sanitization plan)
├── commands.md               (exact command inventory, mutation-tagged)
├── expected-results.md       (assertion plan, per mode)
├── validation-summary.md     (Stage 1 self-validation: go test, script tests)
├── full-access/               (empty until Stage C/D execution)
├── reduced-iam/                (empty until Stage C/D execution)
├── manifests-only/              (empty until Stage C/D execution)
└── comparisons/                  (empty until Stage C/D execution)
```

The three mode subdirectories and `comparisons/` are placeholders only
(each holds a `.gitkeep`) — real evidence files (`findings.json`,
`report.md`, `report.html`, `comparison.json`, `gate.json`,
`rollback-assessment.json`) are only written during actual Stage C/D
execution against a real cluster, which has not been approved (see
`approval-record.md`).

## Assertion plan

See [`expected-results.md`](expected-results.md) for the exact,
field-level assertions per mode, plus the tested assertion tooling in
`scripts/certification/`.

## Cleanup checklist

See `commands.md`'s "Cleanup and independent verification" section —
mirrors `demo/eks-case-study/cleanup.sh` and its README's Step 10
("verify no clusters remain"), extended with an explicit
independently-verified checklist rather than trusting a single command's
exit code.

## Approval checklist

See [`approval-record.md`](approval-record.md) — unchecked, with
placeholders only, ready for a human to fill in during Stage 2.

## Stage boundaries

- **Stage 1: no-spend planning.** No AWS/eksctl/kubectl command against
  real infrastructure. No AWS account, region, or cluster name invented —
  every value in this directory that would identify a real
  account/region/cluster is an explicit placeholder.
- **Stage 2: independent technical review.** A second, independent pass
  verified every Stage 1 claim against actual source, found and fixed
  several real gaps (sanitization coverage, a vacuous-pass risk in the
  comparison proof, an ambiguous kubeconfig context, a missing
  evidence-staging guard), and issued a recommendation of
  `READY_FOR_HUMAN_APPROVAL` — still no AWS/cluster access.
- **Human environment approval (current step):** a human (or the
  authorized account owner) fills in `approval-record.md`'s identity
  fields (account, region, cluster, profile, approver, dates) and
  literally signs the approval statement themselves — no automated agent
  fills in an approver name, date, or "approved" decision. See
  `approval-record.md`'s "Recommended environment & policy decisions" and
  "Sign-off" sections.
- **Stage 3 (locked until the above is complete): real EKS execution.**
  Execute the read-only `kubepreflight` commands in `commands.md` against
  the approved cluster in all three modes, gated by the pre-execution
  checklist in `approval-record.md`.
- **Stage 4 (locked): evidence sanitization and certification lock.**
  Capture evidence into the mode subdirectories, run the sanitization
  check, run the assertion scripts, commit sanitized evidence, tear down
  anything this certification itself created.

Current status:

```text
✅ Stage 1 — No-spend planning
✅ Stage 2 — Independent technical review
✅ Human environment approval
✅ Stage 3 — Real EKS execution
✅ Stage 4 — Evidence sanitization and certification lock
```

## Certification result

**PASS.** Full detail: `validation-summary.md`'s "Stage 3/4 addendum",
per-checkpoint `assertions.txt` files, `checksums.sha256`,
`cleanup-verification.md`, and the full authorization/execution trail in
`approval-record.md`.

```text
Full-access certification:         PASS
Reduced-IAM certification:         initial run found a real,
                                    release-blocking issue (see
                                    reduced-iam/pre-fix-summary.md) —
                                    corrective PR #228 merged (a91deca),
                                    rerun: PASS
Manifests-only certification:      PASS
Comparison (not_re_evaluated) proof: PASS
Live infrastructure cleanup:       PASS (10/10 independently verified)
Evidence sanitization:             PASS (0 findings, all categories)

Overall KubePreflight v1.3.0 real-EKS certification: PASS
```
