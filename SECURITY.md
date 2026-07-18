# Security Policy

## Supported Versions

KubePreflight currently supports the latest published release line for security
fixes. Older releases are unsupported unless a security advisory explicitly
states otherwise.

If you are using an older release, upgrade to the latest release before filing
a security report unless the issue is only reproducible on that older release.

## Reporting a Vulnerability

Please report suspected vulnerabilities privately through GitHub Security
Advisories:

https://github.com/imneeteeshyadav98/kubepreflight/security/advisories/new

Do not open a public GitHub issue with vulnerability details, credentials,
kubeconfig contents, AWS account identifiers, private hostnames, generated
reports, or other sensitive evidence. Public issues are appropriate for
ordinary bugs, usage questions, and feature requests only.

We aim to acknowledge valid security reports within 3 business days.
Investigation and remediation timelines depend on severity and complexity.

## In Scope

Security reports are in scope when they have reproducible product impact in:

- KubePreflight CLI behavior
- generated JSON, Markdown, HTML, or terminal reports
- the local Console/report server
- the GitHub Action
- the published container image
- the release pipeline and published artifacts

Examples include credential exposure, unsafe report rendering, bypasses of
documented redaction behavior, unintended cluster or cloud mutation, release
artifact integrity issues, and vulnerable behavior in the local report server.

Container image vulnerability scanning and the exception process for scanner
findings are documented in [Container image scanning](docs/container-image-scanning.md).

## Out of Scope

The following are generally out of scope:

- third-party Kubernetes, Amazon EKS, AWS, GitHub, browser, operating-system, or
  container-runtime behavior unless KubePreflight creates a reproducible product
  impact
- reports without a reproducible security impact
- findings that require already-compromised local machines, cluster
  credentials, repository credentials, or cloud credentials without showing an
  additional KubePreflight-specific impact
- generic dependency or scanner output without an exploit path, affected
  KubePreflight behavior, or supported remediation

## Safe Disclosure Expectations

When reporting privately, include:

- affected KubePreflight version or commit
- operating system and installation method
- command or workflow used
- minimal reproduction steps
- sanitized report excerpts or generated artifacts when relevant
- expected impact and whether credentials, cluster access, or AWS access are
  required

Do not include live credentials, kubeconfig files, unredacted AWS account IDs,
private node hostnames, production report artifacts, or customer data. If
evidence is sensitive, describe what you can share and coordinate privately
before sending it.

## Repository Settings Verification Checklist

The following settings cannot be safely assumed from repository code alone.
Maintainers should verify them after security-baseline changes merge:

- default branch protection enabled
- pull request required before merge
- required status checks configured
- code-owner review required
- force pushes disabled
- branch deletion restricted as intended
- private vulnerability reporting enabled
- secret scanning status confirmed
- push protection status confirmed
