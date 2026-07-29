import { manifestOnlyCleanNotice, type Report } from "../lib/findings-schema";

interface CleanStatePanelProps {
  report: Report;
  onLoadDemo: () => void;
}

// Shown instead of the findings table when a *loaded* report has zero
// findings — distinct from the pre-load ImportPanel state. A blank table
// with "No findings match these filters" reads as broken; this reads as
// a result.
export default function CleanStatePanel({ report, onLoadDemo }: CleanStatePanelProps) {
  const manifestNotice = manifestOnlyCleanNotice(report);
  return (
    <section className="clean-state-panel" aria-label="No findings">
      <div className="clean-state-mark">✓</div>
      <h2>{manifestNotice ? "Manifest API checks clean" : "No blockers found"}</h2>
      <p>{manifestNotice || "This scan reported zero findings — nothing is blocking the upgrade."}</p>
      <button className="button button-secondary" onClick={onLoadDemo}>
        See a BLOCKED example instead
      </button>
    </section>
  );
}
