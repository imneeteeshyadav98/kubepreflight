# Storyboard — KubePreflight v1.0.0 launch demo

Total scripted timeline: 30.0s (encoded duration 29.92s — Playwright's own
teardown accounts for the ~80ms difference). Every scene's target duration
is enforced wall-clock in `record-browser.mjs`'s `playScene()` helper, not
by naive sequential waits, so entrance-animation timing variance never
compounds into overshoot.

| # | Time | Scene | Source | Real or custom |
|---|---|---|---|---|
| 1 | 0.0s – 3.0s | Opening title card: wordmark, "v1.0.0", "Kubernetes & EKS upgrade and rollback readiness", "Verified against real EKS", "Read-only — never upgrades or rolls back your cluster" | `assets/01-title-open.html` | Custom card, real version/positioning copy |
| 2 | 3.0s – 8.0s | Terminal window types the real scan command, then reveals the real captured output line-by-line: `Collected: …`, `Result: BLOCKED`, `Upgrade Readiness: BLOCKED — Score: 75/100 — Upgrade Continue: No` | `assets/02-terminal.html`, text sourced from `evidence/scan-command.txt` + `evidence/scan-stdout.txt` | Custom animation, real text |
| 3 | 8.0s – 12.0s | Three finding cards slide in: `ADDON-001` (blocker, kube-proxy below catalog minimum), `EKS-NG-002` (warning, limited rolling-update headroom), `WH-005` (warning, admission webhook scope — `failurePolicy: Ignore` today, flags the risk if that ever changes) | `assets/03-findings.html`, wording verified against `evidence/scan-findings.json` and `evidence/scan-stdout.txt` | Custom cards, real finding text |
| 4 | 12.0s – 13.5s | "One result, four formats" — terminal / findings.json / report.md / report.html | `assets/04-reports-overview.html` | Custom card |
| 5 | 13.5s – 16.0s | The real, sanitized `report.html` — with its visible `CLUSTER` value cosmetically redacted from `kp-v1-rc-smoke` to `redacted-eks-cluster` for public-distribution consistency with the terminal scene — plus a bottom-bar caption "CLI · JSON · Markdown · HTML" | `evidence/scan-report.html`, served live, redacted in-DOM at record time only | **Real page**, live navigation |
| 6 | 16.0s – 22.0s | The real embedded Console, loaded with real findings/plan/rollback fixtures via query string. Bottom-bar caption "Reviewable evidence, not a pass/fail guess." Clicks through Findings → Next Actions → Rollback tabs | Built `web/dist` Console, `evidence/*.json` as fixtures | **Real product UI**, live navigation, real clicks |
| 7 | 22.0s – 27.0s | Compare + rollback summary: `kubepreflight compare` (comparison gate: pass, 0 new blockers/warnings, binary vs. container agree) and `kubepreflight rollback plan/assess` (readiness: blocked, recommendation: do_not_proceed, confidence: high) | `assets/06-compare-rollback.html`, numbers sourced from `evidence/comparison.json`, `evidence/gate.json`, `evidence/rollback-plan-assessment.json`, `evidence/rollback-assess-assessment.json` | Custom card, real numbers |
| 8 | 27.0s – 30.0s | Closing title card: wordmark, "v1.0.0 · Read-only Kubernetes / EKS upgrade readiness · Verified against real EKS", links to kubepreflight.com and the GitHub repo, "Open source" | `assets/07-title-close.html` | Custom card |

## Why this order

Command → result → *why* (findings) → *how you'd consume that result*
(report formats, then the real report, then the real Console) → *what
happens next* (compare across binary/container, rollback posture) → close.
The structure mirrors how a first-time viewer would actually use the tool:
run it, see a verdict, understand why, then decide where to go read the
detail.

## Design constraints followed

- Every finding ID, score, and status string traces back to a real,
  already-sanitized evidence file — nothing is invented for pacing or
  drama.
- Scenes 5 and 6 are not screenshots or reconstructions — they are live
  navigations to the real `report.html` and the real built Console,
  recorded as part of the same continuous session as the custom cards, so
  there is no visual seam between "demo" and "product" footage.
- Caption overlays (scenes 5 and 6 only) are a solid, full-width bar
  pinned to the bottom screen edge — deliberately not a floating box —
  because a floating caption over the real report/Console pages collided
  visually with real content underneath it in an earlier cut (a table row,
  a sidebar note) and read as a rendering glitch rather than a caption.

## LinkedIn v2 teaser — a separate, standalone 15.8s recording

Not a re-cut of the 30s master above. The 13s GIF-derived teaser it
replaces opened directly on the terminal and ended directly on the report
— fine with a post caption for context, but it has none when watched
as a standalone video (LinkedIn's native player, a share, a download).
This recording adds a dedicated opening title and closing CTA card, both
**editorial overlays** — real-evidence beats in the middle are otherwise
unchanged. Recorded via `VARIANT=linkedin node record-browser.mjs`,
rendered via `render-linkedin.sh`.

| # | Time | Scene | Source | Real or custom |
|---|---|---|---|---|
| 1 | 0.0s – 1.3s | Opening title card: "KubePreflight v1.0.0" (most prominent line), "Kubernetes & EKS upgrade readiness", "Read-only · Verified against real EKS". Subtle opacity fade-in only (220ms), no other animation | `assets/08-linkedin-title-open.html` | Custom card, editorial overlay |
| 2 | 1.3s – 6.1s | Terminal: real scan command, real captured output, `redacted-eks-cluster`, `Result: BLOCKED`, `Score: 75/100` | `assets/02-terminal.html` (shared with the master) | Custom animation, real text |
| 3 | 6.1s – 9.9s | Same three finding cards as the master (`ADDON-001`, `EKS-NG-002`, `WH-005`) | `assets/03-findings.html` (shared) | Custom cards, real finding text |
| 4 | 9.9s – 11.3s | "One result, four formats" | `assets/04-reports-overview.html` (shared) | Custom card |
| 5 | 11.3s – 13.8s | The real `report.html`, cosmetically redacted in-DOM the same way as the master (see the table above), bottom-bar caption. Fades out via a generic body-opacity fade (the real page has no `window.fadeOut()` of its own) | `evidence/scan-report.html`, served live | **Real page**, live navigation |
| 6 | 13.8s – 15.8s | Closing CTA card: "Catch upgrade blockers before production changes.", `kubepreflight.com` (most prominent), "Open source · Read-only", smaller `github.com/imneeteeshyadav98/kubepreflight` | `assets/09-linkedin-title-close.html` | Custom card, editorial overlay |

Beat durations are compressed from the master (5.0s→4.8s terminal,
4.0s→3.8s findings, 1.5s→1.4s reports-overview) to make room for the
opening/closing cards inside a 15-16s standalone target without cutting
any beat entirely — verified readable at each compressed duration by
frame inspection, not assumed.

**Square (1:1) reframing**: a first attempt used a 1080×1080 center crop
of the 1920×1080 recording, matching the master's `render.sh` approach —
but the terminal window and report layout are ~1500px wide, wider than
the 1080px crop, so a center crop cut off the `CLUSTER` field, the
`NO-GO` badge, and the entire `VERDICT` column. Fixed by scaling the full
frame to fit within 1080 width and padding top/bottom (letterbox, color
matched to the scene background so the bars are invisible against the
dark cards) instead — no source pixels are cropped or stretched.
