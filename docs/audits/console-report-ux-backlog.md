# Console & Report UX Backlog

Source: `console-report-ui-ux-audit.md` (2026-07-30). Each item below is presentation-layer only — none require changes to `internal/rules`, `internal/redact`, `internal/comparison`, or `internal/rollback`'s decision logic. See the audit doc for full evidence, screenshots, and acceptance criteria per finding.

| ID | Title | Severity | Findings | Component(s) |
|---|---|---|---|---|
| UX-001 | Upgrade decision hierarchy | P2 | F2, F3, F7 | `MetricsRow.tsx`, `html.go` summary-grid, `SummaryTab.tsx` EKS panels |
| UX-002 | Evidence coverage visibility | P1/P2 | F1, F2 | `RollbackReadinessTab.tsx`, `rollback-schema.ts`, `MetricsRow.tsx`, `html.go` |
| UX-003 | Finding detail information architecture | — | (no open findings; §6 confirms this is already correct — five-question structure, human message before rule ID, priority/evidence/remediation all present) | `FindingDetail.tsx`, `html.go` finding cards |
| UX-004 | Compare workflow clarity | — | (no open findings; §6 confirms `not_re_evaluated`/gate-neutral handling is correct and visible) | `ComparisonTab.tsx` |
| UX-005 | Report sharing and print readability | — | (no open findings; §6 confirms `beforeprint`/`afterprint` panel-expansion already solves this) | `html.go` print handling |
| UX-006 | Keyboard and screen-reader accessibility | P1/P2/P3 | F1 (rollback labels), F4 (mobile tab affordance), F5 (duplicate h1), F6 (focus ring) | `RollbackReadinessTab.tsx`, `.tab-nav`, `Header.tsx`/`DecisionHero.tsx`, global `styles.css` |

## Notes

- UX-003, UX-004, and UX-005 are listed because the audit brief named them as required review categories — they were audited and found to already meet the bar (see audit doc §6, items 3, 5, 7 for report-print and compare; finding-detail structure was reviewed against the "five questions" framework in `FindingDetail.tsx` and found to already lead with a human message, then priority/evidence/remediation, with rule ID as a secondary eyebrow label). No backlog work is proposed against them. Re-open only if new evidence surfaces.
- Severity key: P0 = blocks a correct go/no-go read, P1 = actively misleading or contradicts the product's own stated behavior, P2 = real gap with a clear, bounded fix, P3 = polish, do opportunistically.
- Recommended sequencing (PR-sized, in order): F1 → F2+F3 → F4 → F5+F6 → F7 (watch-only, no fix scheduled). See audit doc §9.
