import type { Report } from "../lib/findings-schema";

interface ScanBannerProps {
  report: Report;
  sourceName: string;
}

function resultClass(result: string): string {
  return result === "BLOCKED" ? "blocked" : result === "CLEAN" ? "clean" : "warning";
}

function formatDate(value: string): string {
  if (!value) return "Not supplied";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString();
}

export default function ScanBanner({ report, sourceName }: ScanBannerProps) {
  const awsEnrichment =
    report.provider === "eks" || report.findings.some((finding) => finding.resources.some((resource) => resource.plane === "aws"));

  return (
    <section className="scan-banner" id="summary">
      <div>
        <p className="eyebrow">Current scan</p>
        <div className="result-line">
          <span className={`result-badge ${resultClass(report.result)}`} id="result-badge">
            {report.result}
          </span>
          <h2 id="cluster-name">{report.clusterContext}</h2>
        </div>
        <p className="scan-subtitle" id="scan-subtitle">
          {report.findings.length} findings · source: {sourceName}
        </p>
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
