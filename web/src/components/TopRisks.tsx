import { findingResourceLabel, firstSentence, topRisks, type Finding, type Report } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";
import { inspectCommand, operatorStep } from "../lib/actions";
import { useState } from "react";

interface TopRisksProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
  onViewEvidence: (finding: Finding) => void;
}

function CopyInspectButton({ command }: { command: string }) {
  const [label, setLabel] = useState("Copy inspect command");
  if (!command) return null;
  return (
    <button
      className="button button-secondary top-risk-rail-button"
      onClick={async (event) => {
        event.stopPropagation();
        setLabel(await copyToClipboard(command));
        setTimeout(() => setLabel("Copy inspect command"), 1500);
      }}
    >
      {label}
    </button>
  );
}

export default function TopRisks({ report, onOpenFinding, onViewEvidence }: TopRisksProps) {
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
            className={`top-risk-card top-risk-action-card ${finding.severity.toLowerCase()}`}
            key={finding.fingerprint}
          >
            <div
              className="top-risk-main"
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
              <p className="top-risk-remediation">{firstSentence(finding.remediation)}</p>
            </div>
            <aside className="top-risk-rail" aria-label={`${finding.ruleId} next step`}>
              <h3>Next step</h3>
              <p>{operatorStep(finding)}</p>
              {inspectCommand(finding) && (
                <>
                  <p className="inspect-label">Inspect current state first. This does not change the cluster.</p>
                  <pre>{inspectCommand(finding)}</pre>
                  <CopyInspectButton command={inspectCommand(finding)} />
                </>
              )}
              <button className="button button-secondary top-risk-rail-button" onClick={(event) => { event.stopPropagation(); onOpenFinding(finding); }}>
                View full finding
              </button>
              <button className="button button-secondary top-risk-rail-button" onClick={(event) => { event.stopPropagation(); onViewEvidence(finding); }}>
                View evidence
              </button>
            </aside>
          </article>
        ))}
      </div>
    </section>
  );
}
