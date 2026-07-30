# Console & Report UI/UX Audit

Date: 2026-07-30
Scope: read-only audit of the Console (`web/src`) and the static HTML/Markdown report (`internal/report/html.go`) at current `master`. No product code was changed by this audit.

## 1. Objective

Not "make it beautiful" and not "make it look like Grafana." The single question this audit answers:

> Can a DevOps/SRE engineer look at a scan and quickly, confidently understand whether the upgrade is blocked, why, what the evidence is, and what to do next?

Console and report are treated as two different surfaces with two different jobs, per this audit's own brief:

- **Console** — interactive investigation: readiness at a glance, blocker filtering, finding → evidence → remediation drill-down, before/after compare, partial-evidence detection, action-plan assembly.
- **Report (HTML/Markdown)** — decision and evidence artifact: shareable in a change review, explainable to a CAB/manager/customer, preserved offline, understandable when printed.

## 2. Method

Real evidence only — no mockups, no re-derivation from memory of the code:

- Full read of every Console component (`web/src/App.tsx`, all 14 files under `web/src/components/`), `web/src/styles.css` (510 lines), and `internal/report/html.go` (2,674 lines, template + embedded CSS + embedded JS).
- Real, headless-Chrome + Selenium screenshots of the actual embedded Console and `report.html`, served by the real `internal/reportserver` (via `cmd/consoledevserver`, the same harness `web/tests/browser_smoke.py` uses — not a stand-in static server), at 1440×1000 (desktop), 768×1024 (tablet), and 390×844 (mobile).
- Three real report fixtures, not hand-authored mockups:
  1. The project's own committed synthetic fixture (`writeSyntheticFixture`, `cmd/consoledevserver/main.go`) — `BLOCKED`, 6 findings across all 3 severities and 4 confidence tiers.
  2. A real EKS scan from this repository's own certification evidence (`docs/certification/post-232-real-environments/03-full-real-eks/00-full-access-audit-only/findings.json`) — `PASSED_WITH_WARNINGS`, real EKS add-ons/node groups/rule-execution data — paired with a real `upgrade-plan.json` (`demo/v1-launch/evidence/`) and a real `rollback-assessment.json` (`docs/certification/.../05-rollback/case-b-fresh-matching/`) to exercise the Upgrade Planner and Rollback tabs.
  3. A real reduced-RBAC scan from the same certification evidence (`01-reduced-rbac/profile-a/findings.json`) — `INCOMPLETE`, **0 findings, readiness score 100**, `coverage.kubernetes.status: partial` — the exact "does insufficient evidence look like a pass" scenario this audit was commissioned to check.
- Computed-DOM accessibility snapshots (landmarks, heading order, focus-outline computed style, document-level horizontal-overflow) captured via Selenium JS execution against every fixture above.
- Contrast values were **not** independently re-measured pixel-by-pixel; `styles.css` already carries multiple dated, reasoned inline comments recording specific WCAG AA contrast fixes (see §6, Strengths) — this audit spot-checked a sample of those claims by inspecting the cited hex values against WCAG's contrast formula and found them consistent, but did not exhaustively re-audit every color pair in the file.
- One methodology caveat: the Upgrade Planner and Rollback tab screenshots (fixture 2) were produced by pairing two **unrelated** real documents (a same-version EKS scan with an unrelated multi-hop plan fixture) purely to get both tabs rendering in one screenshot pass, via `cmd/consoledevserver`'s `--dir` mode, which unconditionally serves `upgrade-plan.json`/`rollback-assessment.json` if present. `internal/reportserver/server.go` gates these routes behind `Config.ServePlan`/`Config.ServeRollback`, which the real CLI presumably sets only when the matching command actually ran (`server.go:82-94`) — this audit did not trace `internal/cli` to confirm that gating, so **no finding below claims that a stale plan/rollback tab can appear next to an unrelated scan in real usage**. That would need a separate, source-level pass at `internal/cli` if the team wants it closed out.

## 3. Personas

| Persona | Role | Primary surface | Primary question |
|---|---|---|---|
| **Platform Engineer** | Executes the upgrade and the remediation | Console: Findings → Evidence → Remediation, Next Actions, Copy-to-clipboard | "What exactly do I run, on what, in what order?" |
| **SRE / Reviewer** | Validates evidence, can veto the go/no-go | Console: Rule Execution Coverage, Evidence appendix, Compare, Rollback | "Is this evidence actually complete, and can we get back out if it goes wrong?" |
| **Engineering Manager / Change Approver** | Approves the change window without going deep technically | report.html (often as a PDF/print artifact, or the shared Console link) | "Blocked or not, and what's the one-line reason?" |

## 4. Journeys walked

1. **J1 — Platform engineer, post-scan triage.** Opens the just-printed Console URL → reads the decision strip → `Start here` list on Summary → clicks into a blocker's detail → copies the remediation command → checks `Next Actions` for the full ordered list. Works well; see §6.
2. **J2 — SRE reviewer, pre-CAB evidence check.** Opens `report.html` (or a printed/PDF copy) ahead of a change review → checks the decision badge → opens Evidence appendix to see what was actually evaluated → checks Rollback tab for the fallback plan. Surfaces the most serious finding in this audit (§7, F1 — raw rollback reason codes).
3. **J3 — Manager/approver, ten-second read.** Opens the Console link from a ticket → reads only the decision strip + metrics row, does not open any tab. This is the scenario the "ten-second comprehension" and "insufficient evidence must not look like a pass" requirements are really about; see §6 and §7 (F2).

## 5. Ten-second comprehension: verdict

**Meets the bar.** Confirmed against all three real report states above, not just the happy path:

| State | Decision chip | Status badge | Why-line | Screenshot |
|---|---|---|---|---|
| Real EKS, blockers present (synthetic fixture) | `NO-GO` (red outline) | `BLOCKED` | "4 blockers found — fix required before the change window." | `console-report-ui-ux-screenshots/01-console-summary-blocked-desktop.png` |
| Real EKS, warnings only | `REVIEW` (amber outline) | `PASSED_WITH_WARNINGS` | "3 warnings and 1 operator decision found — review before proceeding." | `.../14-console-summary-real-eks-warnings-desktop.png` |
| Real reduced-RBAC, **0 findings**, score 100 | `NO-GO` (red outline) | `INCOMPLETE` (amber) | "Assessment incomplete — evidence collection was incomplete. Resolve coverage errors and rerun." | `.../17-console-summary-incomplete-zero-findings-desktop.png` |

The third row is the important one. A report with zero blockers, zero warnings, and a 100/100 readiness score still renders `NO-GO` / `INCOMPLETE`, not a green pass — confirmed on a real, not synthetic, evidence-starved scan. This directly answers the concern the audit brief opened with. See §6 for why this doesn't fully carry down to the metrics row itself.

## 6. Strengths — do not change without a specific reason

1. **Decision hierarchy is real, not just styled.** `NO-GO`/`REVIEW`/`GO` (decision) is a distinct visual axis from `BLOCKED`/`PASSED_WITH_WARNINGS`/`INCOMPLETE`/`CLEAN` (result) — two badges, not one overloaded color. `DecisionHero.tsx:9-15`, mirrored in `html.go`'s `decisionLabel`/`resultClass`.
2. **Insufficient evidence does not read as a pass**, confirmed on real data (§5, row 3) — the exact requirement in the brief's §2.
3. **Compare tab's `not_re_evaluated` handling is correct and visible**, not just correct in the backend: distinct bucket from `Resolved`, an inline explanation (`NOT_RE_EVALUATED_EXPLANATION`), and the gate decision (`neutral`/`INSUFFICIENT_EVIDENCE`) is shown even when the raw readiness-score delta is positive (`ComparisonTab.tsx:128-129`, confirmed live in `05-console-compare-desktop.png`: score `88 → 47`, still correctly labeled, not glossed over). This matches, field for field, what was independently verified against a real cluster in this repository's own `docs/certification/post-232-real-environments/04-compare/report.md`.
4. **Zero horizontal page overflow** at 390px/768px/1440px, across every tab on both surfaces, confirmed via computed-DOM `scrollWidth` checks in this audit's own capture pass — consistent with the permanent regression guard already committed in `web/tests/browser_smoke.py:420-514`. Don't remove that guard.
5. **Print/PDF is deliberately handled**, not an afterthought: `beforeprint`/`afterprint` listeners (`html.go:2519-2537`) force-expand every hidden tab panel and every collapsed finding `<details>` row before the OS print dialog opens, then restore the compact on-screen state after. Confirmed by source read; this directly satisfies the report's own "must stay understandable on paper" requirement. Easy to accidentally break in a future refactor — flag any PR touching `.tab-panel.hidden` or `.finding-row[open]` handling for review against this.
6. **Raw AWS identifiers are deliberately restrained**, not just redacted server-side: the cluster ARN is never shown inline — only a short display name plus an explicit "Copy ARN" button (`DecisionHero.tsx:52-76`, mirrored in `html.go`'s `meta-chip-copy-source` pattern). Consistent with this project's broader redaction posture.
7. **Mobile Findings flow is a real list→detail pattern**, not a squeezed two-pane layout: list and detail each get the full viewport, with an explicit `← Back to list` control (`FindingDetail.tsx:65-69`, `styles.css:479-484`). Confirmed live in `07/08-console-findings-*-mobile.png`.
8. **Contrast fixes are a maintained practice, not a one-off.** `styles.css` carries at least 4 separate, dated, reasoned comments recording specific WCAG AA contrast corrections (e.g. lines 252-256, 273-277, 302-305, 306-313) — evidence the team already treats this as an ongoing discipline. Any new color pairing added to this file should be checked the same way, not exempted because "the rest of the file already passes."

## 7. Findings

Ordered by severity. Each maps to a UX backlog item in `console-report-ux-backlog.md`.

---

### F1 — Rollback tab shows raw machine reason codes with no human-readable translation

**Severity: P1** · **Component: Console, `RollbackReadinessTab.tsx`** · **Backlog: UX-002, UX-006**

**Evidence:** `console-report-ui-ux-screenshots/16-console-rollback-desktop.png`, a real `rollback assess` output (`docs/certification/post-232-real-environments/05-rollback/case-b-fresh-matching/rollback-assessment.json`). The Reason Codes panel renders verbatim:

> `EKS_UPGRADE_HISTORY_UNAVAILABLE, END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED, EKS_FEATURE_COMPATIBILITY_UNVERIFIED, MANAGED_NODEGROUP_ROLLBACK_REQUIRED, PDB_DISRUPTION_CONSTRAINTS, ROLLBACK_EVIDENCE_TARGET_MISMATCH`

and separately, `Eligibility: unavailable` / `Readiness: insufficient_evidence` render the raw enum values directly.

**Root cause, confirmed at the source:** `web/src/lib/rollback-schema.ts` types `RollbackReasonCode` as a bare `string` (line 1) with no companion label map, unlike `findings-schema.ts`, which has `ruleExecutionStateLabel`, `decisionLabel`, `upgradeGateLabel`, `priorityPillClass`, etc., or `html.go`, which has `ruleTitle()`/`ruleWhy()` for the equivalent job elsewhere in the same product. `RollbackReadinessTab.tsx:14-16` (`reasonList`) does a plain `.join(", ")` over the raw strings; lines 46 and 50 interpolate `assessment.eligibility.status` / `assessment.readiness.status` directly.

**Why it matters:** this is the audit brief's own "rule ID alone must not be the primary headline" principle, violated on the single surface (Rollback) where the SRE/Reviewer persona needs the *fastest* possible read — deciding whether it's safe to back out. Every other finding surface in this product (Findings tab, Next Actions, report.html's finding cards) leads with a human sentence and treats the machine ID as secondary. Rollback is the one place that inverts it.

**Proposed change:** add a `rollbackReasonCodeLabel(code: RollbackReasonCode): string` map (mirroring the existing `*Label` helpers in `findings-schema.ts`) covering at minimum the reason codes already emitted by `internal/rollback` (grep `internal/rollback` for the authoritative enum list), and humanize `eligibility.status`/`readiness.status` the same way `report.result`/`upgradeReadiness.verdict` already are elsewhere. Keep the raw code visible too (in a `<code>` or `title=` attribute) for anyone who wants to grep logs by it — don't remove the machine-readable value, add the human one in front of it.

**Acceptance criteria:**
- Every reason code currently emitted by `internal/rollback` has a human-readable label; an unmapped future code falls back to the raw string (never throws, never renders blank).
- `eligibility.status` and `readiness.status` render a human label with the raw enum available as a tooltip or secondary text.
- No change to `internal/rollback`'s actual decision logic, reason-code values, or JSON schema — this is a presentation-only fix.

---

### F2 — A zero-count metrics row doesn't itself signal "not evaluated" vs. "genuinely clean"

**Severity: P2** · **Component: Console `MetricsRow.tsx`, report.html metric cards** · **Backlog: UX-001, UX-002**

**Evidence:** compare `console-report-ui-ux-screenshots/17-console-summary-incomplete-zero-findings-desktop.png` (INCOMPLETE, 0/0/0/0) against a genuinely clean report — both would render an identical `Blockers 0 / Warnings 0 / Operator decisions 0 / Info 0` row. The top-level decision chip (`NO-GO`/`INCOMPLETE`) does correctly disambiguate the two — this is not a false-pass at the primary decision level, and §6's finding stands — but the metrics row itself carries no local signal.

**Root cause, confirmed at the source:**
- `MetricsRow.tsx` (Console) renders only a label and a raw count (`report.summary.blockers`) — no caption, no coverage-awareness, no different treatment for a report where `coverage.kubernetes.status !== "complete"`.
- `html.go:1903-1906` (static report) does have a caption, but the `{{else}}` branch is a fixed string — `"No blockers found"` — regardless of *why* the count is zero. A report with `coverage.aws.status: partial` and a report with genuinely no AWS issues render the identical caption.

**Why it matters:** this is specifically the scenario the audit brief opened with — "Insufficient evidence ko green/pass jaisa nahi lagna chahiye." The primary decision surface already handles it (§6); this finding is about the secondary surface a screenshot crop or a quick glance is most likely to isolate. A metrics row pasted into a Slack thread without the decision strip above it currently reads as unambiguously clean.

**Proposed change:** when the report's overall coverage status is not `complete` (`report.coverage.*.status` in Console; the equivalent Go-side check already exists for the score-qualification banner further down the same tab, see `SummaryTab.tsx:96-98`), give the metrics row a visible qualifier — e.g. a small "Coverage incomplete" tag on the row itself, not just further down the page. For `html.go`, replace the fixed `"No blockers found"` caption with a coverage-aware one reusing the same `OverallCoverage` classification the score-qualification block already computes (`toHTMLScoreQualification`, `html.go:308`).

**Acceptance criteria:**
- A report with `coverage.*.status: partial` and zero findings shows a visible coverage qualifier directly on/adjacent to the metrics row, not only in a banner further down the tab.
- A genuinely clean, fully-evaluated report's metrics row is visually unchanged from today.
- No change to `report.summary.*` computation — presentation only.

---

### F3 — Console's metric cards are static; report.html's equivalent cards are clickable shortcuts

**Severity: P2** · **Component: Console `MetricsRow.tsx` vs. `html.go` summary-grid** · **Backlog: UX-001**

**Evidence:** `html.go:1903-1906` renders each non-zero metric as `<button data-goto-severity="...">`, jumping straight to the filtered Findings tab. `MetricsRow.tsx` renders every metric as a plain, non-interactive `<article>` — confirmed by source read (no `onClick`, no `href`, no `data-goto-*` anywhere in the component) and by screenshot (no hover/focus affordance in `01-console-summary-blocked-desktop.png`).

**Why it matters:** the project's own convention, stated explicitly in a `styles.css` comment (line 295-297), is that the Console mirrors the static report's visual language "so the two surfaces read as the same product." This is one concrete place they diverge in *capability*, not just style — and the direction is backwards: report.html (the offline artifact) has the shortcut; Console (the interactive investigation surface, per this audit's own framing in §1) doesn't.

**Proposed change:** make `MetricsRow`'s cards behave like `TopRisks`' cards already do — clickable where the count is non-zero, calling the same `jumpToRule`-style pattern already used elsewhere in `App.tsx` (adapt to filter by severity instead of rule ID), with `aria-disabled` and no click handler when the count is zero (matching `html.go`'s own `aria-disabled="true"` branch).

**Acceptance criteria:**
- Clicking a non-zero metric card switches to the Findings tab pre-filtered to that severity.
- Zero-count cards remain non-interactive (`aria-disabled`, no pointer cursor).
- Existing `id="metric-blockers"` etc. selectors (used by `web/tests/browser_smoke.py`) are preserved.

---

### F4 — Mobile tab bar gives no visual cue that more tabs exist beyond what's visible

**Severity: P2** · **Component: Console `.tab-nav` at ≤720px** · **Backlog: UX-006**

**Evidence:** `console-report-ui-ux-screenshots/07-console-findings-list-mobile.png` — at 390px, roughly 3 of 6-7 tabs are visible (`Summary`, `Findings`, the start of `Next Actio[ns]`), with a thin native scrollbar as the only indicator that `Evidence`, `Planner`, `Rollback`, and `Compare` exist further right. `styles.css:461` (`.tab-nav { overflow-x: auto; flex-wrap: nowrap; }`) confirms this is a plain scrolling container with no fade, gradient, or chevron affordance.

**Why it matters:** a first-time mobile user reviewing a blocked upgrade from their phone (a realistic on-call scenario) may not discover the Evidence or Compare tabs exist at all.

**Proposed change:** add a lightweight edge-fade (a `linear-gradient` mask on `.tab-nav`'s container, hidden once scrolled to the end) or a small trailing chevron icon that disappears at max scroll. No JS state machine needed — a pure-CSS `scroll-timeline`/mask approach or a simple `onscroll` class toggle both work; either is a small, isolated change.

**Acceptance criteria:**
- At 390px width, a visual cue is present whenever the tab strip has unscrolled content in either direction.
- No change to tab order, tab count, or the existing keyboard `ArrowLeft`/`ArrowRight` navigation (`Tabs.tsx:43-49`).

---

### F5 — Two `<h1>` elements per page

**Severity: P3** · **Component: Console `Header.tsx` + `DecisionHero.tsx`; `html.go` banner** · **Backlog: UX-006**

**Evidence:** confirmed via this audit's computed-DOM heading snapshot on every fixture — e.g. `console-summary-blocked-desktop`: `[{H1: "KubePreflight Console"}, {H1: "synthetic-fixture"}, {H2: "Upgrade Readiness"}, ...]`. `Header.tsx:14` and `DecisionHero.tsx:56/60` both emit `<h1>`.

**Why it matters:** not a WCAG failure by itself, but a screen-reader user navigating by heading level encounters two "page-title-level" headings, which reads as slightly disorienting. Minor.

**Proposed change:** demote the product-chrome heading (`Header.tsx`'s "KubePreflight Console") to a non-heading element or `<p>` styled the same — the cluster-name heading in `DecisionHero` is the one that should own `<h1>`, since it's the actual subject of the page.

**Acceptance criteria:** exactly one `<h1>` per rendered page state; visual appearance unchanged (this is a semantic-only tag change).

---

### F6 — Focus-visible relies entirely on the unstyled browser-default outline

**Severity: P3** · **Component: Console, global** · **Backlog: UX-006**

**Evidence:** this audit's computed-style capture on 5 sampled interactive elements per page consistently shows `outline: auto` (browser default) at the default width, with no `box-shadow` or custom ring — the only unstyled interactive-state in an otherwise fully art-directed UI (every pill, chip, badge, and card has deliberate custom styling).

**Why it matters:** functionally compliant with WCAG 2.2's focus-visible requirements today — this is a polish item, not a defect. Flagged because the SRE/Reviewer persona's evidence walk (filter → row → detail → copy) is realistically keyboard-heavy, and a custom, higher-contrast focus ring matching the product's navy/mint palette would make that path easier to track visually, especially during screen-sharing in a review meeting.

**Proposed change:** a single `:focus-visible { outline: 2px solid var(--navy); outline-offset: 2px; }` (or similar) rule in `styles.css`, scoped broadly enough to cover buttons/links/inputs/tab rows without fighting the existing `.chip input`-hidden-checkbox pattern (verify that pattern specifically, since its input is visually hidden by design).

**Acceptance criteria:** every interactive element still has a visible focus indicator meeting WCAG 2.2 SC 2.4.11/2.4.7 contrast requirements against both the light (`--surface`/`--paper`) and dark (`--navy`) backgrounds used across the app; no regression to the existing hidden-checkbox chip pattern.

---

### F7 — Dense multi-column tables (EKS node groups, up to 10 columns) have no responsive column strategy

**Severity: P3** · **Component: Console `SummaryTab.tsx` EKS panels, `RuleExecutionCoverage.tsx`** · **Backlog: UX-001**

**Evidence:** `SummaryTab.tsx:257` — the EKS managed node group table has 10 columns (`Node group, Status, Version, Release, AMI type, Capacity, Desired/min/max, Update config, Health, Readiness`). Currently masked by `.table-wrap { overflow: auto }` (confirmed no page-level overflow at any tested width), so this is not a broken-layout bug today — but there's no column-priority or responsive-collapse strategy if this table grows (e.g. more EKS metadata fields added later).

**Proposed change:** no immediate action needed. Worth a design pass (column priority, or a card layout below ~900px matching the `.findings-list-table`'s own responsive pattern) the next time EKS node group or rule-execution data gains columns.

**Acceptance criteria:** none for now — this is a "watch, don't fix yet" item, included for completeness rather than urgency.

---

## 8. Areas explicitly out of scope / do not touch as part of any UX-00x fix

- Rule engine, evidence-dependency logic, redaction, compare semantics, rollback decision logic, exit codes, JSON schema — all of §6 confirms these are working correctly; none of the findings above call for touching `internal/rules`, `internal/redact`, `internal/comparison`, or `internal/rollback`'s actual decision code. Every fix here is presentation-layer only (React components, `styles.css`, or `html.go`'s template/CSS/JS, never its `findings.Report` input).
- `web/tests/browser_smoke.py`'s existing assertions (element IDs, `data-*` attributes used by the test) — any UX-00x fix must keep those selectors working or update the test in the same PR, not silently break it.
- The Console/report visual language itself (navy/mint/paper palette, Georgia-serif headings, monospace pills) — no finding above asks for a redesign of the visual system; F1-F4 are information-architecture and interaction gaps, not aesthetic complaints.

## 9. Recommended sequencing

Per this audit's own brief: audit first, then prioritized fixes, not a redesign. Suggested order, each as its own PR:

1. F1 (Rollback reason-code labels) — P1, the one place the product's own "human-readable first" rule is broken.
2. F2 + F3 (metrics row coverage-awareness + click-to-filter parity) — same components, reasonable to bundle.
3. F4 (mobile tab affordance) — small, isolated CSS/JS change.
4. F5 + F6 (heading semantics, focus ring) — trivial, low-risk, can ride along with any of the above.
5. F7 — no action now; revisit when EKS table columns grow.

Each PR should include a `web/tests/browser_smoke.py` or Vitest assertion for the specific behavior it fixes, consistent with this codebase's existing practice of pairing every UI behavior with a real-browser regression check.
