import { filterFindings, findingResourceLabel, uniqueValues, type Finding, type Report } from "../lib/findings-schema";
import type { Filters } from "../types";

interface FindingsSectionProps {
  report: Report;
  filters: Filters;
  onFiltersChange: (filters: Filters) => void;
  onReset: () => void;
  onOpenFinding: (finding: Finding) => void;
}

function severityPill(severity: string) {
  return <span className={`severity-pill ${severity.toLowerCase()}`}>{severity}</span>;
}

function confidencePill(confidence: string) {
  return <span className="confidence-pill">{confidence}</span>;
}

export default function FindingsSection({ report, filters, onFiltersChange, onReset, onOpenFinding }: FindingsSectionProps) {
  const findings = filterFindings(report.findings, filters);
  const severities = uniqueValues(report.findings, (finding) => [finding.severity]);
  const confidences = uniqueValues(report.findings, (finding) => [finding.confidence]);
  const namespaces = uniqueValues(report.findings, (finding) => finding.resources.map((resource) => resource.namespace || "cluster-scoped"));

  return (
    <section className="findings-section" id="findings">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Risk register</p>
          <h2>Findings</h2>
        </div>
        <span id="finding-count">
          {findings.length} of {report.findings.length} findings
        </span>
      </div>
      <div className="filters" aria-label="Finding filters">
        <label className="search-field">
          <span>Search</span>
          <input
            id="search-filter"
            type="search"
            placeholder="Rule, resource, or message"
            value={filters.search}
            onChange={(event) => onFiltersChange({ ...filters, search: event.target.value })}
          />
        </label>
        <label>
          <span>Severity</span>
          <select id="severity-filter" value={filters.severity} onChange={(event) => onFiltersChange({ ...filters, severity: event.target.value })}>
            <option value="">All severities</option>
            {severities.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Confidence</span>
          <select id="confidence-filter" value={filters.confidence} onChange={(event) => onFiltersChange({ ...filters, confidence: event.target.value })}>
            <option value="">All confidence</option>
            {confidences.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Namespace</span>
          <select id="namespace-filter" value={filters.namespace} onChange={(event) => onFiltersChange({ ...filters, namespace: event.target.value })}>
            <option value="">All namespaces</option>
            {namespaces.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <button className="text-button filter-reset" id="reset-filters" onClick={onReset}>
          Reset
        </button>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Severity</th>
              <th>Rule</th>
              <th>Resource</th>
              <th>Confidence</th>
              <th>Plane</th>
              <th>
                <span className="sr-only">Open</span>
              </th>
            </tr>
          </thead>
          <tbody id="findings-body">
            {findings.map((finding) => {
              const primary = finding.resources[0];
              return (
                <tr
                  key={finding.fingerprint}
                  tabIndex={0}
                  role="button"
                  aria-label={`Open ${finding.ruleId} details`}
                  data-namespace={primary.namespace || "cluster-scoped"}
                  onClick={() => onOpenFinding(finding)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onOpenFinding(finding);
                    }
                  }}
                >
                  <td>{severityPill(finding.severity)}</td>
                  <td>
                    <strong>{finding.ruleId}</strong>
                  </td>
                  <td className="resource-cell">
                    <strong>{findingResourceLabel(finding)}</strong>
                    <small>{finding.message}</small>
                  </td>
                  <td>{confidencePill(finding.confidence)}</td>
                  <td>
                    <span className="plane-pill">{[...new Set(finding.resources.map((resource) => resource.plane))].join(" + ")}</span>
                  </td>
                  <td>
                    <button className="row-open" aria-label="Open details" onClick={(event) => { event.stopPropagation(); onOpenFinding(finding); }}>
                      →
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        <div className="empty-state" id="empty-state" hidden={findings.length !== 0}>
          No findings match these filters.
        </div>
      </div>
    </section>
  );
}
