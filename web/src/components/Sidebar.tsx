// Section navigation now lives in the Tabs (see Tabs.tsx) inside the
// dashboard itself — this sidebar is branding + the privacy note only.
export default function Sidebar() {
  return (
    <aside className="sidebar">
      <a className="brand" href="#top" aria-label="KubePreflight Console home">
        <span className="brand-mark">KP</span>
        <span>
          <strong>KubePreflight</strong>
          <small>Console</small>
        </span>
      </a>
      <div className="privacy-note">
        <span className="privacy-dot" />
        <strong>Local by design</strong>
        <p>Your JSON stays in this browser. No upload API, account, or telemetry.</p>
      </div>
    </aside>
  );
}
