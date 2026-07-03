import type { Finding, Report } from "../lib/findings-schema";
import { findingResourceLabel } from "../lib/findings-schema";

interface ActionsSectionProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
}

const SEVERITY_RANK: Record<string, number> = { Blocker: 0, Warning: 1, Info: 2 };

function firstSentence(value: string): string {
  const firstLine = value.split("\n").find((line) => line.trim()) || value;
  return firstLine.length > 240 ? `${firstLine.slice(0, 237)}…` : firstLine;
}

export default function ActionsSection({ report, onOpenFinding }: ActionsSectionProps) {
  const actions = [...report.findings]
    .filter((finding) => finding.remediation)
    .sort((a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] || a.ruleId.localeCompare(b.ruleId));

  return (
    <section className="actions-section" id="actions">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Change plan</p>
          <h2>Next actions</h2>
        </div>
        <span>Highest severity first</span>
      </div>
      <div className="action-list" id="action-list">
        {actions.map((finding, index) => (
          <article className="action-item" key={finding.fingerprint} onClick={() => onOpenFinding(finding)}>
            <span className="action-number">{String(index + 1).padStart(2, "0")}</span>
            <div className="action-resource">
              <strong>{findingResourceLabel(finding)}</strong>
              <small>
                {finding.ruleId} · {finding.severity}
              </small>
            </div>
            <p className="action-copy">{firstSentence(finding.remediation)}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
