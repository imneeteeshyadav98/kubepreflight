import { useState } from "react";
import type { Finding, Report, Severity } from "../lib/findings-schema";
import { firstSentence, priorityPillClass, upgradeApplicable } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";
import { buildActionGroups, type ActionGroupModel } from "../lib/actions";

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

function ActionGroup({ title, groups, onOpenFinding }: { title: string; groups: ActionGroupModel[]; onOpenFinding: (finding: Finding) => void }) {
	if (groups.length === 0) return null;
  return (
    <div className="action-group">
      <h3 className="action-group-title">{title}</h3>
      <div className="action-list">
		{groups.map((group, index) => {
		  const finding = group.primary;
		  // Exclude the primary finding from its own "related" sub-list —
		  // group.findings is the full group (primary included), so
		  // rendering it unfiltered redundantly re-lists the primary
		  // underneath its own card.
		  const related = group.findings.filter((candidate) => candidate.fingerprint !== finding.fingerprint);
		  return (
		  <article className={`action-item ${finding.severity.toLowerCase()}`} key={finding.fingerprint} role="button" tabIndex={0} onClick={() => onOpenFinding(finding)} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onOpenFinding(finding); } }}>
            <span className="action-number">{String(index + 1).padStart(2, "0")}</span>
            <div className="action-resource">
			  {finding.priority && (
			    <span className={`priority-pill ${priorityPillClass(finding.priority)}`} title={finding.priorityReason}>
			      {finding.priority}
			    </span>
			  )}
			  <strong>{group.resourceLabel}</strong>
			  <small>{group.ruleIds.join(", ")}</small>
            </div>
            <p className="action-copy">{firstSentence(finding.remediation)}</p>
			<CopyButton text={finding.remediation} />
			{related.length > 0 && <ul className="evidence-list">{related.map((relatedFinding) => <li key={relatedFinding.fingerprint}><button className="text-button" onClick={(event) => { event.stopPropagation(); onOpenFinding(relatedFinding); }}>{relatedFinding.ruleId}: {firstSentence(relatedFinding.remediation)}</button></li>)}</ul>}
		  </article>
		)})}
      </div>
    </div>
  );
}

// Its own tab now (was a full-width section on the long-document page) —
// the whole panel scrolls internally; the tab nav above it stays fixed.
export default function NextActionsTab({ report, onOpenFinding }: NextActionsTabProps) {
	const groups = buildActionGroups(report.findings);
	const bySeverity = (severity: Severity) => groups.filter((group) => group.severity === severity);
	const blockers = bySeverity("Blocker");
	const warnings = bySeverity("Warning");
	const infos = bySeverity("Info");
	const upgradeIsApplicable = upgradeApplicable(report);

  return (
    <div className="tab-panel actions-tab" id="actions">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Change plan</p>
          <h2>{upgradeIsApplicable ? "Next actions" : "Recommended maintenance"}</h2>
        </div>
        <span>Safest-first remediation order</span>
      </div>
	  {groups.length === 0 ? (
        <p className="empty-state">No actionable findings.</p>
      ) : (
        <>
		<ActionGroup title={`Blockers (${blockers.length})`} groups={blockers} onOpenFinding={onOpenFinding} />
		<ActionGroup title={`Warnings (${warnings.length})`} groups={warnings} onOpenFinding={onOpenFinding} />
		<ActionGroup title={`Info (${infos.length})`} groups={infos} onOpenFinding={onOpenFinding} />
        </>
      )}
    </div>
  );
}
