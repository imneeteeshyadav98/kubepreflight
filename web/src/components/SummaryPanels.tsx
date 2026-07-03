import type { Report } from "../lib/findings-schema";

interface SummaryPanelsProps {
  report: Report;
}

export default function SummaryPanels({ report }: SummaryPanelsProps) {
  const notes = [...report.assumptions];
  if (report.namespaceAllowlist.length) notes.push(`Namespace allowlist: ${report.namespaceAllowlist.join(", ")}`);

  const confidence = new Map<string, number>();
  report.findings.forEach((finding) => confidence.set(finding.confidence, (confidence.get(finding.confidence) || 0) + 1));

  return (
    <>
      {notes.length > 0 && (
        <section className="assumptions" id="assumptions">
          <strong>Scope notes</strong>
          <ul id="assumption-list">
            {notes.map((note, index) => (
              <li key={index}>{note}</li>
            ))}
          </ul>
        </section>
      )}

      <section className="summary-grid summary-grid-3" aria-label="Scan summary">
        <article className="metric metric-blocker">
          <span>Blockers</span>
          <strong id="metric-blockers">{report.summary.blockers}</strong>
          <small>must resolve</small>
        </article>
        <article className="metric metric-warning">
          <span>Warnings</span>
          <strong id="metric-warnings">{report.summary.warnings}</strong>
          <small>review before change</small>
        </article>
        <article className="metric">
          <span>Information</span>
          <strong id="metric-infos">{report.summary.infos}</strong>
          <small>context only</small>
        </article>
      </section>

      <section className="confidence-panel">
        <div>
          <p className="eyebrow">Evidence posture</p>
          <h2>Confidence mix</h2>
        </div>
        <div className="confidence-list" id="confidence-list">
          {[...confidence.entries()].map(([name, count]) => (
            <div className="confidence-stat" key={name}>
              <b>{count}</b>
              <span>{name}</span>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}
