# Test KubePreflight in 5 Minutes

KubePreflight is read-only by design: it audits Kubernetes upgrade-readiness
signals and writes reports, but it does not modify your cluster, manifests, or
cloud account. For a first test, start with manifests or a non-production
cluster. Review and redact any generated report before attaching it to an issue,
posting screenshots, or sharing it publicly.

## Quick Test

Fastest path if Docker is already installed: create one intentionally outdated
manifest, run a manifest-only scan, then open the generated report.

```bash
rm -rf /tmp/kubepreflight-5min
mkdir -p /tmp/kubepreflight-5min/manifests /tmp/kubepreflight-5min/out

cat > /tmp/kubepreflight-5min/manifests/psp.yaml <<'YAML'
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: manifest-restricted
spec:
  privileged: false
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: RunAsAny
  fsGroup:
    rule: RunAsAny
  volumes:
    - configMap
    - secret
YAML

docker run --rm --user "$(id -u):$(id -g)" \
  -v /tmp/kubepreflight-5min/manifests:/work/manifests:ro \
  -v /tmp/kubepreflight-5min/out:/work/out \
  ghcr.io/imneeteeshyadav98/kubepreflight:latest \
  scan --manifests-only --manifests /work/manifests --target-version 1.36 \
  --output all --output-dir /work/out --serve-report never

echo "scan exit code: $?"
ls -1 /tmp/kubepreflight-5min/out
```

Expected result: the scan reports `BLOCKED`, exits `2`, and writes
`findings.json`, `report.md`, and `report.html` under
`/tmp/kubepreflight-5min/out`. Exit `2` means KubePreflight found a blocker; it
does not mean Docker or KubePreflight failed.

After reviewing the report, share feedback with the
[First External Test Report](https://github.com/imneeteeshyadav98/kubepreflight/issues/new?template=first_external_test_report.yml)
issue form.

## Option 1: Install the binary

This installs the latest GitHub Release for Linux or macOS without hard-coding a
version:

```bash
REPO=imneeteeshyadav98/kubepreflight
VERSION="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" | sed 's#.*/tag/##')"

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

ASSET="kubepreflight_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fLO "https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
curl -fLO "https://github.com/${REPO}/releases/download/${VERSION}/kubepreflight_${VERSION}_checksums.txt"
grep "  ${ASSET}$" "kubepreflight_${VERSION}_checksums.txt" > "${ASSET}.sha256"
if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 -c "${ASSET}.sha256"
else
  sha256sum -c "${ASSET}.sha256"
fi

tar -xzf "${ASSET}"
sudo install "kubepreflight_${VERSION}_${OS}_${ARCH}/kubepreflight" /usr/local/bin/kubepreflight

kubepreflight version
kubepreflight scan --help
```

Manual fallback: open the
[latest release](https://github.com/imneeteeshyadav98/kubepreflight/releases/latest),
download the archive for your OS/architecture, extract it, and run
`kubepreflight version`.

## Option 2: Run with Docker

The public image is published to GHCR:

```bash
docker run --rm ghcr.io/imneeteeshyadav98/kubepreflight:latest version
docker run --rm ghcr.io/imneeteeshyadav98/kubepreflight:latest scan --help
```

Containers do not automatically have access to your files or kubeconfig. Once
you have a manifest directory, mount only what the scan needs, and prefer
read-only mounts for inputs. The next section creates
`/tmp/kubepreflight-5min/manifests`:

```bash
mkdir -p /tmp/kubepreflight-5min/manifests /tmp/kubepreflight-5min/out

docker run --rm --user "$(id -u):$(id -g)" \
  -v /tmp/kubepreflight-5min/manifests:/work/manifests:ro \
  -v /tmp/kubepreflight-5min/out:/work/out \
  ghcr.io/imneeteeshyadav98/kubepreflight:latest \
  scan --manifests-only --manifests /work/manifests --target-version 1.36 \
  --output all --output-dir /work/out --serve-report never
```

For a cluster scan in Docker, mount kubeconfig explicitly and read-only, for
example `-v "$HOME/.kube:/home/nonroot/.kube:ro"`, then pass
`--kubeconfig /home/nonroot/.kube/config`. Do this only for a non-production
first test.

## Test Kubernetes manifests

This manifest-only scan needs no cluster access or kubeconfig. It creates a
small sample that uses `policy/v1beta1` `PodSecurityPolicy`, an API removed in
Kubernetes 1.25, then scans it against target Kubernetes `1.36`.

```bash
mkdir -p /tmp/kubepreflight-5min/manifests /tmp/kubepreflight-5min/out

cat > /tmp/kubepreflight-5min/manifests/psp.yaml <<'YAML'
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: manifest-restricted
spec:
  privileged: false
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: RunAsAny
  fsGroup:
    rule: RunAsAny
  volumes:
    - configMap
    - secret
YAML

kubepreflight scan \
  --manifests-only \
  --manifests /tmp/kubepreflight-5min/manifests \
  --target-version 1.36 \
  --output all \
  --output-dir /tmp/kubepreflight-5min/out \
  --serve-report never
```

Expected behavior: this scan should find one blocker and exit `2`. That is a
successful scan with findings, not an installation failure. The reports are
written to:

```text
/tmp/kubepreflight-5min/out/findings.json
/tmp/kubepreflight-5min/out/report.md
/tmp/kubepreflight-5min/out/report.html
```

To keep a shell session going after the expected non-zero exit, rerun the same
command with `; echo "scan exit code: $?"` at the end.

## Scan a non-production cluster

Do not run your first test against production. Start with a disposable, local,
or otherwise non-production cluster where read-only cluster-wide inventory is
acceptable.

```bash
kubectl config current-context
kubectl get nodes

kubepreflight scan \
  --target-version 1.36 \
  --output all \
  --output-dir /tmp/kubepreflight-5min/cluster-out \
  --serve-report never
```

With no `--kubeconfig` flag, KubePreflight uses the same kubeconfig loading
rules as `kubectl`. The Kubernetes credential needs read-only access to list the
resource types used by the scan, plus a single allowlisted `get` on the
`kube-system/coredns` ConfigMap. See
[`deploy/clusterrole.yaml`](../deploy/clusterrole.yaml) for copy-pasteable RBAC.

KubePreflight reports upgrade-readiness gaps; it does not guarantee an upgrade
is safe.

## Protect Sensitive Identifiers

Use the current redaction flag before sharing generated evidence:

```bash
kubepreflight scan \
  --manifests-only \
  --manifests /tmp/kubepreflight-5min/manifests \
  --target-version 1.36 \
  --output all \
  --output-dir /tmp/kubepreflight-5min/redacted-out \
  --serve-report never \
  --redact-sensitive-identifiers
```

Redaction does not change findings, scores, verdicts, or exit codes. You must
still review reports before attaching them to GitHub issues, posting
screenshots, or sharing them publicly. Look for AWS account IDs, cluster names,
node hostnames, internal domains, namespaces, resource names, and any other
organization-specific identifiers.

## What Output Should I Expect?

A scan reports:

- an upgrade/readiness decision such as `CLEAN`, `WARNINGS`, `BLOCKED`, or
  `INCOMPLETE`
- blockers, warnings, and informational findings
- supporting evidence for each finding
- remediation guidance
- terminal output plus `findings.json`, and optionally `report.md`/`report.html`
- non-zero exit codes when findings or incomplete evidence are detected

Current scan exit codes:

| Code | Meaning |
|---|---|
| `0` | Clean: no blockers or warnings |
| `1` | Warnings only |
| `2` | Blockers found |
| `3` | Assessment incomplete because requested evidence could not be collected |
| `4` | Scan infrastructure failure; no trustworthy report was produced |

Sanitized example from the manifest scan above:

```text
KubePreflight scan - cluster: -  target: 1.36  provider: cluster-only
Result: BLOCKED

Blockers (1)
  [P2/API-001] PodSecurityPolicy "manifest-restricted" (apiVersion policy/v1beta1)
  in psp.yaml uses an API version removed in Kubernetes 1.25 - this manifest
  will fail to apply once the cluster reaches target 1.36

Reports written:
  /tmp/kubepreflight-5min/out/findings.json
  /tmp/kubepreflight-5min/out/report.md
  /tmp/kubepreflight-5min/out/report.html
```

## Share Feedback

Please report first-test feedback using the
[First External Test Report](https://github.com/imneeteeshyadav98/kubepreflight/issues/new?template=first_external_test_report.yml)
issue form.

Helpful reports include:

- false positives
- missing upgrade checks
- unclear remediation
- unexpected severity
- Kubernetes distribution and version
- KubePreflight version

## Troubleshooting

**Binary architecture mismatch:** run `uname -s` and `uname -m`, then download
the matching release archive. `amd64` is for Intel/AMD x86_64; `arm64` is for
Apple Silicon and ARM64 Linux.

**Docker volume/path issue:** create the host output directory first, mount input
manifests read-only, and remember that container paths such as `/work/manifests`
are not the same as host paths.

**Kubeconfig/context issue:** run `kubectl config current-context` and
`kubectl get nodes` before a cluster scan. If those fail, KubePreflight cannot
scan that cluster with the same credentials.

**RBAC access denied:** use a credential with read-only access to the resources
listed in [`deploy/clusterrole.yaml`](../deploy/clusterrole.yaml). Missing
permissions can produce incomplete coverage.

**Scan exits non-zero:** exit `1` or `2` can mean KubePreflight completed
successfully and found warnings or blockers. Read the terminal summary and the
generated reports before treating the run as a tooling failure.
