# Live EKS Released-Artifact Smoke

SEC-TRUST-002 verifies released KubePreflight artifacts against a real,
disposable EKS cluster without mutating Kubernetes or AWS resources. This page
documents the harness added for that proof. Merging the harness is not the same
as completing SEC-TRUST-002:

```text
Harness implemented and merged: IMPLEMENTATION COMPLETE
Real released binary/container on disposable EKS: EXECUTION COMPLETE (v1.0.0-rc.2)
```

The v1 acceptance sequence is:

1. Merge this harness.
2. Run the clean master gate.
3. Cut `v1.0.0-rc.1`.
4. Run this harness against the RC binary and RC GHCR image digest.
5. Confirm binary/container parity and real redaction.
6. Mark SEC-TRUST-002 complete only after the live run passes.

Running the harness against an older release, such as `v0.17.1`, is useful as a
rehearsal but is not v1 acceptance proof.

## Required Tools

- `aws`
- `docker`
- `gh`
- `jq`
- `kubectl`
- `python3`

The Docker parity path uses a short-lived EKS bearer-token kubeconfig generated
from `aws eks get-token`. That avoids depending on an `aws` exec plugin inside
the distroless release image.

## Required Environment

Set every value explicitly:

```bash
export EXPECTED_AWS_ACCOUNT_ID=123456789012
export EXPECTED_AWS_REGION=us-east-1
export EXPECTED_EKS_CLUSTER=kp-v1-rc-smoke
export EXPECTED_KUBE_CONTEXT=arn:aws:eks:us-east-1:123456789012:cluster/kp-v1-rc-smoke
export RELEASE_TAG=v1.0.0-rc.1
export EXPECTED_RELEASE_COMMIT=<merged-master-commit>
export EXPECTED_IMAGE_DIGEST=sha256:<published-ghcr-digest>
export TARGET_VERSION=1.34
export PLAN_TO_VERSION=1.34
export SEC_TRUST_LIVE_EKS_CONFIRM="read-only-live-eks-smoke:${EXPECTED_AWS_ACCOUNT_ID}:${EXPECTED_AWS_REGION}:${EXPECTED_EKS_CLUSTER}:${EXPECTED_KUBE_CONTEXT}:${RELEASE_TAG}"
```

`PLAN_TO_VERSION` must be an upgrade target for the live cluster. If it is not
greater than the detected current version, `kubepreflight plan` should fail
before evidence can prove the planner surface.

`EXPECTED_RELEASE_COMMIT` should be the full merged master commit. Published
artifacts may report a short commit in `kubepreflight version`; the harness
accepts that only when it is a prefix of the full expected commit.

Released binaries and containers are built as separate artifacts. Their build
timestamps must be present and non-`unknown`, but they are not required to be
byte-identical timestamps.

Optional overrides:

```bash
export GH_REPO=imneeteeshyadav98/kubepreflight
export IMAGE_REPOSITORY=ghcr.io/imneeteeshyadav98/kubepreflight
export ARCHIVE_SUFFIX=linux_amd64
export LIVE_EKS_WORKDIR=live-eks-evidence
```

## Workflow

Run from the repository root:

```bash
scripts/live-eks/preflight.sh
scripts/live-eks/download-release.sh
scripts/live-eks/run-smoke.sh
```

The preflight confirms:

- `aws sts get-caller-identity`
- `aws eks describe-cluster --name "$EXPECTED_EKS_CLUSTER"`
- `kubectl config current-context`
- `kubectl auth can-i --list`
- the harness command inventory contains no known mutation commands

The release download step verifies:

- GitHub Release archive checksum
- SPDX SBOM shape
- binary version, commit, and non-`unknown` build timestamp
- GHCR `v` and bare tag aliases resolve to `EXPECTED_IMAGE_DIGEST`
- container version, commit, and non-`unknown` build timestamp

The smoke step captures:

- `scan` terminal, `findings.json`, `report.md`, and `report.html`
- `plan` terminal, `findings.json`, `upgrade-plan.json`, `report.md`, and
  `report.html`
- `rollback plan` terminal, `rollback-assessment.json`, `rollback-report.md`,
  and `rollback-report.html`
- `rollback assess` terminal, `rollback-assessment.json`,
  `rollback-report.md`, and `rollback-report.html`
- `compare` terminal, `comparison.json`, `comparison.md`, and `gate.json`
- binary/container finding-key and verdict parity
- sanitized evidence under `live-eks-evidence/sanitized`

Exit codes `0`, `1`, `2`, and `3` are accepted as product outcomes. Exit code
`4` fails the harness because it represents an internal or document error.

## Read-Only Boundary

The harness intentionally invokes only read-only AWS, kubectl, and
KubePreflight commands. The static and recorded command inventory rejects:

- `kubectl apply/create/delete/patch/edit/label/annotate/replace/scale`
- `kubectl cordon/uncordon/drain/taint`
- `aws eks update-cluster-version`
- `aws eks update-nodegroup-version`
- `aws eks start-update`
- rollback execution commands

KubePreflight rollback coverage is assessment-only: `kubepreflight rollback
plan` and `kubepreflight rollback assess`.

## Evidence Handling

Raw evidence in `live-eks-evidence/raw` can contain cluster metadata and a
short-lived bearer-token kubeconfig. Do not commit raw evidence.

`scripts/live-eks/sanitize-evidence.sh` copies shareable evidence to
`live-eks-evidence/sanitized`, excluding raw cluster metadata and downloaded
release artifacts. It replaces:

- AWS account IDs
- AWS ARNs
- EC2 private hostnames
- short-lived EKS tokens

`scripts/live-eks/check-redaction.sh` fails if sanitized evidence still matches
account ID, ARN, or EC2 private hostname patterns.

## Completion Evidence

After the RC run, the SEC-TRUST-002 record should prove:

- released binary provenance
- released container provenance
- same version, commit, and build timestamp
- same findings and verdicts across binary and container
- `scan`, `plan`, and `compare`
- `rollback plan` and `rollback assess`
- AWS account ID redacted
- cluster ARN redacted
- private node hostname redacted
- terminal, JSON, Markdown, and HTML outputs valid
- no mutation performed
- sanitized evidence leak scan clean

If no disposable EKS cluster is available, keep this phase as execution pending.
Do not mark SEC-TRUST-002 complete from harness implementation alone.

## Latest successful live run

`v1.0.0-rc.2` -- update this line whenever the full sequence above (all
completion-evidence items, against a real disposable cluster) has actually
passed, so this document never silently drifts ahead of what was really
verified.

The first live run, against `v1.0.0-rc.1`, found one real product bug
(`rollback plan`/`rollback assess` doubled `--output-dir` onto an
already-prefixed `--assessment-out`, fixed in `internal/cli/rollback.go`,
requiring `v1.0.0-rc.2`) and several harness-only bugs this run alone could
expose, since none of them are reachable from a mocked cluster or a
manifests-only fixture: a `jq` quoting error in `preflight.sh`; a relative
kubeconfig path that `docker run -v` silently misparses as an invalid named
volume; the container's fixed nonroot UID having neither read access to a
host-mounted `~/.aws` (owned by the invoking user) nor write access to its
own output directory; `aws-sdk-go-v2`'s shared config/credentials loading
never working for an arbitrary `--user` UID inside a `CGO_ENABLED=0`
distroless image regardless of `$HOME`/`AWS_SHARED_CREDENTIALS_FILE`,
fixed by exporting `aws configure export-credentials` as bare `-e KEY`
docker flags (never embedding the value, since the logged command line
becomes evidence); `sanitize-evidence.sh` recursively copying its own
output into itself because its destination directory lives inside the tree
it walks; and `parity-summary.json` referencing a `findings.json.tool`
field that has never existed, silently reporting `null` for the exact
"same version, commit" comparison this document lists as required
completion evidence.
