import { decisionFromResult, decisionSummaryLine, type Report } from "../lib/findings-schema";

interface DecisionHeroProps {
  report: Report;
  sourceName: string;
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

export default function DecisionHero({ report, sourceName }: DecisionHeroProps) {
  const decision = decisionFromResult(report.result);
  const awsEnrichment =
    report.provider === "eks" || report.findings.some((finding) => finding.resources.some((resource) => resource.plane === "aws"));

  return (
    <section className="decision-hero" id="summary">
      <p className="eyebrow">Upgrade readiness</p>
      <div className="decision-hero-main">
        <div className={`decision-mark ${decisionClass(decision)}`}>
          <span className="decision-label">{decision}</span>
        </div>
        <div className="decision-copy">
          <div className="result-line">
            <span className={`result-badge ${resultClass(report.result)}`} id="result-badge">
              {report.result}
            </span>
            <h2 id="cluster-name">{report.clusterContext}</h2>
          </div>
          <p className="decision-why" id="decision-why">
            {decisionSummaryLine(report.summary)}
          </p>
          <p className="scan-subtitle" id="scan-subtitle">
            {report.findings.length} findings · source: {sourceName}
          </p>
        </div>
      </div>
      <dl className="scan-meta">
        <div>
          <dt>Target</dt>
          <dd id="target-version">{report.targetVersion}</dd>
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
    </section>
  );
}
