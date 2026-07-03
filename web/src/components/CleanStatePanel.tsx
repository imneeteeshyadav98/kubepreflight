interface CleanStatePanelProps {
  onLoadDemo: () => void;
}

// Shown instead of the findings table when a *loaded* report has zero
// findings — distinct from the pre-load ImportPanel state. A blank table
// with "No findings match these filters" reads as broken; this reads as
// a result.
export default function CleanStatePanel({ onLoadDemo }: CleanStatePanelProps) {
  return (
    <section className="clean-state-panel" id="findings" aria-label="No findings">
      <div className="clean-state-mark">✓</div>
      <h2>No blockers found</h2>
      <p>This scan reported zero findings — nothing is blocking the upgrade.</p>
      <button className="button button-secondary" onClick={onLoadDemo}>
        See a BLOCKED example instead
      </button>
    </section>
  );
}
