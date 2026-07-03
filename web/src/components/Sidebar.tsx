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
      <nav aria-label="Primary navigation">
        <a className="nav-link" href="#summary">
          <span>01</span> Summary
        </a>
        <a className="nav-link" href="#findings">
          <span>02</span> Findings
        </a>
        <a className="nav-link" href="#actions">
          <span>03</span> Next actions
        </a>
      </nav>
      <div className="privacy-note">
        <span className="privacy-dot" />
        <strong>Local by design</strong>
        <p>Your JSON stays in this browser. No upload API, account, or telemetry.</p>
      </div>
    </aside>
  );
}
