import { firstSentence, type Finding, type Report } from "../lib/findings-schema";
import TopRisks from "./TopRisks";
import { buildActionGroups } from "../lib/actions";

interface SummaryTabProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
  onViewAllActions: () => void;
}

// The Summary tab is a preview, not a full listing — top 3 risks and top 3
// next actions only, so switching to this tab never becomes a long scroll.
// Full lists live in their own tabs (Findings / Next Actions).
export default function SummaryTab({ report, onOpenFinding, onViewAllActions }: SummaryTabProps) {
  const notes = [...report.assumptions];
  if (report.namespaceAllowlist.length) notes.push(`Namespace allowlist: ${report.namespaceAllowlist.join(", ")}`);

  const confidence = new Map<string, number>();
  report.findings.forEach((finding) => confidence.set(finding.confidence, (confidence.get(finding.confidence) || 0) + 1));

	const actionGroups = buildActionGroups(report.findings);
	const topActions = actionGroups.slice(0, 3);

  return (
    <div className="tab-panel summary-tab">
	  {Object.entries(report.coverage).some(([, coverage]) => coverage.status === "partial") && (
		<section className="assumptions" role="alert">
		  <strong>Assessment incomplete</strong>
		  <p>Some evidence could not be collected. Findings shown remain actionable, but absence of findings is not proof of readiness.</p>
		  <ul>{Object.entries(report.coverage).flatMap(([plane, coverage]) => coverage.status === "partial" ? coverage.errors.map((error: string) => <li key={`${plane}-${error}`}>{plane}: {error}</li>) : [])}</ul>
		</section>
	  )}
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

      <TopRisks report={report} onOpenFinding={onOpenFinding} />

      {topActions.length > 0 && (
        <section className="preview-actions" aria-label="Top next actions">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Change plan preview</p>
              <h2>Top next actions</h2>
            </div>
            <button className="text-button" onClick={onViewAllActions}>
			  View all ({actionGroups.length})
            </button>
          </div>
          <ul className="preview-action-list">
			{topActions.map((group) => (
			  <li key={group.primary.fingerprint} role="button" tabIndex={0} onClick={() => onOpenFinding(group.primary)} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onOpenFinding(group.primary); } }}>
				<span className={`severity-pill ${group.severity.toLowerCase()}`}>{group.severity}</span>
				<strong>{group.resourceLabel}</strong>
				<span className="preview-action-remediation">{firstSentence(group.primary.remediation)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

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
    </div>
  );
}
