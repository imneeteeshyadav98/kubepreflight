import { findingResourceLabel, firstSentence, type Finding, type Report } from "../lib/findings-schema";
import TopRisks from "./TopRisks";

interface SummaryTabProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
  onViewAllActions: () => void;
}

const SEVERITY_RANK: Record<string, number> = { Blocker: 0, Warning: 1, Info: 2 };

// The Summary tab is a preview, not a full listing — top 3 risks and top 3
// next actions only, so switching to this tab never becomes a long scroll.
// Full lists live in their own tabs (Findings / Next Actions).
export default function SummaryTab({ report, onOpenFinding, onViewAllActions }: SummaryTabProps) {
  const notes = [...report.assumptions];
  if (report.namespaceAllowlist.length) notes.push(`Namespace allowlist: ${report.namespaceAllowlist.join(", ")}`);

  const confidence = new Map<string, number>();
  report.findings.forEach((finding) => confidence.set(finding.confidence, (confidence.get(finding.confidence) || 0) + 1));

  const topActions = [...report.findings]
    .filter((finding) => finding.remediation)
    .sort((a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] || a.ruleId.localeCompare(b.ruleId))
    .slice(0, 3);

  return (
    <div className="tab-panel summary-tab">
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
              View all ({report.findings.filter((finding) => finding.remediation).length})
            </button>
          </div>
          <ul className="preview-action-list">
            {topActions.map((finding) => (
              <li key={finding.fingerprint} onClick={() => onOpenFinding(finding)}>
                <span className={`severity-pill ${finding.severity.toLowerCase()}`}>{finding.severity}</span>
                <strong>{findingResourceLabel(finding)}</strong>
                <span className="preview-action-remediation">{firstSentence(finding.remediation)}</span>
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
