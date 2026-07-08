import { eksAddonStatus, upgradeDetails, type Finding, type Report } from "../lib/findings-schema";
import TopRisks from "./TopRisks";
import { buildActionGroups, inspectCommand, operatorStep } from "../lib/actions";

interface SummaryTabProps {
  report: Report;
  onOpenFinding: (finding: Finding) => void;
  onViewEvidence: (finding: Finding) => void;
  onViewAllActions: () => void;
}

// The Summary tab is a preview, not a full listing — top 3 risks and top 3
// next actions only, so switching to this tab never becomes a long scroll.
// Full lists live in their own tabs (Findings / Next Actions).
export default function SummaryTab({ report, onOpenFinding, onViewEvidence, onViewAllActions }: SummaryTabProps) {
  const notes = [...report.assumptions];
  if (report.namespaceAllowlist.length) notes.push(`Namespace allowlist: ${report.namespaceAllowlist.join(", ")}`);

  const confidence = new Map<string, number>();
  report.findings.forEach((finding) => confidence.set(finding.confidence, (confidence.get(finding.confidence) || 0) + 1));

	const actionGroups = buildActionGroups(report.findings);
	const topActions = actionGroups.slice(0, 3);
	const hops = upgradeDetails(report);
  const startHere = topActions.slice(0, 4);
  const blockers = report.summary.blockers;

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

      {report.eksAddons && report.eksAddons.length > 0 && (
        <section className="eks-addons-panel" aria-label="EKS add-ons">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Add-on inventory</p>
              <h2>EKS add-ons</h2>
            </div>
          </div>
          <p className="upgrade-path-caption">
            EKS does not automatically update add-ons after a Kubernetes minor version upgrade — review and update them explicitly. Add-ons that fail compatibility also appear as ADDON-001 findings.
          </p>
          <div className="table-wrap">
            <table className="appendix">
              <thead>
                <tr><th>Add-on</th><th>Current version</th><th>Status</th><th>Compatible versions</th></tr>
              </thead>
              <tbody>
                {report.eksAddons.map((addon) => {
                  const status = eksAddonStatus(addon);
                  return (
                    <tr key={addon.name}>
                      <td>{addon.name}</td>
                      <td>{addon.currentVersion || "—"}</td>
                      <td><span className={`eks-addon-status ${status.className}`}>{status.label}</span></td>
                      <td>{addon.compatibleVersions && addon.compatibleVersions.length > 0 ? addon.compatibleVersions.join(", ") : "—"}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {startHere.length > 0 && (
        <section className="start-here-panel" aria-label="Start here">
          <div className="start-here-copy">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Operator sequence</p>
                <h2>Start here</h2>
              </div>
            </div>
            <p className="start-here-intro">Fix these in order:</p>
            <ol className="start-here-list">
              {startHere.map((group) => (
                <li key={group.primary.fingerprint}>
                  <button className="text-button" onClick={() => onOpenFinding(group.primary)}>
                    {operatorStep(group.primary)}
                  </button>
                  <span>{group.resourceLabel}</span>
                </li>
              ))}
            </ol>
            {blockers > 0 && <strong className="gate-warning">Do not start the upgrade until blockers = 0.</strong>}
          </div>
          <aside className="upgrade-gate-checklist" aria-label="Upgrade gate checklist">
            <p className="eyebrow">Upgrade gate checklist</p>
            <label><input type="checkbox" /> Blockers must be 0</label>
            <label><input type="checkbox" /> Warnings reviewed</label>
            <label><input type="checkbox" /> Evidence saved</label>
            <label><input type="checkbox" /> Change window approved</label>
          </aside>
        </section>
      )}

      <TopRisks report={report} onOpenFinding={onOpenFinding} onViewEvidence={onViewEvidence} />

      {hops.length > 0 && (
        <section className="upgrade-path-details" aria-label="Upgrade path details">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Hop-by-hop context</p>
              <h2>Upgrade path details</h2>
            </div>
          </div>
          <p className="upgrade-path-caption">Advisory hop-by-hop context. Re-scan after each hop before treating the next hop as assessed.</p>
          <ol className="upgrade-details-list">
            {hops.map((hop) => (
              <li key={`${hop.from}-${hop.to}`} className={`upgrade-detail-card ${hop.statusClass}`}>
                <div className="upgrade-detail-head">
                  <span className="hop-versions">
                    {hop.from} &rarr; {hop.to}
                  </span>
                  <span className={`upgrade-detail-status ${hop.statusClass}`}>{hop.statusLabel}</span>
                </div>
                <div className="upgrade-detail-body">
                  <h3>Assessment</h3>
                  <p>{hop.assessment}</p>
                  <ul>{hop.findingLines.map((line) => <li key={line}>{line}</li>)}</ul>
                </div>
              </li>
            ))}
          </ol>
          <details className="upgrade-checks-details">
            <summary>Show checks to review</summary>
            <ul>{hops[0].checks.map((check) => <li key={check}>{check}</li>)}</ul>
          </details>
        </section>
      )}

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
				<span className="preview-action-remediation">{operatorStep(group.primary)}</span>
        {inspectCommand(group.primary) && <code className="preview-action-command">{inspectCommand(group.primary)}</code>}
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
