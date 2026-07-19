# Published Installation Matrix

KP-V1-INSTALL-001. Everything in `release.yml` up through `github-release`
only proves the binaries and container *build* correctly. The jobs
described here prove the artifacts a user actually installs -- downloaded
release archives, the GHCR image, `go install`, and the GitHub Action --
work, by operating entirely on what was actually published, never a local
`go build`/rebuild of the tagged commit.

## Tested platforms

Automatically, on every tag push (part of `release.yml`, `needs:
github-release`):

| Platform | Job | Depth |
| --- | --- | --- |
| Linux amd64 | `verify-published-release-linux` | Deep: checksum, SBOM, version/`--version` parity and real provenance, `--help` and `--redact-sensitive-identifiers` on all five command surfaces, deterministic fixture scan (exit code 2, verdict `BLOCKED`), all three report formats present and parseable, redaction leak grep |
| macOS arm64 (Apple Silicon) | `verify-published-release-macos` (matrix) | Deep, same checks as Linux above |
| macOS amd64 (Intel) | `verify-published-release-macos` (matrix) | Light: checksum, extract, version/`--help` on all five surfaces. No duplicate fixture-scan smoke -- one architecture proving the scan/redaction/report pipeline works is sufficient; duplicating it on both only proves the same Go code runs twice, not a new class of bug |
| Windows amd64 | `verify-published-release-windows` | Deep, same checks as Linux, via PowerShell (`scripts/release/verify-published-binary.ps1`) |
| GHCR container (both aliases) | `verify-published-container` | Both `<bare-version>` and `<v-tag>` GHCR tags exist, resolve to **the same immutable digest** (via `docker buildx imagetools inspect`, not a mutable local image ID), and the container reports matching release provenance |
| `go install` | `verify-go-install` | See "go install" below -- currently expected to fail for a known, specific, documented reason |

Manually, via `workflow_dispatch`, updated to a new tag by hand each time
it's meaningfully re-validated (`.github/workflows/validate-published-action.yml`):

| Surface | What's checked |
| --- | --- |
| `imneeteeshyadav98/kubepreflight@<tag>` | Resolves from the real public tag (no repository-local fallback exists -- `entrypoint.sh` hard-fails if `github.action_ref` is empty), runs a manifests-only scan against a deterministic fixture, produces a `BLOCKED` verdict with a parseable `findings.json` and report file |
| `imneeteeshyadav98/kubepreflight/compare@<tag>` | Same resolution guarantee, runs against a known-pass fixture pair, produces a `pass` decision with parseable comparison/gate JSON |

This second workflow is deliberately not part of the tag-push release
pipeline: `uses: owner/repo@<tag>` can't reference a tag until after that
tag (and its GHCR image) are fully published, which is exactly the moment
a same-run trigger would need to already be pointing at it. It also can't
use a dynamic `${{ env.REF }}` in the `uses:` line -- GitHub Actions
doesn't support that -- so the pinned tag and the `REF` env var (used only
in messages, not resolution) have to be bumped together, by hand, in that
file, matching the existing convention `validate-comparison-rc.yml`
already established for RC tags.

## Release archive contract

Every archive (`kubepreflight_<tag>_<os>_<arch>.{tar.gz,zip}`) contains the
`kubepreflight` (or `kubepreflight.exe`) binary plus `README.md` and
`LICENSE`. Checksums for every asset live in one
`kubepreflight_<tag>_checksums.txt`; the SBOM is
`kubepreflight_<tag>_sbom.spdx.json` (SPDX 2.3). The binary's `version` and
`--version` output is identical and always three lines: `KubePreflight
<version>`, `commit: <short SHA>`, `built: <RFC3339 timestamp>` -- never
`dev`/`unknown`/`unknown` for a real release build (that fallback is only
for an unflagged local `go build`).

## GHCR aliases

`ghcr.io/imneeteeshyadav98/kubepreflight` is published under two tags per
release: the bare version (`0.16.2-security-trust`) and the `v`-prefixed
git tag (`v0.16.2-security-trust`). Both must resolve to the same
manifest-index digest. This is a permanent regression guard, not a nice-to
-have: `v0.16.1-security-trust` shipped with only the bare alias, because
`docker/metadata-action`'s `type=semver` tag type silently discards any
custom `pattern` for a pre-release-shaped tag (see the `docker` job's
`tags:` block and its comments in `release.yml`). Every tag this project
has ever cut is pre-release-shaped (`X.Y.Z-suffix`), so this bug was
permanent and release-wide until `type=ref,event=tag` replaced the broken
pattern in `v0.16.2-security-trust`.

## `go install`

`go install github.com/imneeteeshyadav98/kubepreflight/cmd/kubepreflight@<tag>`
is **not** advertised anywhere (README, this docs directory, the website)
as a supported install method, and `verify-go-install` exists because
KP-V1-INSTALL-001 found out why: `go.mod` declares this module's path as
plain `kubepreflight`, not its GitHub import path, so Go's module
resolution rejects the request outright --

```text
module declares its path as: kubepreflight
        but was required as: github.com/imneeteeshyadav98/kubepreflight
```

This is a materially different (and larger) gap than "go install works but
has no injected version" -- fixing it means changing `go.mod`'s module
directive, which means rewriting every internal import path across the
whole codebase, a repo-wide change well outside a CI-verification PR's
scope. `verify-go-install` asserts the actual current behavior honestly:
it passes when `go install` fails for this exact, known reason, and fails
loudly if `go install` fails for any *other* reason (a real regression) or
unexpectedly starts succeeding (a signal that the module path was fixed
and this job needs updating, not a silent pass).

Separately, even if the module path were fixed, `go install <mod>@<tag>`
does not run this project's release-workflow `-ldflags`, so a
`go install`-built binary would report the safe `dev`/`unknown`/`unknown`
buildinfo fallback, not real release provenance -- deriving a stable
version for that path (e.g. from the module's own build info) is tracked
separately as a possible future item, not implemented or asserted here.
Nothing here fakes release metadata for a path that can't actually carry
it.

## What's verified automatically vs. what needs a live cluster

Everything above runs against static fixtures (`testdata/manifest-repo/raw`,
`compare/testdata/fixtures/pass`) and needs no Kubernetes cluster, AWS
credentials, or network access beyond GitHub/GHCR/the Go module proxy. It
proves the *artifacts* are correct and internally consistent. It does not
and cannot prove `--provider eks` enrichment, live-cluster collection, or
end-to-end redaction against real AWS ARNs/account IDs/EC2 hostnames --
that needs an actual EKS cluster, tracked separately as SEC-TRUST-002 and
run whenever the next disposable cluster is available, not on a fixed
schedule.

## Latest successful matrix release

`v0.16.2-security-trust` -- update this line whenever the full matrix
(all jobs in the table above, plus a manual `validate-published-action.yml`
run) has actually passed against a specific release, so this document
never silently drifts ahead of what was really verified. Nothing else in
this file needs to change for an ordinary patch release.
