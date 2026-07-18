# Container image scanning

KubePreflight scans its container image with Trivy before merge and before a
tagged release is published. The scan covers operating-system packages and
application dependencies, reports High and Critical vulnerabilities to GitHub
code scanning as SARIF, and fails the workflow when a fixed High or Critical
finding is present.

The pull-request workflow builds the image from the checked-out `Dockerfile`.
The release workflow scans the published GHCR image by digest before GitHub
Release assets are created, so release artifacts are not published when the
release image violates the vulnerability policy.

The release-blocking scanner currently runs with `ignore-unfixed: true`.
Findings without an upstream fix are excluded from the blocking gate until a
patched version is available.

## Ignore process

KubePreflight does not use permanent blanket ignores. Any future vulnerability
exception must be submitted as a normal pull request and must include:

- the specific vulnerability ID;
- the affected image or image digest;
- why the finding is not exploitable or cannot be fixed yet;
- a named owner;
- an expiration date;
- the planned removal or remediation path.

Expired exceptions must be removed or renewed in a follow-up pull request with
fresh justification. Broad package, directory, severity, or scanner-wide ignores
are not acceptable for release-blocking scans.
