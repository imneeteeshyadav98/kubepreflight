import { decisionFromResult, decisionSummaryLine, upgradeContext, type Report } from "../lib/findings-schema";

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
        <h1 id="cluster-name">{report.clusterContext}</h1>
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
          <dd id="provider-name">{report.provider}</dd>
        </div>
        <div>
          <dt>AWS enrichment</dt>
          <dd id="aws-enrichment">{String(awsEnrichment)}</dd>
        </div>
        <div>
          <dt>Scanned</dt>
          <dd id="scanned-at">{formatDate(report.scannedAt)}</dd>
        </div>
      </dl>
    </header>
  );
}
