import { eksAddonStatus, eksNodegroupHealthLabel, eksNodegroupReadinessClass, eksUpgradeInsightDetails, eksUpgradeInsightStatusClass, priorityPillClass, upgradeDetails, type APICompatibilitySummary, type EKSNodegroupInfo, type Finding, type Report } from "../lib/findings-schema";
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
  const nodegroupCoverageUnavailable = report.coverage.aws.errors.some((error) => error.includes("list-nodegroups") || error.includes("describe-nodegroup:"));
  const showEKSNodegroups = report.eksNodegroups !== undefined || (report.provider === "eks" && !!report.eksCluster && !nodegroupCoverageUnavailable);
  const upgradeInsightsUnavailable = report.coverage.aws.errors.some((error) => error.includes("list-insights") || ((report.eksUpgradeInsights?.length ?? 0) === 0 && error.includes("describe-insight:")));
  const showEKSUpgradeInsights = report.eksUpgradeInsights !== undefined || (report.provider === "eks" && !!report.eksCluster);

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

      {report.apiCompatibility && (
        <section className="api-compatibility-panel" aria-label="Kubernetes API compatibility">
          <div className="section-heading">
            <div>
              <p className="eyebrow">API readiness</p>
              <h2>Kubernetes API compatibility</h2>
            </div>
          </div>
          <div className="table-wrap">
            <table className="appendix">
              <thead>
                <tr><th>Status</th><th>Upgrade continue</th><th>Score impact</th><th>Removed objects</th><th>Deprecated objects</th><th>Critical impact</th></tr>
              </thead>
              <tbody>
                <tr>
                  <td><span className={`eks-addon-status ${apiCompatibilityStatusClass(report.apiCompatibility)}`}>{report.apiCompatibility.status}</span></td>
                  <td>{yesNo(report.apiCompatibility.upgradeContinue)}</td>
                  <td>{report.apiCompatibility.scoreImpact}</td>
                  <td>{report.apiCompatibility.removedObjects}</td>
                  <td>{report.apiCompatibility.deprecatedObjects}</td>
                  <td>{yesNo(report.apiCompatibility.criticalImpact)}</td>
                </tr>
              </tbody>
            </table>
          </div>
          {report.apiCompatibility.removedFamilies && report.apiCompatibility.removedFamilies.length > 0 && (
            <div className="table-wrap">
              <table className="appendix">
                <thead>
                  <tr><th>Removed API version</th><th>Kind</th><th>Objects</th><th>Resources</th></tr>
                </thead>
                <tbody>
                  {report.apiCompatibility.removedFamilies.map((family) => (
                    <tr key={`removed-${family.apiVersion}-${family.kind}`}>
                      <td>{family.apiVersion || "—"}</td>
                      <td>{family.kind || "—"}</td>
                      <td>{family.count}</td>
                      <td>{family.resources && family.resources.length > 0 ? family.resources.join(", ") : "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {report.apiCompatibility.deprecatedFamilies && report.apiCompatibility.deprecatedFamilies.length > 0 && (
            <div className="table-wrap">
              <table className="appendix">
                <thead>
                  <tr><th>Deprecated API version</th><th>Kind</th><th>Objects</th><th>Resources</th></tr>
                </thead>
                <tbody>
                  {report.apiCompatibility.deprecatedFamilies.map((family) => (
                    <tr key={`deprecated-${family.apiVersion}-${family.kind}`}>
                      <td>{family.apiVersion || "—"}</td>
                      <td>{family.kind || "—"}</td>
                      <td>{family.count}</td>
                      <td>{family.resources && family.resources.length > 0 ? family.resources.join(", ") : "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
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

      {showEKSNodegroups && (
        <section className="eks-nodegroups-panel" aria-label="EKS managed node groups">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Node group inventory</p>
              <h2>EKS managed node groups</h2>
            </div>
          </div>
          <p className="upgrade-path-caption">Inventory covers EKS managed node groups returned by AWS ListNodegroups. Self-managed nodes are not listed by that API.</p>
          {report.eksNodegroups && report.eksNodegroups.length > 0 ? (
            <div className="table-wrap">
              <table className="appendix">
                <thead>
                  <tr><th>Node group</th><th>Status</th><th>Version</th><th>Release</th><th>AMI type</th><th>Capacity</th><th>Desired/min/max</th><th>Update config</th><th>Health</th><th>Readiness</th></tr>
                </thead>
                <tbody>
                  {report.eksNodegroups.map((nodegroup) => (
                    <tr key={nodegroup.name}>
                      <td>{nodegroup.name}</td>
                      <td>{nodegroup.status || "—"}</td>
                      <td>{nodegroup.version || "—"}</td>
                      <td>{nodegroup.releaseVersion || "—"}</td>
                      <td>{nodegroup.amiType || "—"}</td>
                      <td>{nodegroup.capacityType || "—"}</td>
                      <td>{scalingLabel(nodegroup)}</td>
                      <td>{updateConfigLabel(nodegroup)}</td>
                      <td>{eksNodegroupHealthLabel(nodegroup)}</td>
                      <td><span className={`eks-addon-status ${eksNodegroupReadinessClass(nodegroup)}`}>{nodegroup.readinessStatus}</span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="empty-state">No EKS managed node groups found. Self-managed nodes are not listed by the EKS ListNodegroups API.</p>
          )}
        </section>
      )}

      {showEKSUpgradeInsights && (
        <section className="eks-upgrade-insights-panel" aria-label="EKS Upgrade Insights">
          <div className="section-heading">
            <div>
              <p className="eyebrow">AWS-native signal</p>
              <h2>EKS Upgrade Insights</h2>
            </div>
          </div>
          <p className="upgrade-path-caption">AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.</p>
          {upgradeInsightsUnavailable ? (
            <p className="empty-state">EKS Upgrade Insights unavailable. Kubernetes findings are still valid.</p>
          ) : report.eksUpgradeInsights && report.eksUpgradeInsights.length > 0 ? (
            <div className="table-wrap">
              <table className="appendix">
                <thead>
                  <tr><th>Insight</th><th>Status</th><th>Kubernetes version</th><th>Last refreshed</th><th>Recommendation</th><th>Details</th></tr>
                </thead>
                <tbody>
                  {report.eksUpgradeInsights.map((insight) => (
                    <tr key={insight.id}>
                      <td>{insight.name}</td>
                      <td><span className={`eks-addon-status ${eksUpgradeInsightStatusClass(insight)}`}>{insight.status}</span></td>
                      <td>{insight.kubernetesVersion || "—"}</td>
                      <td>{insightTimeLabel(insight.lastRefreshTime, insight.lastTransitionTime)}</td>
                      <td>{insight.recommendation || "—"}</td>
                      <td>{eksUpgradeInsightDetails(insight)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="empty-state">No EKS upgrade insights returned.</p>
          )}
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
				{group.primary.priority && (
				  <span className={`priority-pill ${priorityPillClass(group.primary.priority)}`} title={group.primary.priorityReason}>
				    {group.primary.priority}
				  </span>
				)}
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

function scalingLabel(nodegroup: EKSNodegroupInfo): string {
  return `${numberOrDash(nodegroup.desiredSize)} / ${numberOrDash(nodegroup.minSize)} / ${numberOrDash(nodegroup.maxSize)}`;
}

function yesNo(value: boolean): string {
  return value ? "Yes" : "No";
}

function apiCompatibilityStatusClass(summary: APICompatibilitySummary): "clean" | "warn" | "blocked" {
  if (summary.status === "Failed") return "blocked";
  if (summary.status === "Warning") return "warn";
  return "clean";
}

function updateConfigLabel(nodegroup: EKSNodegroupInfo): string {
  if (nodegroup.maxUnavailable !== undefined) return `maxUnavailable: ${nodegroup.maxUnavailable}`;
  if (nodegroup.maxUnavailablePercentage !== undefined) return `maxUnavailable: ${nodegroup.maxUnavailablePercentage}%`;
  return "—";
}

function numberOrDash(value?: number): string {
  return value === undefined ? "—" : String(value);
}

function insightTimeLabel(refresh?: string, transition?: string): string {
  if (refresh && transition) return `${refresh} / ${transition}`;
  if (refresh) return refresh;
  if (transition) return `transition: ${transition}`;
  return "—";
}
