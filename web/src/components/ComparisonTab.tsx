import { useEffect, useState, type ChangeEvent } from "react";
import type { Finding, Report } from "../lib/findings-schema";
import type { ChangedFinding, Comparison } from "../lib/comparison-schema";
import { findingResourceLabel } from "../lib/findings-schema";

interface ComparisonTabProps {
  report: Report;
  baselineName: string;
  comparison: Comparison | null;
  error: string | null;
  onBaselineFile: (event: ChangeEvent<HTMLInputElement>) => void;
  onClearBaseline: () => void;
  onOpenFinding: (finding: Finding) => void;
}

const COMPARISON_PAGE_SIZE = 250;

function verdictClass(verdict: string): "clean" | "warn" | "blocked" {
  if (verdict === "BLOCKED") return "blocked";
  if (verdict === "PASSED_WITH_WARNINGS" || verdict === "INCOMPLETE") return "warn";
  return "clean";
}

function severityClass(severity: string): "clean" | "warn" | "blocked" {
  if (severity === "Blocker") return "blocked";
  if (severity === "Warning") return "warn";
  return "clean";
}

function signedDelta(delta: number): string {
  return delta > 0 ? `+${delta}` : `${delta}`;
}

// ComparisonTab treats the currently-loaded report (App.tsx's `report`) as
// "current" and a second, separately-uploaded findings.json as "baseline" —
// the natural Console workflow is "I already have this scan open; let me
// see what changed since an earlier one," not a dedicated two-file import
// screen. New/Changed/Unchanged findings come from `report.findings`
// (already loaded, already navigable via onOpenFinding); Resolved findings
// only exist in the baseline and have no matching entry in the current
// report's Findings tab, so they render read-only here.
export default function ComparisonTab({ report, baselineName, comparison, error, onBaselineFile, onClearBaseline, onOpenFinding }: ComparisonTabProps) {
  if (!comparison) {
    return (
      <section className="comparison-panel" id="comparison-panel" aria-labelledby="comparison-heading">
        <p className="eyebrow">Compare against an earlier scan</p>
        <h2 id="comparison-heading">Upload a baseline findings.json</h2>
        <p>
          The currently-loaded scan (<code>{report.clusterContext}</code>, target <code>{report.targetVersion}</code>) becomes{" "}
          <strong>current</strong>. Choose an earlier <code>findings.json</code> from the same cluster to see what's new,
          resolved, or changed since then.
        </p>
        <div className="import-actions">
          <label className="button button-primary" htmlFor="baseline-file-input">
            Choose baseline findings.json
          </label>
          <input id="baseline-file-input" type="file" accept="application/json,.json" hidden onChange={onBaselineFile} />
        </div>
        {error && (
          <p className="error-message" id="comparison-error-message" role="alert">
            {error}
          </p>
        )}
      </section>
    );
  }

  const s = comparison.summary;

  return (
    <section className="comparison-panel" id="comparison-panel" aria-labelledby="comparison-heading">
      <div className="comparison-header">
        <div>
          <p className="eyebrow">Comparing against</p>
          <h2 id="comparison-heading">{baselineName}</h2>
        </div>
        <button className="button button-ghost" id="comparison-clear-button" onClick={onClearBaseline}>
          Change baseline
        </button>
      </div>

      {comparison.warnings.map((warning) => (
        <p className="error-message" key={warning} role="alert">
          {warning}
        </p>
      ))}

      <table className="comparison-summary-table">
        <tbody>
          <tr>
            <th>Verdict</th>
            <td>
              <span className={`eks-addon-status ${verdictClass(s.baselineVerdict)}`}>{s.baselineVerdict}</span>
              {s.verdictChanged && (
                <>
                  {" → "}
                  <span className={`eks-addon-status ${verdictClass(s.currentVerdict)}`}>{s.currentVerdict}</span>
                </>
              )}
            </td>
          </tr>
          <tr>
            <th>Readiness score</th>
            <td>
              {s.baselineReadinessScore} → {s.currentReadinessScore} ({signedDelta(s.readinessScoreDelta)})
            </td>
          </tr>
          <tr>
            <th>New</th>
            <td id="comparison-new-count">
              {s.new} ({s.newBlockers} blocker(s))
            </td>
          </tr>
          <tr>
            <th>Resolved</th>
            <td id="comparison-resolved-count">
              {s.resolved} ({s.resolvedBlockers} blocker(s))
            </td>
          </tr>
          <tr>
            <th>Changed</th>
            <td id="comparison-changed-count">{s.changed}</td>
          </tr>
          <tr>
            <th>Unchanged</th>
            <td id="comparison-unchanged-count">{s.unchanged}</td>
          </tr>
        </tbody>
      </table>

      <ComparisonFindingList title="New findings" findings={comparison.new} onOpenFinding={onOpenFinding} navigable emptyLabel="No new findings." />
      <ComparisonChangedList changed={comparison.changed} />
      <ComparisonFindingList title="Resolved findings" findings={comparison.resolved} onOpenFinding={onOpenFinding} navigable={false} emptyLabel="No resolved findings." />

      <details className="comparison-unchanged" id="comparison-unchanged-details">
        <summary>Unchanged findings ({comparison.unchanged.length})</summary>
        <ComparisonFindingList title="" findings={comparison.unchanged} onOpenFinding={onOpenFinding} navigable emptyLabel="No unchanged findings." hideHeading />
      </details>
    </section>
  );
}

interface ComparisonFindingListProps {
  title: string;
  findings: Finding[];
  onOpenFinding: (finding: Finding) => void;
  navigable: boolean;
  emptyLabel: string;
  hideHeading?: boolean;
}

function ComparisonFindingList({ title, findings, onOpenFinding, navigable, emptyLabel, hideHeading }: ComparisonFindingListProps) {
  const [visibleCount, setVisibleCount] = useState(COMPARISON_PAGE_SIZE);
  const visibleFindings = findings.slice(0, visibleCount);

  useEffect(() => {
    setVisibleCount(COMPARISON_PAGE_SIZE);
  }, [findings]);

  return (
    <div className="comparison-section">
      {!hideHeading && <h3>{title} ({findings.length})</h3>}
      {findings.length === 0 ? (
        <p className="comparison-empty">{emptyLabel}</p>
      ) : (
        <table className="comparison-findings-table">
          <thead>
            <tr>
              <th>Priority</th>
              <th>Severity</th>
              <th>Rule</th>
              <th>Resource</th>
              <th>Message</th>
            </tr>
          </thead>
          <tbody>
            {visibleFindings.map((finding) => (
              <tr key={finding.fingerprint}>
                <td>{finding.priority}</td>
                <td>
                  <span className={`eks-addon-status ${severityClass(finding.severity)}`}>{finding.severity}</span>
                </td>
                <td>
                  {navigable ? (
                    <button type="button" className="rule-id-chip" onClick={() => onOpenFinding(finding)}>
                      {finding.ruleId}
                    </button>
                  ) : (
                    <code>{finding.ruleId}</code>
                  )}
                </td>
                <td>{findingResourceLabel(finding)}</td>
                <td>{finding.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {visibleCount < findings.length && (
        <div className="pagination-controls">
          <span>
            Showing {visibleFindings.length} of {findings.length}
          </span>
          <button className="text-button" type="button" onClick={() => setVisibleCount((count) => Math.min(count + COMPARISON_PAGE_SIZE, findings.length))}>
            Show more
          </button>
        </div>
      )}
    </div>
  );
}

function ComparisonChangedList({ changed }: { changed: ChangedFinding[] }) {
  const [visibleCount, setVisibleCount] = useState(COMPARISON_PAGE_SIZE);
  const visibleChanges = changed.slice(0, visibleCount);

  useEffect(() => {
    setVisibleCount(COMPARISON_PAGE_SIZE);
  }, [changed]);

  return (
    <div className="comparison-section">
      <h3>Changed findings ({changed.length})</h3>
      {changed.length === 0 ? (
        <p className="comparison-empty">No changed findings.</p>
      ) : (
        <ul className="comparison-changed-list">
          {visibleChanges.map((entry) => (
            <li key={entry.fingerprint}>
              <code>{entry.ruleId}</code>
              <ul>
                {Object.entries(entry.changes).map(([field, change]) => (
                  <li key={field}>
                    {field}: <code>{change.before}</code> → <code>{change.after}</code>
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      )}
      {visibleCount < changed.length && (
        <div className="pagination-controls">
          <span>
            Showing {visibleChanges.length} of {changed.length}
          </span>
          <button className="text-button" type="button" onClick={() => setVisibleCount((count) => Math.min(count + COMPARISON_PAGE_SIZE, changed.length))}>
            Show more
          </button>
        </div>
      )}
    </div>
  );
}
