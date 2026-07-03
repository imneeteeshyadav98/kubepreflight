import { findingResourceLabel, firstSentence, topRisks, type Finding, type Report } from "../lib/findings-schema";

interface TopRisksProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
}

export default function TopRisks({ report, onOpenFinding }: TopRisksProps) {
  const risks = topRisks(report.findings, 3);
  if (risks.length === 0) return null;

  return (
    <section className="top-risks" id="top-risks" aria-label="Top risks">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Highest severity first</p>
          <h2>Top risks</h2>
        </div>
      </div>
      <div className="top-risks-grid">
        {risks.map((finding, index) => (
          <article
            className="top-risk-card"
            key={finding.fingerprint}
            tabIndex={0}
            role="button"
            aria-label={`Open ${finding.ruleId} details`}
            onClick={() => onOpenFinding(finding)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                onOpenFinding(finding);
              }
            }}
          >
            <div className="top-risk-head">
              <span className="top-risk-rank">{index + 1}</span>
              <span className={`severity-pill ${finding.severity.toLowerCase()}`}>{finding.severity}</span>
              <span className="rule-chip">{finding.ruleId}</span>
            </div>
            <strong className="top-risk-resource">{findingResourceLabel(finding)}</strong>
            <p className="top-risk-reason">{firstSentence(finding.message)}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
