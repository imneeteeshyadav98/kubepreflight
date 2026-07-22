import type { Report } from "../lib/findings-schema";

interface MetricsRowProps {
  report: Report;
}

// Part of the always-visible chrome (header strip + this row + tabs, see
// App.tsx) — fixed height, one row, never grows regardless of finding
// count.
export default function MetricsRow({ report }: MetricsRowProps) {
  return (
    <section className="summary-grid" aria-label="Scan summary">
      <article className="metric metric-blocker">
        <span>Blockers</span>
        <strong id="metric-blockers">{report.summary.blockers}</strong>
      </article>
      <article className="metric metric-warning">
        <span>Warnings</span>
        <strong id="metric-warnings">{report.summary.warnings}</strong>
      </article>
      <article className="metric metric-warning">
        <span>Operator decisions</span>
        <strong id="metric-operator-decisions">{report.summary.operatorDecisions ?? 0}</strong>
      </article>
      <article className="metric">
        <span>Info</span>
        <strong id="metric-infos">{report.summary.infos}</strong>
      </article>
    </section>
  );
}
