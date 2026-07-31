import {
  evaluationCoverageStatus,
  evaluationCoverageStatusLabel,
  overallCoverageStatus,
  ruleExecutionCoverageSummary,
  type Report,
  type Severity,
} from "../lib/findings-schema";
import type { Filters } from "../types";

interface MetricsRowProps {
  report: Report;
  filters: Filters;
  onFilterBySeverity: (severity: Severity) => void;
}

// isActive mirrors what clicking a card would set: filters.severities pared
// down to exactly this one severity, with every other dimension cleared —
// the same equality check filterBySeverity (App.tsx) uses to decide whether
// a second click on the same card should toggle the filter back off.
function isActive(filters: Filters, severity: Severity): boolean {
  return (
    filters.severities.length === 1 &&
    filters.severities[0] === severity &&
    filters.search === "" &&
    filters.confidence === "" &&
    filters.namespace === ""
  );
}

// A metric card: a real <button> (click-to-filter, matches report.html's
// existing data-goto-severity cards) when its count is non-zero, or a
// non-interactive <article aria-disabled="true"> when it's zero -- same
// split report.html already uses, so "zero" never invites a click that goes
// nowhere. Caption text is the exact wording report.html already uses for
// the equivalent card, so the two surfaces never drift into two different
// vocabularies for the same state.
function MetricCard({
  id,
  className,
  label,
  count,
  activeCaption,
  zeroCaption,
  ariaLabel,
  severity,
  active,
  coverageComplete,
  onFilterBySeverity,
}: {
  id: string;
  className: string;
  label: string;
  count: number;
  activeCaption: string;
  zeroCaption: string;
  ariaLabel: string;
  severity: Severity;
  active: boolean;
  coverageComplete: boolean;
  onFilterBySeverity: (severity: Severity) => void;
}) {
  if (count === 0) {
    // "No blockers found" (etc.) is only ever shown once coverage is
    // complete -- the exact claim this card must never make when evidence
    // was partial/unavailable is "found none," full stop. The shared
    // metrics-coverage-note above already carries the fuller explanation;
    // this stays a short, un-overloaded qualifier at the card itself.
    const caption = coverageComplete ? zeroCaption : "Not evaluated";
    return (
      <article className={`metric ${className}`} aria-disabled="true">
        <span>{label}</span>
        <strong id={id}>{count}</strong>
        <small>{caption}</small>
      </article>
    );
  }
  return (
    <button
      type="button"
      className={`metric ${className} metric-button`}
      data-goto-severity={severity}
      aria-label={ariaLabel}
      aria-pressed={active}
      onClick={() => onFilterBySeverity(severity)}
    >
      <span>{label}</span>
      <strong id={id}>{count}</strong>
      <small>{activeCaption}</small>
    </button>
  );
}

// Part of the always-visible chrome (header strip + this row + tabs, see
// App.tsx) — fixed height, one row, never grows regardless of finding
// count, and rendered on every tab, not just Summary, so its own coverage
// note (below) is the one place that context is guaranteed visible
// regardless of which tab an operator happens to be on.
export default function MetricsRow({ report, filters, onFilterBySeverity }: MetricsRowProps) {
  // Reuses the exact same combined coverage classification the Summary
  // tab's "Coverage: Partial" advisory already computes (findings-schema.ts
  // mirrors internal/report/evaluation_coverage.go's OverallCoverage) —
  // deliberately not a second, parallel status model. A zero count here
  // means "none detected in the evidence that was actually evaluated," not
  // "no risk exists," and that distinction only matters when coverage isn't
  // complete -- this note is omitted entirely for a fully-evaluated report,
  // matching how the Summary tab's own advisory already stays silent then.
  const coverageStatus = overallCoverageStatus(evaluationCoverageStatus(ruleExecutionCoverageSummary(report)), report.coverage);

  return (
    <>
      {coverageStatus !== "complete" && (
        <section className="assumptions" role="status" id="metrics-coverage-note">
          <strong>{evaluationCoverageStatusLabel(coverageStatus)} evidence.</strong> The counts below reflect only what could be
          evaluated — a zero here means none was detected in that evidence, not that no risk exists.
        </section>
      )}
      <section className="summary-grid" aria-label="Scan summary">
        <MetricCard
          id="metric-blockers"
          className="metric-blocker"
          label="Blockers"
          count={report.summary.blockers}
          activeCaption="View blocker findings"
          zeroCaption="No blockers found"
          ariaLabel="View blocker findings"
          severity="Blocker"
          active={isActive(filters, "Blocker")}
          coverageComplete={coverageStatus === "complete"}
          onFilterBySeverity={onFilterBySeverity}
        />
        <MetricCard
          id="metric-operator-decisions"
          className="metric-warning"
          label="Operator decisions"
          count={report.summary.operatorDecisions ?? 0}
          activeCaption="Review before proceeding"
          zeroCaption="No operator decisions"
          ariaLabel="View operator decision findings"
          severity="Warning"
          active={isActive(filters, "Warning")}
          coverageComplete={coverageStatus === "complete"}
          onFilterBySeverity={onFilterBySeverity}
        />
        <MetricCard
          id="metric-warnings"
          className="metric-warning"
          label="Warnings"
          count={report.summary.warnings}
          activeCaption="View warning findings"
          zeroCaption="No warnings found"
          ariaLabel="View warning findings"
          severity="Warning"
          active={isActive(filters, "Warning")}
          coverageComplete={coverageStatus === "complete"}
          onFilterBySeverity={onFilterBySeverity}
        />
        <MetricCard
          id="metric-infos"
          className="metric-info"
          label="Info"
          count={report.summary.infos}
          activeCaption="View info findings"
          zeroCaption="No info findings"
          ariaLabel="View info findings"
          severity="Info"
          active={isActive(filters, "Info")}
          coverageComplete={coverageStatus === "complete"}
          onFilterBySeverity={onFilterBySeverity}
        />
      </section>
    </>
  );
}
