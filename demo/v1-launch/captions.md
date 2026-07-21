# Captions and on-screen text — verbatim

Every string that appears on screen anywhere in the video, listed exactly
as rendered, with its source. Nothing below was paraphrased or
approximated from evidence — where a source file is named, the on-screen
text is a direct copy or a verified subset of that file's content.

## Scene 1 — opening title (0.0s–3.0s)

```
KubePreflight
v1.0.0
● Kubernetes & EKS upgrade and rollback readiness
● Verified against real EKS
● Read-only — never upgrades or rolls back your cluster
```

## Scene 2 — terminal (3.0s–8.0s)

Typed command (source: `evidence/scan-command.txt`, flags reordered onto
multiple lines for on-screen legibility only — every flag and value is
present verbatim in the real command):

```
$ kubepreflight scan \
    --provider eks \
    --cluster-name redacted-eks-cluster \
    --target-version 1.34 \
    --output all \
    --redact-sensitive-identifiers
```

The real captured command uses `--cluster-name kp-v1-rc-smoke` (a
disposable smoke-test cluster name). The on-screen version replaces it
with `redacted-eks-cluster` — deliberately *not* a plausible-looking
identifier like `production`, to avoid any misreading that the
SEC-TRUST-002 verification ran against a live production cluster. A
caption under the terminal window states this explicitly: "Real
disposable EKS cluster — SEC-TRUST-002 verification run, name redacted".
This is the one deliberate wording substitution in the entire video, and
it changes only a cluster-name argument value, not any finding, score, or
result.

Revealed output lines (source: `evidence/scan-stdout.txt`, verbatim except
the cluster name matches the substitution above):

```
Collected: 2 nodes, 6 pods, 1 PDBs, 3 webhooks, 3 services, 3 endpointslices, 7 CRDs, 1 deployments, 2 daemonsets

KubePreflight scan — cluster: redacted-eks-cluster  target: 1.34  provider: eks
Result: BLOCKED

Upgrade Readiness: BLOCKED — Score: 75/100 — Upgrade Continue: No
```

Below the terminal window, a caption reads:

```
Real disposable EKS cluster — SEC-TRUST-002 verification run, name redacted
```

## Scene 3 — findings (8.0s–12.0s)

```
What KubePreflight found
Not a linter. A readiness signal.

ADDON-001 · Blocker
EKS add-on kube-proxy is below the catalog minimum required for Kubernetes 1.34.

EKS-NG-002 · Warning
Managed node group has limited rolling-update headroom.

WH-005 · Warning
An admission webhook's scope deserves review.
failurePolicy: Ignore today — the finding flags the risk if that ever changes to fail-closed.
```

Source verification: `ADDON-001` message is a shortened paraphrase of
`evidence/scan-stdout.txt`'s `[P2/ADDON-001] EKS add-on "kube-proxy" is on
version v1.33.10-eksbuild.11, which is below the catalog minimum
v1.34.0-eksbuild.1 for target Kubernetes 1.34`. `EKS-NG-002` paraphrases
`Managed node group … desired size equals or is below minimum size.
Rolling update may have limited disruption headroom.` `WH-005`'s
`failurePolicy: Ignore` line is a direct read of the evidence field
`failurePolicy: Ignore` under `[P4/WH-005]` — this is the wording the user
specifically flagged as needing to stay accurate, since the finding is
about the webhook's *current* `Ignore` policy and the *risk if it changes*,
not a claim that the webhook is already fail-closed.

## Scene 4 — report formats overview (12.0s–13.5s)

```
Every scan produces
One result, four formats.

terminal — Human-first, exit code carries the verdict
findings.json — Canonical, machine-readable
report.md — PR comments, tickets
report.html — Standalone, no server required
```

## Scene 5 — real report.html (13.5s–16.0s)

This is the real page, with one cosmetic change: the recorder replaces the
visible `CLUSTER` value (and every other on-page text occurrence of the
same string) from `kp-v1-rc-smoke` to `redacted-eks-cluster` via a
DOM-only text substitution, immediately after page load and before
capture. **The displayed cluster name is cosmetically redacted for public
distribution. Findings, score, verdict, and remediation text are
unchanged.** `evidence/scan-report.html` on disk is never modified — see
`record-browser.mjs`'s `redactClusterName()` and README.md's "Evidence
provenance and normalization" section.

Overlay caption:

```
CLI · JSON · Markdown · HTML
```

## Scene 6 — real Console (16.0s–22.0s)

No custom text — this is the real product UI, navigated through its
Findings / Next Actions / Rollback tabs. Overlay caption:

```
Reviewable evidence, not a pass/fail guess
```

## Scene 7 — compare + rollback (22.0s–27.0s)

```
Compare progress · assess rollback posture
The released binary and container agree.

kubepreflight compare
Comparison gate — pass
New blockers — 0
New warnings — 0
Binary vs container — same findings

kubepreflight rollback plan / assess
Eligibility — unavailable
Readiness — blocked
Recommendation — do_not_proceed
Confidence — high
```

Source verification: comparison gate/blocker/warning counts read from
`evidence/gate.json` and `evidence/comparison.json`; rollback readiness,
recommendation, and confidence read from
`evidence/rollback-plan-assessment.json` /
`evidence/rollback-assess-assessment.json`.

## Scene 8 — closing title (27.0s–30.0s)

```
KubePreflight
v1.0.0 · Read-only Kubernetes / EKS upgrade readiness · Verified against real EKS
kubepreflight.com
github.com/imneeteeshyadav98/kubepreflight
Open source
```
