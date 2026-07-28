# Approval record — v1.3.0 real-EKS certification (PR 8)

This checklist gates Stage 2 → Stage 3 (real AWS command execution). No
box here may be checked by an automated agent — every item requires an
explicit human decision. Stage 1 (this planning PR) leaves every box
unchecked and every value below as a placeholder.

## Checklist

```
[x] AWS account approved
[x] Region approved
[x] EKS cluster approved
[x] Estimated spend accepted
[x] No production mutation confirmed
[x] Read-only commands reviewed
[x] Evidence sanitization plan reviewed
[x] Cleanup plan reviewed
[x] Human approval recorded
```

## Values (transcribed verbatim from the account owner's explicit Stage 3 authorization message — see "Sign-off" below for the full text)

| Field | Value |
|---|---|
| AWS account ID or approved account alias | Account resolved via `aws sts get-caller-identity` at 2026-07-27 14:3x IST: IAM user `Neetesh`, account ending in `...4067` (full ID intentionally not recorded in this file per repo convention — see raw `aws sts get-caller-identity` output shown to the approver in chat before this record was written) |
| Region | `ap-south-1` |
| Existing EKS cluster (reuse) | Not used — this authorization is for fresh cluster creation, overriding Stage 2's earlier "prefer reuse" recommendation; account owner explicitly judged the reuse precondition (a qualifying existing sandbox) not met and authorized fresh creation instead, accepting the associated cost/mutation tradeoff |
| EKS cluster name | `kubepreflight-v130-cert` (confirmed via read-only `aws eks list-clusters --region ap-south-1`: no existing cluster with this name in this account/region) |
| Cluster purpose | Temporary, disposable, non-production certification environment for KubePreflight v1.3.0 (PR 8, Stage 3) — no other purpose |
| Production or non-production | Non-production |
| Target Kubernetes version pair | N/A — this is a certification scan target, not an upgrade case study; single version `1.36` (see `cluster.yaml`). Selected because: this CLI's `aws eks` command set does not support `describe-cluster-versions` (older CLI, 2.22.6), so standard-vs-extended-support status could not be queried directly; cross-checked instead via `aws eks describe-addon-versions --addon-name vpc-cni --region ap-south-1`, which reported compatible cluster versions `1.30`–`1.36` — `1.36` is the newest version EKS currently allows creating in this region, which is the strongest read-only signal available that it is standard support (a version cannot be in extended support before it has completed a standard-support window, and the newest creatable version is by definition still within that window). **Flagged for approver's awareness, not blocking**: this determination is inferred, not directly confirmed by an authoritative `describe-cluster-versions` call. |
| AWS profile or assumed-role mechanism | Default AWS CLI credentials already configured in this environment (no named `--profile` set) — the same identity confirmed above via `aws sts get-caller-identity` |
| Permitted Kubernetes context | Not yet known — `eksctl create cluster` will write a kubeconfig context named `<cluster-name>.<region>.eksctl.io` (eksctl's standard naming) once the cluster is created; the exact value will be confirmed and recorded here, and passed explicitly via every subsequent `--context` flag, per the account owner's explicit "do not rely on the active kubeconfig context" requirement |
| Expected maximum incremental spend | `$3` (account owner's explicit, corrected ceiling — supersedes Stage 2's earlier `$1` estimate, which the account owner identified as unrealistic for a fresh-cluster-creation scenario) |
| Approved command list/version | This commit's `commands.md`, plus this explicit fresh-cluster authorization recorded here and in the account owner's verbatim message below |
| Read-only confirmation | Cluster/node-group creation itself is the one explicitly authorized mutation (see "Sign-off" below); every `kubepreflight scan`/`compare`/`rollback assess` command against the created cluster remains read-only, per `commands.md`'s existing classification |
| Permitted temporary IAM mechanism | Default AWS CLI credentials/identity already configured (no new IAM user/role created for cluster creation itself); the full-access/reduced-IAM read-only policies from `commands.md` Phase 1 are created and deleted within this same session, scoped and temporary |
| Full-access IAM policy/role identifier | Not yet created — to be created during Stage 3 per `commands.md` Phase 1, after cluster creation |
| Reduced-IAM policy/role identifier | Not yet created — to be created during Stage 3 per `commands.md` Phase 1, after cluster creation |
| Approved evidence storage location | `docs/certification/v1.3.0/{full-access,reduced-iam,manifests-only,comparisons}/` (unchanged default, gitignored except `.gitkeep` per this commit's staging guard) |
| Cleanup requirements | Mandatory even if certification fails: delete the managed node group, delete the EKS cluster, verify the cluster no longer exists (`aws eks list-clusters`), verify no certification-tagged resources remain, remove the temporary kubeconfig context and any temporary credentials, delete local raw evidence after sanitized evidence is verified, report actual runtime and estimated spend. See "Exact cleanup command" below and `commands.md` Phase 7 for the independently-verified checklist this extends. |
| Planned execution window | This certification session |
| Approver name | Neetesh Yadav |
| Approver authority/role | AWS account owner or authorized operator |
| Approval decision | **APPROVED** — fresh temporary EKS cluster creation and use, strictly per the restrictions in "Sign-off" below |
| Approval date and time with timezone | 2026-07-27 14:29:47 IST (+0530) |
| Approval expiration | 6 hours after cluster creation, or end of this certification session, whichever occurs first. **Exact clock-time expiration will be computed and recorded here once cluster creation actually completes** — it cannot be computed before creation happens. |
| Additional restrictions | Node group name `certification-ng`; capacity type On-Demand; root volume smallest practical supported EBS size (20 GiB, matching `demo/eks-case-study/cluster.yaml`'s already-used minimum); no public workload exposure; no NAT Gateway; no Load Balancer; no Elastic IP; no application workload deployment unless a repository-owned, non-mutating manifest fixture is explicitly required and shown to the approver first; no upgrade; no rollback; no add-on modification; no node-group resize after creation; no permanent IAM changes; no additional AWS resources unless separately listed and approved; every command must use explicit `--profile`/`--region`/`--cluster-name`/`--context` values, never the ambient active kubeconfig context; `not_re_evaluated >= 1` must be genuinely exercised — no workload mutation to manufacture a finding, use the documented fallback order instead if the real baseline has no suitable finding |

## Recommended environment & policy decisions

The account owner has directed the following defaults for this
certification. These are policy recommendations prepared for sign-off —
they do not themselves constitute approval, and none of the identity
fields below (which specific account/cluster) are filled in by this
recommendation.

| Decision | Recommended value |
|---|---|
| Environment choice | Reuse an already-running non-production sandbox EKS cluster (see "Design question" below for the exact fallback order if none qualifies) |
| Resource creation permitted | No |
| EKS upgrade permitted | No |
| Rollback permitted | No |
| Node-group changes permitted | No |
| Add-on changes permitted | No |
| Kubernetes workload mutation permitted | No |
| Permanent IAM changes permitted | No |
| Temporary IAM mechanism | Approved scoped role/profile or temporary session only |
| Maximum incremental spend | $1 |
| Evidence handling | Raw evidence stays local only; sanitized evidence must pass `scripts/certification/check-evidence-sanitized.sh` before force-staging (`git add -f`, per `.gitignore`'s staging guard) |
| Cleanup | Remove temporary credentials/session, delete local raw evidence, restore the original kubeconfig context, verify no AWS or Kubernetes resource changed (see `commands.md` Phase 7's independently-verified checklist) |

These map directly onto `commands.md`'s existing mutation classification:
with "Resource creation permitted: No" and reuse-existing-cluster chosen,
Phase 0 (`eksctl create cluster`) and Phase 7's cluster-delete step are
**both out of scope** for this approval and must not be run — only the
IAM-policy create/delete pair (scoped, temporary, per "Temporary IAM
mechanism" above) and the read-only `kubepreflight`/verification commands
apply.

## Design question: cluster selection and the comparison-proof condition

`commands.md`'s Phase 0 describes creating a brand-new, disposable EKS
cluster via `eksctl create cluster -f demo/eks-case-study/cluster.yaml`
(then deleting it in Phase 7). Per the recommendation above, reusing an
already-running sandbox is preferred — fresh cluster creation is the last
resort, not the default, and applies only when an existing cluster fails
one of the criteria below.

**An existing cluster qualifies for reuse only if it satisfies all of:**
- represents the required Kubernetes version/environment for this
  certification;
- is reachable via a safe, scoped test principal (no shared
  production-critical IAM identity);
- can produce **at least one baseline finding whose responsible rule will
  not be evaluated in the manifests-only or reduced-IAM current scan** —
  this is required, not optional: `expected-results.md`'s "Comparison
  proof" section and `scripts/certification/assert-comparison.sh
  --min-not-re-evaluated 1` both require `not_re_evaluated >= 1`, and a
  comparison with zero such findings must **fail**, not vacuously pass
  (see Stage 2's independent review for why this check exists);
- satisfies this certification's evidence-isolation and sanitization
  requirements (no evidence that cannot be sanitized before commit).

**If no existing sandbox satisfies the third bullet (zero suitable
baseline findings), in this order:**
1. Do **not** mutate a workload on the existing cluster merely to
   manufacture a finding — that creates real risk for no certification
   value and is explicitly out of scope.
2. Consider a documented route using an archived/sanitized known baseline
   report instead of a freshly captured one (e.g. adapting
   `demo/eks-case-study/evidence/before/findings.json`'s shape, redacted,
   as the full-access baseline) — to be worked out and documented here if
   this path is taken.
3. Choose a different already-approved existing sandbox cluster that does
   satisfy all four criteria.
4. Only if none of the above works: fall back to `commands.md` Phase 0's
   fresh create-and-delete cluster, which is guaranteed to have zero
   AWS-plane rule executions the first time it's scanned reduced-IAM or
   manifests-only, satisfying the `not_re_evaluated >= 1` requirement
   trivially and safely.

Fill in **either** "Existing EKS cluster" (to reuse one) **or** "EKS
cluster name" (to confirm creating the one `commands.md` Phase 0
describes) in the Values table above, not necessarily both, and adjust
the "No production mutation confirmed" note below accordingly.

## This approval's scope

This approval authorizes only the commands and environment listed in this
record. It does not authorize an EKS upgrade, rollback, node-group change,
add-on modification, workload mutation, permanent IAM change, resource
creation, or resource deletion unless explicitly listed.

## What "approved" means for each line

- **AWS account approved** — confirmed to be a disposable
  sandbox/non-production account, not shared with any real workload.
- **Region approved** — confirmed EKS + the chosen instance type are
  available there; no other constraint.
- **EKS cluster approved** — either an already-running cluster the approver
  names under "Existing EKS cluster" (see the design question above), or a
  new cluster name confirmed not to collide with anything pre-existing
  (`aws eks list-clusters` checked clean first, per
  `demo/eks-case-study/README.md`'s own Step 1) if Phase 0 will create one.
- **Estimated spend accepted** — the human reviewing this has read
  `README.md`'s spend estimate and accepts it as a bound on what this
  certification may cost.
- **No production mutation confirmed** — every `MUTATING`-tagged command in
  `commands.md` is scoped to resources this certification itself creates
  (its two IAM policies, and the cluster if Phase 0 is used) and tears down
  in Phase 7; nothing pre-existing is touched. If the approver instead
  chooses to reuse an existing cluster (see the design question above),
  Phase 0's cluster-create command and Phase 7's cluster-delete command are
  both out of scope for this approval and must not be run.
- **Read-only commands reviewed** — every `READ-ONLY`-tagged command in
  `commands.md` has been read and understood before Stage 3 begins.
- **Evidence sanitization plan reviewed** — `environment.md`'s
  sanitization section and `scripts/certification/
  check-evidence-sanitized.sh` have been read/understood before any real
  evidence is captured.
- **Cleanup plan reviewed** — `commands.md`'s Phase 7 (teardown +
  independent verification checklist) has been read before Stage 3
  begins, so teardown isn't improvised after the fact.
- **Human approval recorded** — the "Values" table above is fully filled
  in with real (but still non-sensitive-in-this-doc) values, and this
  file has been committed with those values before Stage 3 execution
  starts.

## Sign-off

**Recorded below is the account owner's own explicit authorization
message, transcribed verbatim as they stated it in chat (not invented,
not inferred by an agent).** This supersedes the generic Stage 3
read-only sign-off statement originally drafted as a template — the
account owner instead gave this fuller, fresh-cluster-creation-specific
authorization directly:

```text
I am the authorized human approver for this certification environment.

I approve Stage 3 creation and use of a fresh, temporary, non-production
Amazon EKS cluster solely for KubePreflight v1.3.0 certification, subject
to the exact restrictions below.

Approved environment:

- AWS account: use the AWS account returned by `aws sts get-caller-identity`,
  but stop and show it to me before creating anything
- Region: ap-south-1
- Cluster name: kubepreflight-v130-cert
- Environment: temporary non-production certification environment
- Kubernetes version: choose a currently supported standard-support EKS
  version compatible with the repository certification plan; show me the
  selected version before creation
- Node group name: certification-ng
- Instance type: t3.small
- Desired nodes: 1
- Minimum nodes: 1
- Maximum nodes: 1
- Capacity type: On-Demand
- Root volume: smallest practical supported EBS volume
- Public workload exposure: not permitted
- NAT Gateway: not permitted
- Load Balancer: not permitted
- Elastic IP: not permitted
- Application workload deployment: not permitted unless a repository-owned,
  non-mutating manifest fixture is explicitly required and shown to me first
- Upgrade: not permitted
- Rollback: not permitted
- Add-on modification: not permitted
- Node-group resize after creation: not permitted
- Permanent IAM changes: not permitted
- Additional AWS resources: not permitted unless listed and approved
  separately
- Maximum incremental spend: $3
- Approval expiration: end of this certification session or 6 hours after
  cluster creation, whichever occurs first

I authorize only the minimum AWS and Kubernetes mutations required to:

1. create the temporary EKS control plane
2. create one managed node group with exactly one t3.small On-Demand node
3. run the approved read-only KubePreflight certification commands
4. collect local evidence
5. delete every AWS resource created by this certification

Before creating resources, perform the Stage 3 pre-execution checks and
stop for confirmation after showing:

- `aws sts get-caller-identity`
- selected AWS profile
- selected region
- proposed cluster name
- selected Kubernetes version
- exact creation command
- expected resources
- expected estimated cost
- cleanup command
- confirmation that no NAT Gateway, Load Balancer, Elastic IP, application
  deployment, upgrade or rollback will be created or invoked

Do not proceed if:

- the account or region differs from this approval
- required placeholders remain unresolved
- the command would create a NAT Gateway, Load Balancer, Elastic IP, more
  than one node, or any unapproved resource
- the estimated spend exceeds $3
- cleanup commands are not available
- the cluster name already exists
- the selected Kubernetes version is in extended support
- any approval check fails

After I confirm the displayed pre-execution plan, you may create the
cluster.

During certification:

- use explicit `--profile`, `--region`, `--cluster-name`, and `--context`
  values
- do not rely on the active kubeconfig context
- keep raw evidence only in the gitignored evidence directory
- do not stage raw evidence
- run the sanitization scanner before staging sanitized evidence
- require `not_re_evaluated >= 1`; do not mutate workloads to manufacture
  a finding
- if the real baseline has no suitable finding, stop and use the
  documented fallback order rather than modifying the cluster

Cleanup is mandatory even if certification fails:

- delete the managed node group
- delete the EKS cluster
- verify the cluster no longer exists
- verify no certification-tagged resources remain
- remove temporary kubeconfig context and temporary credentials
- delete local raw evidence after sanitized evidence is verified
- report actual runtime and estimated spend
```

```text
Approval decision: APPROVED
Approver name: Neetesh Yadav
Approver authority/role: AWS account owner or authorized operator
Approval date/time with timezone: 2026-07-27 14:29:47 IST (+0530)
Approval expiration: 6 hours after cluster creation, or end of this
  certification session, whichever occurs first. Exact clock-time value
  pending — will be recorded here once cluster creation actually
  completes (cannot be computed before creation happens).
```

**Explicit note on this authorization's own boundary**: this statement
approves cluster/node-group *creation* and the certification's read-only
use of it. It does **not** itself constitute the final "proceed" signal
for Stage 3 execution — per the account owner's own explicit instruction,
the agent must first display the full pre-execution plan (below) and wait
for a separate, final confirmation message before running any
resource-creation command.

## Stage 3 pre-execution gate

Even after this file is fully signed off, run this checklist immediately
before executing any Stage 3 command — a separate, final local gate, not
a substitute for the sign-off above:

```text
[ ] 1. approval-record.md has no required blanks remaining
[ ] 2. account/profile/region/cluster/context match every command about
       to run (compared line-by-line against commands.md, not assumed)
[ ] 3. every command about to run is from the approved, versioned
       commands.md (matches the "Approved command list/version" value
       above — no ad-hoc or improvised command)
[ ] 4. the mutating-verb scan (see Stage 2's review method: create,
       delete, update, patch, apply, replace, scale, upgrade, rollback,
       associate, attach, detach, put-, modify, set-) finds no command
       outside the explicitly approved set
[ ] 5. `aws sts get-caller-identity` output is displayed and manually
       matched against the approved account/profile before proceeding
[ ] 6. `kubectl config current-context` output is displayed and manually
       matched against the approved "Permitted Kubernetes context" value
[ ] 7. the spend ceiling above is re-confirmed against what's about to
       run (no cluster creation if reuse-existing was chosen, etc.)
[ ] 8. the raw evidence destination directories are confirmed gitignored
       (`git check-ignore docs/certification/v1.3.0/<mode>/*` — see
       .gitignore's staging guard)
[ ] 9. `scripts/certification/check-evidence-sanitized.sh` and
       `scripts/certification/assert-findings.sh`/`assert-comparison.sh`
       have been run successfully against local/synthetic data at least
       once before real evidence exists
[ ] 10. this specific Stage 3 execution session is explicitly approved
        (a fresh confirmation, not just reusing an old sign-off if time
        has passed — see "Approval expiration" above)
```

## Final Stage 3 pre-execution confirmation (received after the plan was displayed)

The account owner reviewed the exact pre-execution plan (identity, region,
cluster name, Kubernetes version and its standard-support status per AWS's
own documentation, node group shape, cost estimate, exact creation
command, expected resources, cleanup command) and gave this exact,
separate final confirmation, transcribed verbatim:

```text
I have reviewed and approved the displayed Stage 3 pre-execution plan.

I confirm that:

- AWS account ending in ...4067 is the intended temporary sandbox account.
- Region ap-south-1 is approved.
- Cluster name kubepreflight-v130-cert is approved.
- Kubernetes version 1.36 is approved.
- One managed node group named certification-ng is approved.
- Instance type t3.small is approved.
- desiredCapacity=1, minSize=1, and maxSize=1 are approved.
- A 20 GiB gp3 root volume is approved.
- The NAT-disabled public-subnet design is approved.
- The node's auto-assigned ephemeral public IPv4 address is understood and approved; no Elastic IP is authorized.
- The estimated incremental spend of approximately $0.78, with a hard ceiling of $3, is approved.
- The exact creation command `eksctl create cluster -f docs/certification/v1.3.0/cluster.yaml` is approved.
- Mandatory cleanup through `eksctl delete cluster --name kubepreflight-v130-cert --region ap-south-1 --wait` is approved.
- No NAT Gateway, Load Balancer, Elastic IP, application workload deployment, upgrade, rollback, node-group resize, add-on modification, or permanent IAM change is authorized.
- Raw evidence must remain in the gitignored evidence location and must not be staged before sanitization.
- Every AWS and Kubernetes command must use the explicitly approved region, cluster name, AWS profile or identity, and Kubernetes context.
- The certification must stop rather than mutate workloads if a genuine not_re_evaluated >= 1 proof cannot be obtained.

Proceed with Stage 3 exactly as approved.

Immediately after cluster creation:

1. Show the exact created kubeconfig context.
2. Verify the cluster status and Kubernetes version.
3. Verify the node group contains exactly one t3.small On-Demand node.
4. Verify that no NAT Gateway, Load Balancer, or Elastic IP was created.
5. Record the creation start and completion times.
6. Continue only with the approved read-only certification commands.
7. Run mandatory cleanup even if certification fails.
8. Stop and ask for renewed approval if any planned resource, command, cost, account, region, version, or context differs from this approval.
```

**Creation start time**: 2026-07-27 14:38:37 IST (+0530)
**Creation completion time**: _(recorded below once complete)_

Per item 8 of the confirmation above and this whole certification's established
cadence, cluster creation and its immediate post-creation verification are
performed now; Phase 1 (temporary IAM policies) and the actual scan
commands are treated as a distinct next step, presented for review before
execution, not run unattended as part of this same confirmation.

## Stage 3 cluster creation — post-creation verification (completed)

**Creation start**: 2026-07-27 14:38:37 IST (+0530)
**Creation completion**: 2026-07-27 14:52:18 IST (+0530) (~13m46s, confirmed from `eksctl create cluster`'s own real success markers in the log, not merely a captured exit code — the log's terminal line was "[✔] EKS cluster \"kubepreflight-v130-cert\" in \"ap-south-1\" region is ready")
**Approval expiration (computed)**: 2026-07-27 20:52:18 IST (+0530) — 6 hours after creation completion, per the approved "6 hours after cluster creation, or end of this certification session, whichever occurs first" rule

All 8 post-creation checks from the account owner's confirmation message, performed via fresh, independent read-only AWS/kubectl calls (not inferred from the creation log alone):

1. **Kubeconfig context**: `Neetesh@kubepreflight-v130-cert.ap-south-1.eksctl.io` — confirmed present via `kubectl config get-contexts`, and is what every subsequent command explicitly passes via `--context`, never the ambient current-context. **Disclosure**: this same kubeconfig file also contains contexts for at least 9 other AWS accounts unrelated to this approval (including one context named to suggest a production cluster) — none of these were queried or touched; flagged to the account owner directly in chat for awareness.
2. **Cluster status/version**: `aws eks describe-cluster` → `status: ACTIVE`, `version: 1.36` — matches approved version exactly.
3. **Node group**: `aws eks describe-nodegroup` → `status: ACTIVE`, `instanceTypes: [t3.small]`, `scalingConfig: {min:1, max:1, desired:1}`, `capacityType: ON_DEMAND` — matches approved shape exactly. `kubectl get nodes` independently confirms exactly one `Ready` node running kubelet `v1.36.2-eks`, consistent with the cluster version.
4. **No NAT/LB/EIP**: `aws ec2 describe-nat-gateways` scoped to this cluster's VPC → empty. `aws elbv2 describe-load-balancers` scoped to this VPC → empty. `aws ec2 describe-addresses` (Elastic IPs) for the whole account/region → zero. The node does have a normal auto-assigned ephemeral public IPv4 (expected, approved, not an Elastic IP, matches the NAT-disabled public-subnet design reviewed before creation).
5. **Timestamps**: recorded above.
6. Continuing only with approved read-only certification commands from here — no scan/compare/rollback command has been run yet against this cluster; that is the next, separately-presented step.
7. Cleanup command (`eksctl delete cluster --name kubepreflight-v130-cert --region ap-south-1 --wait` + independent verification) remains queued for Stage 4, to run even if certification fails.
8. No deviation from the approved plan occurred — every resource, command, cost driver, account, region, version, and context matched what was reviewed and approved before creation.

## Reduced-IAM checkpoint — release-blocking finding, PR 8 paused

The reduced-IAM scan completed successfully as a *scan* (exit 3/INCOMPLETE,
correctly reflecting partial AWS evidence at the `Coverage.AWS` plane
level), but exposed a real, release-blocking product inconsistency:

```
coverage.aws.status = partial       Result = INCOMPLETE       exit code = 3
                          but
EvaluationCoverage = Complete       Score qualification/advisory = absent
Readiness score: 77 -> 89 (only because AWS-dependent findings could no
                            longer be produced -- not a genuine improvement)
```

Root cause: PR #226's evaluation-coverage/score-qualification classifier
derives `EvaluationCoverage.Status` purely from `RuleExecutions`
(`report.BuildEvaluationCoverage`), which has no path for AWS collector
failures to propagate into any `RuleExecutionRecord.State` — confirmed
directly (`internal/rules/execution.go`'s `ruleErrorsMapKeys` covers 6
Kubernetes-plane rules only, zero AWS-plane rules), and now proven against
real infrastructure rather than only inferred from source.

**Decision: PR 8 is BLOCKED pending a corrective PR** (to combine
rule-execution coverage and evidence-plane coverage into one honest
overall decision-coverage status) and a mandatory rerun of the
reduced-IAM scan under identical conditions once that fix lands.

## Reduced-IAM temporary resource cleanup — completed

- `aws iam delete-role-policy --role-name kubepreflight-v130-cert-reduced-iam --policy-name reduced-scan-access` — succeeded
- `aws iam delete-role --role-name kubepreflight-v130-cert-reduced-iam` — succeeded
- Independently verified via `aws iam get-role` — `NoSuchEntity` (role genuinely gone, not just assumed)
- `docs/certification/v1.3.0/raw/aws-config-reduced` — removed
- Cloned kubeconfig context `kp-v130-cert-reduced-iam-test` and its user entry — removed
- Original context `Neetesh@kubepreflight-v130-cert.ap-south-1.eksctl.io` — confirmed still present and functional (`aws eks describe-cluster` → `ACTIVE`)
- Environment variables (`AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE`, `AWS_PROFILE`, `AWS_REGION`) — unset
- **Raw reduced-IAM evidence preserved** at `docs/certification/v1.3.0/reduced-iam/` (gitignored, uncommitted) as the regression proof for the corrective PR

## Cluster disposition

Cluster `kubepreflight-v130-cert` (ap-south-1) **retained** — the full-access baseline evidence and this reduced-IAM regression proof both depend on it, and the corrective-PR-then-rerun cycle is expected to complete within the current approval window. Mandatory cleanup remains queued for Stage 4 once the certification either completes or the approval window is reassessed.

## Reduced-IAM certification rerun — corrected behavior confirmed (post-PR-228)

Product source synced to merged master `a91deca`; fresh binary built;
confirmed product code byte-identical to `a91deca` (`git diff` on all
non-certification paths empty); full local test suite passed on the
synced worktree before scanning.

Reduced-IAM role/profile/context mechanism recreated identically to the
original design (role `kubepreflight-v130-cert-reduced-iam`, inline
policy `reduced-scan-access` allowing only `eks:DescribeCluster` scoped
to this cluster, named-profile role assumption, cloned kubeconfig context
`kp-v130-cert-reduced-iam-test` with the same 4 pinned exec-env vars).
All 10 pre-checks re-verified and passed identically to the original run.

Scan re-run under equivalent conditions (same cluster, same target
version, same profile/context, same command shape) against the fixed
binary. Result:

| Field | Pre-fix (preserved) | Post-fix (rerun) |
|---|---|---|
| Findings | 3 | 3 (identical) |
| `Coverage.AWS.status` | partial | partial (identical) |
| Readiness score | 89 | 89 (identical -- confirms score formula untouched) |
| `Result` | INCOMPLETE | INCOMPLETE (identical) |
| Exit code | 3 | 3 (identical) |
| Rule execution coverage | Complete (31/31 evaluated) | Complete (31/31 evaluated, identical) |
| Overall/decision coverage shown | **Complete (the bug)** | **Partial (fixed)** |
| Score interpretation text | absent | **present**: "The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score." |
| Advisory text | absent | **present**: "evidence collection was incomplete for: AWS. Review before approving the change." |

Every structural fact (findings, plane coverage, score, result, exit
code, rule-execution states) is byte-identical between the two runs --
confirming this was purely a presentation/decision-context fix, exactly
as scoped. Cross-surface parity (Terminal/Markdown/HTML) confirmed for
the new Coverage/Advisory text. Sanitization scanner correctly flagged
the new raw evidence (same categories as before); not sanitized or
staged.

**Certification decision: reduced-IAM mode -- PASS** (post-fix). No
AWS/Kubernetes mutation occurred during this rerun (identical read-only
command set to the original run).

Raw evidence preserved at both `docs/certification/v1.3.0/reduced-iam/`
(pre-fix, regression proof) and `docs/certification/v1.3.0/reduced-iam-rerun/`
(post-fix, corrected-behavior proof) -- both gitignored, neither staged.

## Approval window expiration disclosure

The approval window (`2026-07-27 20:52:18 IST`, 6 hours after cluster
creation) expired during a session interruption, before the account
owner returned to continue. Disclosed to the account owner immediately
upon discovery, before any further action was taken.

**Assessed impact**: the `kubepreflight compare` run and its sanitization
scan (both purely local operations against already-collected findings.json
files — confirmed no AWS/Kubernetes client is ever constructed on that
code path) had already run in the few minutes after expiration, before
the expiration was noticed. **No new AWS or Kubernetes API call occurred
past the expiry boundary** — the comparison results remain valid evidence.
The account owner's explicit direction: clean up all live infrastructure
immediately, then review the (still-valid) comparison results afterward.

## Full cleanup — completed and independently verified

IAM role/policy: `aws iam delete-role-policy` + `aws iam delete-role` —
both succeeded; independently re-verified via `aws iam get-role` ->
`NoSuchEntity`.

Isolated AWS config file (`docs/certification/v1.3.0/raw/aws-config-reduced`)
and cloned kubeconfig context/user (`kp-v130-cert-reduced-iam-test`) —
removed.

EKS cluster: `eksctl delete cluster --name kubepreflight-v130-cert
--region ap-south-1 --wait` — genuine success confirmed from the actual
log content (`[✔] all cluster resources were deleted`), not merely a
piped exit code.

**10-point independent verification, all passed:**
1. EKS cluster absent — `aws eks list-clusters` returns empty.
2. Managed node group absent — `ResourceNotFoundException` (cluster gone).
3. EC2 instance(s) — none remain tagged to this cluster.
4. EBS volume(s) — none remain tagged to this cluster.
5. eksctl CloudFormation stacks — none remain for this cluster name.
6. Temporary IAM role — confirmed absent (re-verified).
7. Temporary kubeconfig context — confirmed absent (re-verified).
8. Original certification context (`Neetesh@kubepreflight-v130-cert.ap-south-1.eksctl.io`)
   — removed automatically by `eksctl delete cluster`'s own kubeconfig
   update, correctly, since it belonged to the now-deleted cluster.
9. Every other pre-existing kubeconfig context (9+ other AWS accounts,
   `kind-kp-smoke`) — confirmed still present and untouched.
10. Raw evidence — all five evidence directories (`full-access`,
    `reduced-iam`, `reduced-iam-rerun`, `manifests-only`,
    `comparisons/full-vs-manifests`) still contain their real files on
    disk, all confirmed gitignored, nothing staged (`git status --short`
    clean).

**All live infrastructure for this certification is now fully torn
down.** Remaining work (Stage 4: sanitize evidence, run assertions,
create checksums, prepare certification summary, commit sanitized PR 8
evidence) is entirely local-only.
