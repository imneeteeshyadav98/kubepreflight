import { useEffect, useState } from "react";
import type { Finding, RemediationAction } from "../lib/findings-schema";
import { resourceLabel } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";

interface FindingDetailProps {
  finding: Finding | null;
  onBack?: () => void;
}

function ActionPanel({ action, className = "" }: { action?: RemediationAction; className?: string }) {
  const [copyLabel, setCopyLabel] = useState("Copy command");
  if (!action) return null;
  return (
    <div className={`remediation-section ${action.risky ? "emergency-panel" : ""} ${className}`}>
      <div className="section-inline"><h3>{action.label}</h3>{action.command && <button className="text-button" onClick={async () => setCopyLabel(await copyToClipboard(action.command || ""))}>{copyLabel}</button>}</div>
      {action.steps && <ul className="evidence-list">{action.steps.map((step, index) => <li key={index}>{step}</li>)}</ul>}
      {action.command && <pre>{action.command}</pre>}
    </div>
  );
}

function CopyBlock({ title, text }: { title: string; text: string }) {
  const [label, setLabel] = useState("Copy");
  return <div className="remediation-section"><div className="section-inline"><h3>{title}</h3><button className="text-button" onClick={async () => setLabel(await copyToClipboard(text))}>{label}</button></div><pre>{text}</pre></div>;
}

// Inline detail pane (Findings tab's right-hand side), not a modal — this
// used to be a <dialog> overlay; the command-center layout keeps list and
// detail on screen together instead of popping over the list. On mobile,
// list and detail can't fit side by side, so onBack (visible only there,
// see .back-to-list in styles.css) returns to the list while preserving
// the selected row.
export default function FindingDetail({ finding, onBack }: FindingDetailProps) {
  const [copyLabel, setCopyLabel] = useState("Copy steps");
  const [copyJSONLabel, setCopyJSONLabel] = useState("Copy finding JSON");

  useEffect(() => {
    setCopyLabel("Copy steps");
    setCopyJSONLabel("Copy finding JSON");
  }, [finding]);

  if (!finding) {
    return (
      <div className="finding-detail-empty" id="finding-detail-empty">
        Select a finding from the list to see its evidence and remediation.
      </div>
    );
  }

  async function copyRemediation() {
    if (!finding) return;
    setCopyLabel(await copyToClipboard(finding.remediation));
  }

  async function copyFindingJSON() {
    if (!finding) return;
    setCopyJSONLabel(await copyToClipboard(JSON.stringify(finding, null, 2)));
  }

  const planes = [...new Set(finding.resources.map((resource) => resource.plane))].join(" + ");

  return (
    <div className="finding-detail" id="finding-detail">
      {onBack && (
        <button type="button" className="back-to-list" onClick={onBack}>
          ← Back to list
        </button>
      )}
      <header className="finding-detail-header">
        <p className="eyebrow" id="dialog-rule">
          {finding.ruleId}
        </p>
        <h2 id="dialog-title">{finding.message}</h2>
      </header>
      <div className="dialog-badges" id="dialog-badges">
        <span className={`severity-pill ${finding.severity.toLowerCase()}`}>{finding.severity}</span>
        <span className="confidence-pill">{finding.confidence}</span>
        <span className="plane-pill">{planes}</span>
      </div>
      <section>
        <h3>Resources</h3>
        <div id="dialog-resources" className="resource-stack">
          {finding.resources.map((resource, index) => (
            <div className="resource-card" key={index}>
              <span className="plane-pill">{resource.plane}</span>
              <div>
                <strong>{resourceLabel(resource)}</strong>
                <small>{resource.uid || resource.sourcePath || resource.providerId || "No occurrence ID"}</small>
              </div>
            </div>
          ))}
        </div>
      </section>
      <section>
        <h3>Evidence</h3>
        <ul id="dialog-evidence" className="evidence-list">
          {finding.evidence.length ? finding.evidence.map((item, index) => <li key={index}>{item}</li>) : <li>No evidence supplied.</li>}
        </ul>
      </section>
      <section className="remediation-section">
        <div className="section-inline">
          <h3>Remediation</h3>
          <button className="text-button" id="copy-remediation" onClick={copyRemediation}>
            {copyLabel}
          </button>
        </div>
        <pre id="dialog-remediation">{finding.remediation}</pre>
      </section>
	  {finding.remediationDetail && (
		<section aria-label="Structured remediation">
		  {finding.remediationDetail.changes && finding.remediationDetail.changes.length > 0 && <div className="change-required"><h3>Change required</h3>{finding.remediationDetail.changes.map((change, index) => <div className="change-row" key={index}><span>{change.field}</span><span>{change.current}</span><span>→</span><span>{change.required}</span></div>)}</div>}
		  {finding.remediationDetail.diff && <CopyBlock title="Suggested diff" text={finding.remediationDetail.diff} />}
		  <ActionPanel action={finding.remediationDetail.safeFix} />
		  <ActionPanel action={finding.remediationDetail.emergency} />
		  <ActionPanel action={finding.remediationDetail.breakGlass} className="breakglass-panel" />
		  {finding.remediationDetail.verifyCommand && <><CopyBlock title="Verify" text={finding.remediationDetail.verifyCommand} />{finding.remediationDetail.expectedResult && <p>Expected: {finding.remediationDetail.expectedResult}</p>}</>}
		</section>
	  )}
      <footer className="dialog-footer">
        <div>
          <span>Fingerprint</span>
          <code id="dialog-fingerprint">{finding.fingerprint}</code>
        </div>
        <button className="text-button" id="copy-finding-json" onClick={copyFindingJSON}>
          {copyJSONLabel}
        </button>
      </footer>
    </div>
  );
}
