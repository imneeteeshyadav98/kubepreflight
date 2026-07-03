import { useEffect, useRef, useState } from "react";
import type { Finding } from "../lib/findings-schema";
import { resourceLabel } from "../lib/findings-schema";
import { copyToClipboard } from "../lib/clipboard";

interface FindingDialogProps {
  finding: Finding | null;
  onClose: () => void;
}

export default function FindingDialog({ finding, onClose }: FindingDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [copyLabel, setCopyLabel] = useState("Copy steps");
  const [copyJSONLabel, setCopyJSONLabel] = useState("Copy finding JSON");

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (finding) {
      setCopyLabel("Copy steps");
      setCopyJSONLabel("Copy finding JSON");
      if (!dialog.open) dialog.showModal();
    } else if (dialog.open) {
      dialog.close();
    }
  }, [finding]);

  async function copyRemediation() {
    if (!finding) return;
    setCopyLabel(await copyToClipboard(finding.remediation));
  }

  async function copyFindingJSON() {
    if (!finding) return;
    setCopyJSONLabel(await copyToClipboard(JSON.stringify(finding, null, 2)));
  }

  const planes = finding ? [...new Set(finding.resources.map((resource) => resource.plane))].join(" + ") : "";

  return (
    <dialog
      className="finding-dialog"
      id="finding-dialog"
      ref={dialogRef}
      onClick={(event) => {
        if (event.target === dialogRef.current) onClose();
      }}
      onClose={onClose}
    >
      <div className="dialog-shell">
        <header className="dialog-header">
          <div>
            <p className="eyebrow" id="dialog-rule">
              {finding?.ruleId ?? "Finding"}
            </p>
            <h2 id="dialog-title">{finding?.message ?? "—"}</h2>
          </div>
          <button className="icon-button" id="dialog-close" aria-label="Close finding details" onClick={onClose}>
            ×
          </button>
        </header>
        {finding && (
          <>
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
          </>
        )}
      </div>
    </dialog>
  );
}
