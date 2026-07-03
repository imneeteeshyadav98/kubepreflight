import type { ChangeEvent } from "react";

interface ImportPanelProps {
  onFile: (event: ChangeEvent<HTMLInputElement>) => void;
  onLoadDemo: () => void;
  onLoadClean: () => void;
}

export default function ImportPanel({ onFile, onLoadDemo, onLoadClean }: ImportPanelProps) {
  return (
    <section className="import-panel" id="import-panel" aria-labelledby="import-heading">
      <div className="import-copy">
        <p className="eyebrow">Start here</p>
        <h2 id="import-heading">Turn scan output into a decision surface.</h2>
        <p>
          Drop a canonical <code>findings.json</code> from the CLI. The viewer runs entirely on your machine and keeps
          evidence attached to the finding that produced it.
        </p>
        <div className="import-actions">
          <label className="button button-primary" htmlFor="file-input-secondary">
            Choose findings.json
          </label>
          <input id="file-input-secondary" type="file" accept="application/json,.json" hidden onChange={onFile} />
          <button className="button button-secondary" id="load-demo-button" onClick={onLoadDemo}>
            Load worst-case demo
          </button>
          <button className="text-button" id="load-clean-button" onClick={onLoadClean}>
            Preview clean state
          </button>
        </div>
      </div>
      <div className="import-visual" aria-hidden="true">
        <div className="signal-ring">
          <span>
            READ
            <br />
            ONLY
          </span>
        </div>
        <div className="signal-line" />
        <div className="signal-card">
          <b>JSON</b>
          <span>evidence in</span>
        </div>
        <div className="signal-card accent">
          <b>GO / NO-GO</b>
          <span>decision out</span>
        </div>
      </div>
    </section>
  );
}
