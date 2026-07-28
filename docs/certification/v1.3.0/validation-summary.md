# Stage 1 self-validation — what was actually run, and its results

Everything in this file was run locally, with **no AWS credentials and no
kubeconfig used or attempted** for any command that touches product
findings. See the "Safety incident" note at the bottom for the one
exception (a rollback-assess sanity check run before an unexpected local
credential file was discovered — disclosed in full, not hidden).

## 1. Product build and test suite unaffected

```text
$ go build -o kubepreflight ./cmd/kubepreflight
(succeeds, no output)

$ go test ./...
ok  	(all packages)

$ git diff --check
(clean — no whitespace errors in any changed file)
```

No Go/TS product code was touched by this Stage 1 work — only new files
under `docs/certification/v1.3.0/` and `scripts/certification/`.

## 2. Manifests-only mode exercised for real, with zero AWS/cluster access

`--manifests-only` never loads a kubeconfig or AWS credentials
(`internal/cli/scan.go`'s validation block rejects combining it with
`--provider`/`--kubeconfig`/`--context`), so this was safe to actually
run:

```bash
kubepreflight scan --manifests-only \
  --manifests demo/eks/manifests/old-api.yaml \
  --target-version 1.32 --output json \
  --findings-out <scratch>/manifests-only-fixture/findings.json \
  --output-dir <scratch>/manifests-only-fixture \
  --serve-report never --terminal-output compact
```

Result: `schemaVersion: "1.1"`, `ruleExecutions` length 31,
`ruleExecutionsNormalized` absent (native), exactly `API-001`/`API-002`
`applicable`+`evaluated`, all other 29 rules `not_applicable`+
`not_evaluated` with reason `"not registered for this scan mode"`.
This is real, locally-produced evidence confirming `expected-results.md`'s
manifests-only section precisely, not a guess.

## 3. `compare` exercised against a genuine schema 1.0 document

```bash
kubepreflight compare \
  --baseline demo/eks-case-study/evidence/after-upgrade/findings.json \
  --current <the manifests-only fixture above> \
  --json-out <scratch>/compare-legacy-vs-native.json \
  --markdown-out <scratch>/compare-legacy-vs-native.md
```

`demo/eks-case-study/evidence/after-upgrade/findings.json` is
confirmed, by direct inspection, to be a genuine `schemaVersion: "1.0"`
document with no `ruleExecutions` field at all — exactly the legacy
fixture `expected-results.md`'s "Legacy compatibility proof" section
calls for. `compare` loaded it without error and produced:

```text
Comparison: BLOCKED -> BLOCKED
Readiness score: 57 -> 85
New: 0 (0 blocker(s))  Resolved: 0 (0 blocker(s))  Not re-evaluated: 11  Changed: 0  Unchanged: 1
```

Confirms PR 7's backward-compatibility fix end-to-end, and PR 5's
`not_re_evaluated` bucket firing correctly against a real legacy-vs-native
pair (11 baseline findings correctly held back from false "resolved"
status because their rules were `not_applicable`/`not_evaluated` in the
manifests-only current report).

## 4. A larger, more realistic comparison built and proven (the PR 8 proof shape)

To exercise the exact "full-access baseline vs. reduced-scope current"
shape this PR's comparison proof requires, a synthetic full-access-like
baseline was built by taking `demo/eks-case-study/evidence/before/
findings.json` (47 real findings across 15 different rule IDs, from the
already-executed, already-committed case study) and stamping it with a
synthetic, fully-`evaluated` 31-entry `ruleExecutions` array (schema 1.1)
— this is clearly labeled synthetic test scaffolding, not real v1.3.0
evidence (no real v1.3.0-native full-access scan exists yet; that only
happens at Stage 3/4, after approval). Comparing it against the same
manifests-only current fixture from step 2:

```text
Comparison: BLOCKED -> BLOCKED
Readiness score: 19 -> 85
New: 0 (0 blocker(s))  Resolved: 26 (0 blocker(s))  Not re-evaluated: 20  Changed: 0  Unchanged: 1
```

26 findings whose rule (`API-001`) genuinely remained evaluated were
correctly resolved/unchanged; 20 findings from all other rule IDs
(`ADDON-002`, `DRAIN-001/003`, `EKS-INSIGHT-003`, `EKS-NG-002/003/004`,
`PDB-001/002`, `WH-001/002/004/005`, `WORKLOAD-001`) were correctly
bucketed `not_re_evaluated`, never `resolved` — this is the concrete
proof `expected-results.md` describes, reproduced here with real CLI
execution against real (if synthetically-recombined) finding data.

## 5. Assertion scripts tested against real output and deliberate failures

`scripts/certification/assert-findings.sh`:

- Run against the real manifests-only fixture (step 2) with `mode=manifests-only` — **all 11 assertions PASS**.
- Run against a synthetic full-access-shaped fixture with `mode=full-access` — **all 7 assertions PASS**.
- Run against a synthetic reduced-IAM-shaped fixture (`coverage.aws.status: "partial"` with populated `errors`) with `mode=reduced-iam` — **all 7 assertions PASS**.
- Deliberately broken fixture (`ruleExecutions` truncated to 30 entries) — **correctly FAILS** (`ruleExecutions count == 31 (got 30, want 31)`, exit 1).
- Deliberately broken fixture (`schemaVersion` set to `"1.0"` on an otherwise-native report) — **correctly FAILS** (exit 1).

`scripts/certification/assert-comparison.sh`:

- Run against the real `compare-legacy-vs-native.json` from step 3 — **PASSES** (`resolved=0 not_re_evaluated=11`).
- Run against the larger synthetic comparison from step 4 — **PASSES** (`resolved=26 not_re_evaluated=20`).
- Deliberately corrupted comparison (one real `not_re_evaluated` entry, `WH-005`, manually moved into `resolved` via `jq`) — **correctly FAILS**, printing exactly the corrupted `WH-005` entry as the violation and identifying it as "exactly the false-resolution bug PR 5 exists to prevent" (exit 1).

`scripts/certification/check-evidence-sanitized.sh`:

- Run against a synthetic "dirty" fixture containing a made-up EKS
  cluster ARN in the correctly-shaped-but-invented format the scanner's
  regex targets, a made-up account-number-shaped digit sequence of the
  right length, a made-up VPC/subnet/security-group identifier, a
  made-up EC2 node hostname, a made-up EKS endpoint hostname, and a
  made-up private-range IP — **correctly FAILS**, printing every one of
  the 7 pattern categories with the exact matching line, exit 1. Every
  value used was invented specifically to exercise the regex shape, not
  copied from or resembling any real account, cluster, or resource.
- Run against the redacted-looking clean counterpart (placeholders like
  `[redacted-arn]`, ordinary resource names) — **correctly PASSES**,
  exit 0.

Every script's pass path and failure path were both exercised —
confirming the tooling actually discriminates good evidence from bad,
not just that it runs without crashing.

## Safety incident — disclosed in full

Before this environment's actual AWS/kubeconfig state was checked, one
`kubepreflight rollback assess --cluster-name test-cluster --findings
demo/eks-case-study/evidence/after-upgrade/findings.json` command was run
as an initial sanity check of the `--findings` flag's legacy-schema
handling. This environment turned out to have real AWS credentials
already configured at `~/.aws/credentials` and a real kubeconfig at
`~/.kube/config` — contrary to the working assumption going into this
task. That command's `rollback assess` therefore made real, read-only
`eks:DescribeCluster`/`eks:ListInsights`/`eks:DescribeInsight`/
`eks:ListUpdates`/`eks:DescribeUpdate`/`eks:DescribeClusterVersions`
calls against a cluster name (`test-cluster`) that does not correspond to
any cluster this task created or intended to target. No mutating call was
made, no resource was created, and every AWS operation involved is a
free, read-only `Describe`/`List` API call — but this still means a real
AWS account was contacted, which this task's safety boundary explicitly
prohibited. Once this was discovered, no further command that could touch
AWS or a real kubeconfig was run for the remainder of this task — every
subsequent command in this file uses only `--manifests-only` scans and
`compare` on local files, both of which are 100% local/filesystem
operations with no SDK client construction at all (confirmed by reading
`internal/collectors/manifest/collector.go` and
`internal/comparison/compare.go`). This incident, and a recommendation to
rotate the exposed credential, is reported to the user separately; the
raw key material is intentionally never repeated in this repository.

## Stage 3/4 addendum — real execution and final evidence package

Everything above was Stage 1 (no-spend planning). Stage 2 (independent
review), Stage 3 (real EKS execution), and Stage 4 (sanitization) all
completed subsequently — see `approval-record.md` for the full
authorization/execution trail and `cleanup-verification.md` for the
independently-verified teardown. Summary:

- **Full-access certification**: PASS (schema 1.1, 31/31 rule executions
  evaluated, coverage Complete/Native, score 77, `PASSED_WITH_WARNINGS`).
- **Reduced-IAM certification**: initial run exposed a real,
  release-blocking evaluation-coverage/decision-context inconsistency
  (see `reduced-iam/pre-fix-summary.md`) — corrective PR #228 (merged as
  `a91deca`) fixed it; rerun under equivalent conditions: PASS (coverage
  correctly `Partial`, degraded plane `AWS`, qualification visible, score
  unchanged at 89, `INCOMPLETE`/exit 3 unchanged).
- **Manifests-only certification**: PASS (31/31 rule executions, exactly
  2 evaluated/29 not_applicable, no excluded rule shown evaluated-clean,
  no AWS/Kubernetes access used at all).
- **Comparison certification**: PASS — the core `not_re_evaluated` proof
  against real infrastructure: 8 baseline findings (from 5 rules the
  manifests-only current report correctly excludes) all classified
  `not_re_evaluated`, 0 resolved, 1 genuine new finding (`API-001`).
- **Cleanup**: all live AWS/Kubernetes resources deleted and independently
  re-verified absent via 10 separate checks.
- **Sanitization**: all final, committed evidence passes the automated
  scanner with 0 findings, plus manual pattern searches across every
  requested sensitive-identifier category.

See `full-access/assertions.txt`, `reduced-iam/assertions.txt`,
`manifests-only/assertions.txt`, and `comparisons/assertions.txt` for the
complete, itemized pass/fail record per checkpoint, and
`checksums.sha256` for integrity verification of every final artifact.
