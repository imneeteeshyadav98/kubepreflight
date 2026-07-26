import { useState } from "react";
import {
  RULE_EXECUTION_DISPLAY_STATES,
  ruleApplicabilityLabel,
  ruleExecutionCoverageSummary,
  ruleExecutionDisplayLabel,
  ruleExecutionDisplayState,
  ruleExecutionStateLabel,
  type Report,
  type RuleExecutionDisplayState,
} from "../lib/findings-schema";

interface RuleExecutionCoverageProps {
  report: Report;
}

// Evaluation Coverage: the "no finding does not imply clean" story made
// visible. Renders nothing at all for a report with no ruleExecutions field
// (pre-v1.3.0 findings.json, or hand-built demo data) — this section is
// purely additive and must never imply "fully evaluated" for data that
// simply predates this feature, nor clutter the Summary tab for every
// existing report that doesn't carry it.
export default function RuleExecutionCoverage({ report }: RuleExecutionCoverageProps) {
  const [activeStates, setActiveStates] = useState<RuleExecutionDisplayState[]>([]);
  const records = report.ruleExecutions;
  const summary = ruleExecutionCoverageSummary(report);

  if (!records || records.length === 0) return null;

  const filtered = activeStates.length === 0 ? records : records.filter((record) => activeStates.includes(ruleExecutionDisplayState(record)));

  function toggleState(state: RuleExecutionDisplayState) {
    setActiveStates((current) => (current.includes(state) ? current.filter((value) => value !== state) : [...current, state]));
  }

  return (
    <section className="rule-execution-panel" aria-label="Evaluation coverage">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Evaluation coverage</p>
          <h2>Rule Execution Coverage</h2>
        </div>
      </div>

      {summary.source === "normalized-legacy" && (
        <p className="assumptions normalized-legacy-banner" role="status" id="normalized-legacy-banner">
          <strong>Normalized legacy metadata</strong> — this report predates native rule-execution tracking. The evaluated / not
          evaluated states below were backfilled from finding presence after the fact, not recorded natively during the scan
          itself. Treat them as an inference, not this scan&apos;s own evidence — see each rule&apos;s Reason for detail.
        </p>
      )}

      <div className="table-wrap" tabIndex={0}>
        <table className="appendix rule-execution-summary">
          <thead>
            <tr>
              <th>Total rules</th>
              <th>Evaluated</th>
              <th>Not evaluated</th>
              <th>Insufficient evidence</th>
              <th>Failed</th>
              <th>Not applicable</th>
              <th>Metadata source</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td id="rule-execution-total">{summary.total}</td>
              <td id="rule-execution-evaluated">{summary.counts.evaluated}</td>
              <td id="rule-execution-not-evaluated">{summary.counts.not_evaluated}</td>
              <td id="rule-execution-insufficient-evidence">{summary.counts.insufficient_evidence}</td>
              <td id="rule-execution-failed">{summary.counts.failed}</td>
              <td id="rule-execution-not-applicable">{summary.counts.not_applicable}</td>
              <td>
                <span className={`eks-addon-status ${summary.source === "native" ? "clean" : "warn"}`} id="rule-execution-source">
                  {summary.source === "native" ? "Native" : "Normalized (legacy)"}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div className="severity-chips rule-state-filters" role="group" aria-label="Filter by execution state">
        {RULE_EXECUTION_DISPLAY_STATES.map((state) => {
          const active = activeStates.includes(state);
          return (
            <label key={state} className={`chip ${active ? "chip-active" : ""}`}>
              <input type="checkbox" checked={active} onChange={() => toggleState(state)} />
              {ruleExecutionDisplayLabel(state)} ({summary.counts[state]})
            </label>
          );
        })}
        {activeStates.length > 0 && (
          <button type="button" className="text-button filter-reset" onClick={() => setActiveStates([])}>
            Clear filters
          </button>
        )}
      </div>

      <div className="table-wrap" tabIndex={0}>
        <table className="appendix rule-execution-table" id="rule-execution-table">
          <thead>
            <tr>
              <th>Rule ID</th>
              <th>Applicability</th>
              <th>State</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((record) => (
              <tr key={record.ruleId}>
                <td>
                  <strong>{record.ruleId}</strong>
                </td>
                <td>{ruleApplicabilityLabel(record.applicability)}</td>
                <td>
                  <span className={`eks-addon-status ${ruleExecutionStateClass(ruleExecutionDisplayState(record))}`}>
                    {ruleExecutionStateLabel(record.state)}
                  </span>
                </td>
                <td>{record.reason || "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && <p className="empty-state">No rules match these filters.</p>}
      </div>
    </section>
  );
}

function ruleExecutionStateClass(state: RuleExecutionDisplayState): "clean" | "warn" | "blocked" | "info" {
  switch (state) {
    case "evaluated":
      return "clean";
    case "insufficient_evidence":
      return "warn";
    case "failed":
      return "blocked";
    case "not_evaluated":
    case "not_applicable":
    default:
      return "info";
  }
}
