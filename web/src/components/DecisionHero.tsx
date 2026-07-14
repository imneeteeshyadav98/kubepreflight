import { useState } from "react";
import { clusterDisplayName, decisionFromResult, decisionSummaryLine, eksEndpointAccessLabel, eksSupportTypeLabel, upgradeContext, type Report } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";

interface DecisionHeroProps {
  report: Report;
}

function resultClass(result: string): string {
  return result === "BLOCKED" ? "blocked" : result === "CLEAN" ? "clean" : "warning";
}

function decisionClass(decision: string): string {
  return decision === "NO-GO" ? "blocked" : decision === "GO" ? "clean" : "warning";
}

function formatDate(value: string): string {
  if (!value) return "Not supplied";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString();
}

function providerLabel(provider?: string): string {
  switch ((provider || "").toLowerCase()) {
    case "eks":
      return "EKS";
    case "aks":
      return "AKS";
    case "gke":
      return "GKE";
    case "cluster-only":
      return "Cluster-only";
    default:
      return provider || "Unknown";
  }
}

function awsEnrichmentLabel(value?: boolean): string {
  return value ? "On" : "Off";
}

// ClusterIdentifier shows the short, human-friendly cluster name as the
// big heading, with the full identifier (e.g. an EKS ARN) available via a
// native tooltip and a copy button rather than displayed inline — an EKS
// cluster ARN can be 60+ characters and previously made this card
// unnecessarily wide. Renders a plain heading with no copy affordance at
// all when there's nothing beyond the short name to offer (full === "").
function ClusterIdentifier({ report }: { report: Report }) {
  const { short, full } = clusterDisplayName(report);
  const [label, setLabel] = useState("Copy ARN");
  if (!full) {
    return <h1 id="cluster-name">{short}</h1>;
  }
  return (
    <span className="cluster-identifier">
      <h1 id="cluster-name" title={full}>
        {short}
      </h1>
      <button
        type="button"
        className="text-button cluster-copy-button"
        onClick={async (event) => {
          event.stopPropagation();
          setLabel(await copyToClipboard(full));
          setTimeout(() => setLabel("Copy ARN"), 1500);
        }}
      >
        {label}
      </button>
    </span>
  );
}

// Fixed-height header strip — part of the always-visible chrome above the
// tabs (see App.tsx's dashboard-shell), not a scrolling section, so it
// stays compact by design rather than by convention.
export default function DecisionHero({ report }: DecisionHeroProps) {
  const decision = decisionFromResult(report.result);
  const incomplete = Object.values(report.coverage).some((plane) => plane.status === "partial");
  const upgrade = upgradeContext(report);
  // Prefer the honest coverage signal — it correctly reflects a failed/
  // skipped AWS collection even for an "eks" provider run. But a genuinely
  // legacy document (no schemaVersion field at all, so parseFindingsDocument
  // defaulted it to "legacy") never had a coverage field either, and
  // normalizeCoverage's default ("skipped") would otherwise make an old
  // report with real AWS findings wrongly show "AWS enrichment: false" —
  // fall back to the old provider/finding-based heuristic only for those.
  const awsEnrichment =
    report.coverage.aws.status === "complete" ||
    (report.schemaVersion === "legacy" &&
      (report.provider === "eks" || report.findings.some((finding) => finding.resources.some((resource) => resource.plane === "aws"))));

  return (
    <header className="decision-strip" id="summary">
      <div className="decision-strip-row">
        <span className={`decision-chip ${decisionClass(decision)}`}>{decision}</span>
        <span className={`result-badge ${resultClass(report.result)}`} id="result-badge">
          {report.result}
        </span>
        <ClusterIdentifier report={report} />
      </div>
      <p className="decision-why" id="decision-why">
        {decisionSummaryLine(report.summary, incomplete)}
      </p>
      <p className="upgrade-context-line">{upgrade.line}</p>
      {upgrade.note ? <p className="upgrade-context-note">{upgrade.note}</p> : null}
      <dl className="decision-meta">
        <div>
          <dt>Current</dt>
          <dd id="current-version">{upgrade.current}</dd>
        </div>
        <div>
          <dt>Target</dt>
          <dd id="target-version">{report.targetVersion}</dd>
        </div>
        <div className="decision-meta-wide">
          <dt>Upgrade path</dt>
          <dd id="upgrade-path">
            {upgrade.path}
            <span>{upgrade.label}</span>
          </dd>
        </div>
        <div>
          <dt>Provider</dt>
          <dd id="provider-name">{providerLabel(report.provider)}</dd>
        </div>
        <div>
          <dt>AWS enrichment</dt>
          <dd id="aws-enrichment">{awsEnrichmentLabel(awsEnrichment)}</dd>
        </div>
        <div>
          <dt>Scanned</dt>
          <dd id="scanned-at">{formatDate(report.scannedAt)}</dd>
        </div>
        {report.eksCluster?.region && (
          <div>
            <dt>Region</dt>
            <dd id="eks-region">{report.eksCluster.region}</dd>
          </div>
        )}
        {report.eksCluster?.version && (
          <div>
            <dt>EKS version</dt>
            <dd id="eks-version">{report.eksCluster.version}</dd>
          </div>
        )}
        {report.eksCluster?.platformVersion && (
          <div>
            <dt>Platform version</dt>
            <dd id="eks-platform-version">{report.eksCluster.platformVersion}</dd>
          </div>
        )}
        {report.eksCluster?.status && (
          <div>
            <dt>EKS status</dt>
            <dd id="eks-status">{report.eksCluster.status}</dd>
          </div>
        )}
        {eksSupportTypeLabel(report.eksCluster?.supportType) && (
          <div>
            <dt>Support</dt>
            <dd id="eks-support-type">{eksSupportTypeLabel(report.eksCluster?.supportType)}</dd>
          </div>
        )}
        {eksEndpointAccessLabel(report.eksCluster?.endpointAccess) && (
          <div>
            <dt>Endpoint access</dt>
            <dd id="eks-endpoint-access">{eksEndpointAccessLabel(report.eksCluster?.endpointAccess)}</dd>
          </div>
        )}
      </dl>
    </header>
  );
}
