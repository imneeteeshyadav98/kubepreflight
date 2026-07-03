import { useEffect, useState } from "react";
import type { Finding } from "../lib/findings-schema";
import { resourceLabel } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";

interface FindingDetailProps {
  finding: Finding | null;
  onBack?: () => void;
}

// Inline detail pane (Findings tab's right-hand side), not a modal — this
// used to be a <dialog> overlay; the command-center layout keeps list and
// detail on screen together instead of popping over the list. On mobile,
// list and detail can't fit side by side, so onBack (visible only there,
// see .back-to-list in styles.css) clears the selection to return to the
// list.
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
