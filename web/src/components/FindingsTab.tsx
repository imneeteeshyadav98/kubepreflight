import { filterFindings, findingResourceLabel, uniqueValues, type Finding, type Report, type Severity } from "../lib/findings-schema";
import { ALL_SEVERITIES, type Filters } from "../types";
import FindingDetail from "./FindingDetail";

interface FindingsTabProps {
  report: Report;
  filters: Filters;
  onFiltersChange: (filters: Filters) => void;
  onReset: () => void;
  selected: Finding | null;
  onSelectFinding: (finding: Finding) => void;
  onClearSelection: () => void;
}

function severityPill(severity: string) {
  return <span className={`severity-pill ${severity.toLowerCase()}`}>{severity}</span>;
}

function confidencePill(confidence: string) {
  return <span className="confidence-pill">{confidence}</span>;
}

function toggleSeverity(active: Severity[], severity: Severity): Severity[] {
  return active.includes(severity) ? active.filter((value) => value !== severity) : [...active, severity];
}

// Split pane: compact list on the left, selected finding's detail on the
// right — both scroll independently (see .findings-tab in styles.css).
// Filters are sticky to the top of the list pane only, not the page.
export default function FindingsTab({ report, filters, onFiltersChange, onReset, selected, onSelectFinding, onClearSelection }: FindingsTabProps) {
  const findings = filterFindings(report.findings, filters);
  const confidences = uniqueValues(report.findings, (finding) => [finding.confidence]);
  const namespaces = uniqueValues(report.findings, (finding) => finding.resources.map((resource) => resource.namespace || "cluster-scoped"));

  return (
    <div className="tab-panel findings-tab" id="findings">
      <div className={`findings-list-pane ${selected ? "mobile-hidden" : ""}`}>
        <div className="filters" aria-label="Finding filters">
          <div className="severity-chips" role="group" aria-label="Severity">
            {ALL_SEVERITIES.map((severity) => {
              const active = filters.severities.includes(severity);
              return (
                <label key={severity} className={`chip chip-${severity.toLowerCase()} ${active ? "chip-active" : ""}`}>
                  <input
                    type="checkbox"
                    checked={active}
                    onChange={() => onFiltersChange({ ...filters, severities: toggleSeverity(filters.severities, severity) })}
                  />
                  {severity}
                </label>
              );
            })}
          </div>
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
            Clear filters
          </button>
          <span id="finding-count">
            {findings.length} of {report.findings.length} findings
          </span>
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
              </tr>
            </thead>
            <tbody id="findings-body">
              {findings.map((finding) => {
                const primary = finding.resources[0];
                const isSelected = selected?.fingerprint === finding.fingerprint;
                return (
                  <tr
                    key={finding.fingerprint}
                    tabIndex={0}
                    role="button"
                    aria-label={`Open ${finding.ruleId} details`}
                    aria-selected={isSelected}
                    className={isSelected ? "row-selected" : ""}
                    data-namespace={primary.namespace || "cluster-scoped"}
                    onClick={() => onSelectFinding(finding)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        onSelectFinding(finding);
                      }
                    }}
                  >
                    <td>{severityPill(finding.severity)}</td>
                    <td>
                      <strong>{finding.ruleId}</strong>
                    </td>
                    <td className="resource-cell">
                      <strong>{findingResourceLabel(finding)}</strong>
                    </td>
                    <td>{confidencePill(finding.confidence)}</td>
                    <td>
                      <span className="plane-pill">{[...new Set(finding.resources.map((resource) => resource.plane))].join(" + ")}</span>
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
      </div>
      <div className={`findings-detail-pane ${!selected ? "mobile-hidden" : ""}`}>
        <FindingDetail finding={selected} onBack={onClearSelection} />
      </div>
    </div>
  );
}
