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
