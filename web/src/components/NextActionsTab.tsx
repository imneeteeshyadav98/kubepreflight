import { useState } from "react";
import type { Finding, Report } from "../lib/findings-schema";
import { findingResourceLabel, firstSentence } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";

interface NextActionsTabProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
}

function CopyButton({ text }: { text: string }) {
  const [label, setLabel] = useState("Copy");
  return (
    <button
      className="text-button action-copy-button"
      onClick={async (event) => {
        event.stopPropagation();
        setLabel(await copyToClipboard(text));
        setTimeout(() => setLabel("Copy"), 1500);
      }}
    >
      {label}
    </button>
  );
}

function ActionGroup({ title, findings, onOpenFinding }: { title: string; findings: Finding[]; onOpenFinding: (finding: Finding) => void }) {
  if (findings.length === 0) return null;
  return (
    <div className="action-group">
      <h3 className="action-group-title">{title}</h3>
      <div className="action-list">
        {findings.map((finding, index) => (
          <article className="action-item" key={finding.fingerprint} onClick={() => onOpenFinding(finding)}>
            <span className="action-number">{String(index + 1).padStart(2, "0")}</span>
            <div className="action-resource">
              <strong>{findingResourceLabel(finding)}</strong>
              <small>{finding.ruleId}</small>
            </div>
            <p className="action-copy">{firstSentence(finding.remediation)}</p>
            <CopyButton text={finding.remediation} />
          </article>
        ))}
      </div>
    </div>
  );
}

// Its own tab now (was a full-width section on the long-document page) —
// the whole panel scrolls internally; the tab nav above it stays fixed.
export default function NextActionsTab({ report, onOpenFinding }: NextActionsTabProps) {
  const actionable = report.findings.filter((finding) => finding.remediation);

  const blockers = actionable.filter((finding) => finding.severity === "Blocker").sort((a, b) => a.ruleId.localeCompare(b.ruleId));
  const warnings = actionable.filter((finding) => finding.severity === "Warning").sort((a, b) => a.ruleId.localeCompare(b.ruleId));
  const infos = actionable.filter((finding) => finding.severity === "Info").sort((a, b) => a.ruleId.localeCompare(b.ruleId));

  return (
    <div className="tab-panel actions-tab" id="actions">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Change plan</p>
          <h2>Next actions</h2>
        </div>
        <span>Safest-first remediation order</span>
      </div>
      {actionable.length === 0 ? (
        <p className="empty-state">No actionable findings.</p>
      ) : (
        <>
          <ActionGroup title={`Blockers (${blockers.length})`} findings={blockers} onOpenFinding={onOpenFinding} />
          <ActionGroup title={`Warnings (${warnings.length})`} findings={warnings} onOpenFinding={onOpenFinding} />
          <ActionGroup title={`Info (${infos.length})`} findings={infos} onOpenFinding={onOpenFinding} />
        </>
      )}
    </div>
  );
}
