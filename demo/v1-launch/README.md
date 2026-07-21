# KubePreflight v1.0.0 launch demo

A 25-30 second demo video for the v1.0.0 launch, built entirely from real,
already-sanitized SEC-TRUST-002 evidence — no new EKS cluster was created,
no cluster or file in the repo was mutated, and no finding, score, or piece
of output text was invented. The only custom-built parts are title cards,
a typed-command animation of a real captured command/output pair, and
small summary cards; everything else is the real `report.html` and the
real embedded Console, navigated to and recorded live.

## What this is built from

All source evidence lives in [`evidence/`](evidence/) and is a curated
copy of files already produced and sanitized by the SEC-TRUST-002 live-EKS
run against `v1.0.0-rc.2` (see `docs/live-eks-release-smoke.md` at the repo
root) — copied from `live-eks-evidence/sanitized/`, not re-generated. Every
number, finding ID, and line of terminal output in this video (`BLOCKED`,
`75/100`, `ADDON-001`, `EKS-NG-002`, `WH-005`, the exact `failurePolicy:
Ignore` wording) is read verbatim from these files, not typed from memory:

- `scan-command.txt` / `scan-stdout.txt` — the real `kubepreflight scan`
  invocation and its real terminal output.
- `scan-findings.json` / `scan-report.html` — the real findings and the
  real, standalone report used for the report-format scene and for the
  Console's `?findings=` fixture.
- `upgrade-plan.json`, `plan-findings.json` — real `kubepreflight plan`
  output, used as the Console's `?plan=` fixture.
- `rollback-plan-assessment.json`, `rollback-assess-assessment.json` — real
  `kubepreflight rollback plan` / `rollback assess` output, used as the
  Console's `?rollback=` fixture and as the source for the closing
  compare/rollback summary card's numbers.
- `compare-current.json`, `compare-baseline.json`, `comparison.json`,
  `gate.json` — real `kubepreflight compare` output (binary vs. container
  parity), source for the same closing summary card.

## Directory layout

```
demo/v1-launch/
├── README.md              this file
├── storyboard.md           scene-by-scene timeline and shot list
├── captions.md              every on-screen caption/overlay string, verbatim
├── record-browser.mjs        Playwright recorder — the only recording tool
├── render.sh                 ffmpeg encode: MP4 (16:9), GIF, MP4 (1:1), poster PNG
├── verify.sh                  ffprobe + leak-scan checks against output/
├── assets/                    custom scene HTML (title cards, terminal, findings, summary)
├── evidence/                   curated real sanitized evidence (see above)
├── recordings/                  raw Playwright capture (gitignored, reproducible)
└── output/                       the 4 final deliverables
```

There is no separate `record-terminal.sh`. The terminal segment is not a
recorded real terminal session — it is `assets/02-terminal.html`, a typing
animation that renders the exact text of `evidence/scan-command.txt` and
`evidence/scan-stdout.txt` character-for-character. This was a deliberate
substitution: capturing a real terminal recording would have meant
re-running the CLI (out of scope — no new cluster, no mutation) or
splicing in a differently-formatted terminal capture from the original
live-EKS run, whereas typing out the *actual* captured text keeps the
video's terminal content byte-identical to real evidence while giving
full control over pacing and legibility at video resolution.

## How this was produced

1. **Scenes** (`assets/*.html`): self-contained HTML/CSS pages (fonts
   embedded as base64 so they render offline), each setting
   `window.sceneReady = true` when its entrance animation finishes and
   exposing `window.fadeOut()` for the recorder to call.
2. **Recording** (`record-browser.mjs`): a single continuous Playwright
   session navigates through the scenes, the real `report.html`, and the
   real Console (built `web/dist`, served locally, loaded with the
   `evidence/` files as fixtures via query-string URLs) and records one
   continuous video — not a multi-clip splice — so there are no seams to
   misalign. Scene durations are wall-clock scheduled against a fixed
   30.0s timeline (see `storyboard.md`).
3. **Encoding** (`render.sh`): the one raw capture
   (`recordings/raw-capture.webm`) is transcoded to all four final formats
   in `output/` — nothing is re-recorded per format.
4. **Verification** (`verify.sh`): re-runs the leak scan and the
   resolution/codec/duration/faststart checks below on demand.

### Reproducing this locally

Requires Node.js with `playwright` installed (not a project dependency —
this is a one-off recording tool) and a static file server able to serve
this directory plus the built Console (`web/dist`) at the same origin so
Console fixture `fetch()` calls stay same-origin. Then:

```sh
BASE_URL=http://localhost:8899 OUT_DIR=./recordings node record-browser.mjs
./render.sh
./verify.sh
```

## Verified properties of the four exports

| File | Resolution | Codec | Duration | Size |
|---|---|---|---|---|
| `kubepreflight-v1-launch-16x9.mp4` | 1920x1080 | H.264, yuv420p, faststart | 29.92s | 2.35 MB |
| `kubepreflight-v1-launch-1x1.mp4` | 1080x1080 (center crop) | H.264, yuv420p, faststart | 29.92s | 1.67 MB |
| `kubepreflight-v1-launch-16x9.gif` | 640x360, 8fps | — | 13.0s cut (3.0s-16.0s: terminal → findings → report) | 680 KB |
| `kubepreflight-v1-launch-poster.png` | 1920x1080 | — | still, t=15.0s | 447 KB |

`verify.sh` checks all of the above plus a faststart (`moov` before `mdat`)
byte-offset check on both MP4s, so a player can begin playback before the
full file downloads.

## Evidence provenance and normalization

`evidence/scan-command.txt` is whitespace-normalized (trailing whitespace
on its one line stripped) from the original sanitized SEC-TRUST-002
artifact, so this repository's `git diff --check` gate stays clean.
Command content and captured product output are otherwise byte-identical
to the source; nothing else in `evidence/` was touched. Original source
artifact, for provenance:

```
live-eks-evidence/sanitized/binary/scan/terminal/command.txt
sha256:87eb9647618637d41430423a2c4582bdc92ce8042b07a24245f1b3b1b3ad1241
```

## Sensitive-data checks

`verify.sh` greps `evidence/`, `assets/`, `record-browser.mjs`, and
`render.sh` for AWS ARNs, 12-digit account IDs, EC2-internal hostnames,
and `AKIA`-prefixed access keys — the same pattern set the repo's own
`scripts/live-eks/check-redaction.sh` uses. All evidence files were
already sanitized upstream (SEC-TRUST-002); this is a second, independent
pass scoped to exactly what ships in this demo. The evidence files
(`evidence/`) still carry the real disposable cluster name
(`kp-v1-rc-smoke`), per the same convention the sanitized evidence tree
already uses — that's an internal test-cluster identifier, not a secret,
and `evidence/scan-report.html` on disk is never modified.

The video itself, however, shows `redacted-eks-cluster` throughout,
consistently. This started as a fix to the terminal scene: an earlier
draft used `production` as a stand-in cluster name for on-screen
legibility, which read as implying the SEC-TRUST-002 verification ran
against a live production cluster. It didn't — it ran against a real,
disposable EKS cluster created and torn down for that verification only —
so the on-screen name was changed to something that can't be misread that
way, plus an explicit caption: "Real disposable EKS cluster — SEC-TRUST-002
verification run, name redacted." The real `report.html` scene (13.5s–16.0s)
initially still showed the real `kp-v1-rc-smoke` identifier in its visible
`CLUSTER` field, since that page is loaded live rather than typed by a
custom scene — inconsistent with the terminal fix, and a real (if
non-secret) cluster identifier visible in public-distribution media.
`record-browser.mjs`'s `redactClusterName()` now runs a DOM-only text
substitution on that page immediately after load, in the recording
browser only, replacing every visible occurrence of `kp-v1-rc-smoke` with
`redacted-eks-cluster` before capture begins. **The displayed cluster name
is cosmetically redacted for public distribution. Findings, score,
verdict, and remediation text are unchanged** — verified by comparing
extracted frames before/after: same `BLOCKED` verdict, `75/100` score, `1
blocker / 2 warnings / 3 info` counts, same category table.

## Known limitation: poster frame text

The poster (`output/kubepreflight-v1-launch-poster.png`, extracted from
the real `report.html` at t=15.0s) clearly shows "KubePreflight Scan
Report", the `NO-GO` badge, `BLOCKED`, and `75/100` — but does **not**
contain the literal strings "KubePreflight v1.0.0" or "Verified against
real EKS", since that copy lives in the video's title cards
(`01-title-open.html`, `07-title-close.html`), not on the real report page
itself. A frame from those title cards would show the version/verification
copy but not the BLOCKED/75/100 result. Rather than overlay fabricated
text on top of a real product screenshot for the thumbnail, this was left
as an honest trade-off — the report-page frame was chosen because it reads
as an authentic in-product screenshot, which matters more for a poster
than literal string coverage.

## Recommended distribution

- **Core repo**: the GIF only, committed at
  [`docs/assets/kubepreflight-v1-launch.gif`](../../docs/assets/kubepreflight-v1-launch.gif)
  (605 KB) — placed alongside the four other product GIFs already in
  `docs/assets/` rather than a new `docs/media/` directory, to keep one
  convention for README-embedded media. Embedding it into the top-level
  `README.md` is a separate follow-up step, not done as part of this PR.
  The MP4s and the poster PNG are **not** committed here — see
  `.gitignore` in this directory. A 1-2 MB binary blob per export doesn't
  belong in git history for a marketing asset that will be re-cut before
  every future release anyway.
- **kubepreflight.com (website repo, not this one)**: copy
  `output/kubepreflight-v1-launch-16x9.mp4` and
  `output/kubepreflight-v1-launch-poster.png` into the website repo's
  static/public media directory, self-hosted from static hosting/CDN (not
  YouTube-embedded, so there's no ad/recommendation chrome on a product
  marketing page), with the poster PNG as the `<video>` element's `poster`
  attribute for instant first-paint. Optionally also copy the 1:1 MP4 if
  the site wants a social/download variant.
- **README / GitHub**: the GIF committed above — GitHub renders GIFs
  inline in Markdown without a video player or click-to-play friction, and
  605 KB is well within normal README image budgets.
- **LinkedIn**: two options depending on the post. A 13-second, 1280×720
  LinkedIn-formatted teaser cut of the GIF also exists at
  `docs/assets/kubepreflight-linkedin-launch.mp4` — generated from the
  committed GIF (`ffmpeg`, H.264, no audio, faststart, 24fps, ~930 KB) for
  a quick launch/teaser post. **This file is intentionally left untracked
  — `git status` will show it, but it is not meant to be committed**;
  regenerate it locally with the command in this README's history (scale
  to 1280×720 lanczos + pad, `fps=24`) whenever the source GIF changes.
  For a fuller product-walkthrough post, use the 1:1 MP4 from `output/`
  (not committed either — export it locally and upload directly) — square
  is what LinkedIn's mobile feed crops to regardless of what's uploaded,
  so uploading square avoids an algorithmic re-crop cutting off the
  terminal window or Console sidebar.
