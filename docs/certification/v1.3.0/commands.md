# Command inventory — exact commands, mutation classification

Every flag name below is taken directly from `internal/cli/scan.go`,
`internal/cli/compare.go`, and `internal/cli/rollback.go` (read in full
for this plan) — none are guessed. `<...>` placeholders are filled in at
Stage 2/3, never invented here. `CLUSTER_NAME`, `REGION`,
`TARGET_VERSION`, and `KUBEPREFLIGHT_BIN` follow
`scripts/case-study/*.sh`'s existing environment-variable convention.

Legend: **READ-ONLY** = makes no mutating call against AWS or the
cluster. **MUTATING** = creates, changes, or deletes a real resource and
requires separate, explicit human approval per `approval-record.md`
before it is ever run (this document proposes the exact command; it does
not authorize running it).

## Phase 0 — cluster setup (MUTATING, one-time, shared by full-access and reduced-IAM modes)

Reuses `demo/eks-case-study/cluster.yaml` unchanged (one `t3.small`
managed node, no NAT gateway) unless Stage 2 approval specifies a
different cluster name — if so, copy `cluster.yaml` and change only
`metadata.name`/`metadata.region`, keep every sizing field identical.

```bash
# READ-ONLY — confirms account/region and that no stale cluster exists.
aws sts get-caller-identity
aws eks list-clusters --region "${REGION}"

# MUTATING — creates a real, billable EKS cluster + node group.
eksctl create cluster -f demo/eks-case-study/cluster.yaml   # or a v1.3.0-named copy
```

## Phase 1 — IAM setup for the two scanning identities (MUTATING)

```bash
# MUTATING — creates two IAM policies (see environment.md for the exact
# JSON) and, depending on the account's existing pattern, either two IAM
# users/roles or two profiles assumed via `aws sts assume-role`. Exact
# creation commands are deliberately left to Stage 2/3 execution (the
# account's existing IAM conventions decide user vs. role vs. IRSA), not
# prescribed here — only the policy documents themselves are fixed by
# this plan (environment.md).
aws iam create-policy --policy-name kubepreflight-cert-full-access --policy-document file://<full-access-policy.json>
aws iam create-policy --policy-name kubepreflight-cert-reduced-iam --policy-document file://<reduced-iam-policy.json>
# ... attach to whichever principal each local AWS_PROFILE resolves to.
```

Both identities also need the same Kubernetes-side read RBAC (not an
IAM action) mapped via the cluster's access entries / `aws-auth`
ConfigMap — same MUTATING classification, same "left to Stage 2/3"
scoping, since the exact mechanism (access entries vs. `aws-auth`)
depends on the cluster's authentication mode.

## Phase 2 — full-access mode evidence (READ-ONLY against the cluster/AWS)

All commands below use `AWS_PROFILE=<full-access-profile>`.

`--context` is passed explicitly on every `scan --provider eks` command below
(never omitted) so the command cannot silently fall back to whatever
kubeconfig context happens to be current in the shell. This matters
concretely, not just in principle: this repo's own Stage 1 validation
(`validation-summary.md`'s "Safety incident" note) already found a real,
pre-existing kubeconfig at `~/.kube/config` in this environment that this
task never set up — an implicit-context command here would have targeted
whatever that stray kubeconfig's current context pointed at, not
necessarily this certification's intended cluster. `AWS_REGION` is
likewise exported explicitly rather than left to an ambient/shared AWS
config default, for the same reason (`internal/collectors/aws/collector.go`
`LoadCollector` resolves region from the standard SDK chain, which has no
CLI `--region` flag to pin it — the environment variable is the only
explicit control this codebase offers).

```bash
# READ-ONLY
export AWS_PROFILE=<full-access-profile>
export AWS_REGION="${REGION:-<PLACEHOLDER-REGION>}"
export CLUSTER_NAME="${CLUSTER_NAME:-<PLACEHOLDER-CLUSTER-NAME>}"
export TARGET_VERSION="${TARGET_VERSION:-<PLACEHOLDER-TARGET-VERSION>}"
export EKS_CONTEXT="${EKS_CONTEXT:-<PLACEHOLDER-KUBECONFIG-CONTEXT>}"

kubepreflight scan \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --context "${EKS_CONTEXT}" \
  --target-version "${TARGET_VERSION}" \
  --output all \
  --findings-out docs/certification/v1.3.0/full-access/findings.json \
  --output-dir docs/certification/v1.3.0/full-access \
  --serve-report never \
  --redact-sensitive-identifiers

# READ-ONLY — rollback assess against the same cluster, using the just-
# captured findings.json as operational-readiness evidence.
kubepreflight rollback assess \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --findings docs/certification/v1.3.0/full-access/findings.json \
  --output all \
  --output-dir docs/certification/v1.3.0/full-access \
  --assessment-out docs/certification/v1.3.0/full-access/rollback-assessment.json \
  --redact-sensitive-identifiers
```

## Phase 3 — reduced-IAM mode evidence (READ-ONLY against the cluster/AWS)

Same cluster, same target version, different profile:

```bash
# READ-ONLY
export AWS_PROFILE=<reduced-iam-profile>
# AWS_REGION and EKS_CONTEXT are already exported from Phase 2 above (same
# cluster, same kubeconfig context) — only the profile changes here. If
# running this phase in a fresh shell, re-export both explicitly rather than
# relying on ambient defaults, per Phase 2's note above.

kubepreflight scan \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --context "${EKS_CONTEXT}" \
  --target-version "${TARGET_VERSION}" \
  --output all \
  --findings-out docs/certification/v1.3.0/reduced-iam/findings.json \
  --output-dir docs/certification/v1.3.0/reduced-iam \
  --serve-report never \
  --redact-sensitive-identifiers

# READ-ONLY
kubepreflight rollback assess \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --findings docs/certification/v1.3.0/reduced-iam/findings.json \
  --output all \
  --output-dir docs/certification/v1.3.0/reduced-iam \
  --assessment-out docs/certification/v1.3.0/reduced-iam/rollback-assessment.json \
  --redact-sensitive-identifiers
```

## Phase 4 — manifests-only mode evidence (READ-ONLY, zero AWS/cluster access)

No `AWS_PROFILE`, no kubeconfig — `--manifests-only` refuses to be
combined with `--provider`/`--kubeconfig`/`--context` at the flag-parsing
stage (`internal/cli/scan.go`'s validation block), so this cannot
accidentally touch AWS or the cluster even if credentials happen to be
present in the shell.

```bash
# READ-ONLY — local filesystem only.
kubepreflight scan \
  --manifests-only \
  --manifests demo/eks/manifests/old-api.yaml \
  --target-version "${TARGET_VERSION}" \
  --output all \
  --findings-out docs/certification/v1.3.0/manifests-only/findings.json \
  --output-dir docs/certification/v1.3.0/manifests-only \
  --serve-report never \
  --redact-sensitive-identifiers
```

## Phase 5 — comparisons (READ-ONLY, local files only, no AWS/cluster access)

The central `not_re_evaluated` proof: full-access baseline vs. a
reduced-scope current report.

```bash
# READ-ONLY
kubepreflight compare \
  --baseline docs/certification/v1.3.0/full-access/findings.json \
  --current docs/certification/v1.3.0/reduced-iam/findings.json \
  --json-out docs/certification/v1.3.0/comparisons/full-vs-reduced-iam.json \
  --markdown-out docs/certification/v1.3.0/comparisons/full-vs-reduced-iam.md \
  --gate-out docs/certification/v1.3.0/comparisons/full-vs-reduced-iam-gate.json \
  --redact-sensitive-identifiers

# READ-ONLY
kubepreflight compare \
  --baseline docs/certification/v1.3.0/full-access/findings.json \
  --current docs/certification/v1.3.0/manifests-only/findings.json \
  --json-out docs/certification/v1.3.0/comparisons/full-vs-manifests-only.json \
  --markdown-out docs/certification/v1.3.0/comparisons/full-vs-manifests-only.md \
  --gate-out docs/certification/v1.3.0/comparisons/full-vs-manifests-only-gate.json \
  --redact-sensitive-identifiers

# READ-ONLY — legacy schema 1.0 backward-compatibility proof (PR 7). Reuses
# the already-committed, already-public schema-1.0 fixture
# demo/eks-case-study/evidence/after-upgrade/findings.json (confirmed by
# direct inspection: schemaVersion "1.0", no ruleExecutions field) rather
# than fabricating a new one — see expected-results.md's legacy-
# compatibility section for why this specific file qualifies.
kubepreflight compare \
  --baseline demo/eks-case-study/evidence/after-upgrade/findings.json \
  --current docs/certification/v1.3.0/full-access/findings.json \
  --json-out docs/certification/v1.3.0/comparisons/legacy-1.0-vs-full-access.json \
  --markdown-out docs/certification/v1.3.0/comparisons/legacy-1.0-vs-full-access.md \
  --redact-sensitive-identifiers

# READ-ONLY — PR 7's other fix: rollback assess must also load a genuine
# schema 1.0 document via --findings. Run in whichever mode's IAM identity
# is convenient (rollback assess still needs live AWS/cluster access for
# its own collector, independent of --findings' schema version).
kubepreflight rollback assess \
  --provider eks \
  --cluster-name "${CLUSTER_NAME}" \
  --findings demo/eks-case-study/evidence/after-upgrade/findings.json \
  --output json \
  --output-dir docs/certification/v1.3.0/comparisons \
  --assessment-out docs/certification/v1.3.0/comparisons/legacy-1.0-rollback-assess.json \
  --redact-sensitive-identifiers
```

## Phase 6 — sanitization check (READ-ONLY, local files only)

```bash
# READ-ONLY
scripts/certification/check-evidence-sanitized.sh docs/certification/v1.3.0/full-access
scripts/certification/check-evidence-sanitized.sh docs/certification/v1.3.0/reduced-iam
scripts/certification/check-evidence-sanitized.sh docs/certification/v1.3.0/manifests-only
scripts/certification/check-evidence-sanitized.sh docs/certification/v1.3.0/comparisons
```

## Phase 7 — cleanup and independent verification (MUTATING, then READ-ONLY verification)

Mirrors `demo/eks-case-study/cleanup.sh` and its README's Step 10, made
explicit as an independently-verified checklist rather than trusting
`eksctl delete cluster`'s exit code alone:

```bash
# MUTATING — deletes the two IAM policies/roles created in Phase 1 (exact
# commands depend on the user/role/IRSA mechanism chosen at Stage 2/3;
# not prescribed further here).
aws iam delete-policy --policy-arn <full-access-policy-arn>
aws iam delete-policy --policy-arn <reduced-iam-policy-arn>
# ... plus detaching/deleting whichever principal each was attached to.

# MUTATING — deletes the EKS cluster and its node group (eksctl waits for
# completion; several minutes).
eksctl delete cluster --name "${CLUSTER_NAME}" --region "${REGION}" --wait
```

### Independent verification checklist (every item, not just the delete command's exit code)

- [ ] `aws eks list-clusters --region "${REGION}"` (READ-ONLY) returns an
      empty `clusters` list — matches `demo/eks-case-study/README.md`'s
      own Step 10.
- [ ] `aws ec2 describe-instances --filters "Name=tag:eks:cluster-name,Values=${CLUSTER_NAME}" --region "${REGION}"` (READ-ONLY)
      returns no running/pending instances — catches a node group that
      failed to drain cleanly.
- [ ] `aws cloudformation list-stacks --region "${REGION}" --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE` (READ-ONLY)
      shows no remaining `eksctl-${CLUSTER_NAME}-*` stacks — `eksctl`
      manages the cluster/nodegroup/VPC via CloudFormation; a stuck stack
      here is the most reliable signal of an incomplete teardown.
- [ ] `aws iam list-policies --scope Local --query "Policies[?contains(PolicyName, 'kubepreflight-cert')]"` (READ-ONLY)
      returns empty — confirms both certification-specific IAM policies
      were actually deleted, not just detached.
- [ ] No EBS volumes remain tagged for this cluster:
      `aws ec2 describe-volumes --filters "Name=tag:eks:cluster-name,Values=${CLUSTER_NAME}" --region "${REGION}"` (READ-ONLY)
      returns empty.
- [ ] Local `kubeconfig` context for the deleted cluster is removed
      (`kubectl config get-contexts`, `kubectl config delete-context
      <ctx>` if still present) — cosmetic, but prevents an accidental
      future command from targeting a name that no longer resolves to
      anything real.

Every verification step above is READ-ONLY; none creates or deletes
anything. If any check fails, investigate before walking away — a
lingering resource keeps billing, matching
`demo/eks-case-study/README.md`'s own Step 10 warning.
