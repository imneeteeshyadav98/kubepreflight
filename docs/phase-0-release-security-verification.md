# Phase 0 release and security verification

Status date: 2026-07-18

## KP-REL-001 — Deployed release verification

Current public release checked:

- Release: `v0.14.0`
- Published: 2026-07-17 18:10:05 UTC
- Annotated tag object: `b38e54cc89fb6d4304b35360aaabf54a8732e2a7`
- Tag target commit: `d66bc5b77238f60d6e9793e09884e217fc72d8f1`
- Release assets present: Linux amd64/arm64, macOS amd64/arm64, Windows amd64, checksums, SPDX SBOM

Current `origin/master` checked:

- Commit: `03f8aad635967f5d74b735a89b95c4c9acc5a6b0`
- Distance from `v0.14.0`: `v0.14.0-4-g03f8aad`

Finding:

- The published `v0.14.0` release is real and has the expected release artifacts.
- The sensitive-identifier redaction feature is on `origin/master`, after `v0.14.0`.
- Do not claim redaction is included in `v0.14.0`; it needs a new release tag after validation.

## KP-SEC-001 — Redaction verification

Verified redaction scope on `origin/master`:

- `scan --redact-sensitive-identifiers`
- `plan --redact-sensitive-identifiers`
- `rollback plan --redact-sensitive-identifiers`
- `rollback assess --redact-sensitive-identifiers`
- `compare --redact-sensitive-identifiers`

Validated surfaces:

- `findings.json`
- `report.md`
- `report.html`
- terminal report output
- `upgrade-plan.json`
- plan HTML and terminal summary
- `rollback-assessment.json`
- `rollback-report.md`
- `rollback-report.html`
- rollback terminal output
- `comparison.json`
- `comparison.md`
- compare terminal warning output

Regression coverage:

```bash
GOCACHE=/tmp/kubepreflight-go-cache go test ./internal/redact
GOCACHE=/tmp/kubepreflight-go-cache go test ./internal/cli ./internal/report ./internal/comparison ./internal/plan ./internal/rollback ./internal/redact
GOCACHE=/tmp/kubepreflight-go-cache go test ./...
```

All listed checks passed on `03f8aad635967f5d74b735a89b95c4c9acc5a6b0`.

Release-lock requirement:

- Create the next release from a commit that includes `internal/redact/render_output_test.go`.
- Re-run the full test suite before tagging.
- Verify the release assets, checksum file, and SBOM after publication.
