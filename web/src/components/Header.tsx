import type { ChangeEvent } from "react";

interface HeaderProps {
  exportDisabled: boolean;
  onFile: (event: ChangeEvent<HTMLInputElement>) => void;
  onExport: () => void;
}

export default function Header({ exportDisabled, onFile, onExport }: HeaderProps) {
  return (
    <header className="topbar">
      <div>
        <p className="eyebrow">Upgrade readiness workspace</p>
        <h1>KubePreflight Console</h1>
      </div>
      <div className="topbar-actions">
        <button className="button button-ghost" id="export-button" disabled={exportDisabled} onClick={onExport}>
          Export JSON
        </button>
        <label className="button button-primary" htmlFor="file-input">
          Import findings.json
        </label>
        <input id="file-input" type="file" accept="application/json,.json" hidden onChange={onFile} />
      </div>
    </header>
  );
}
