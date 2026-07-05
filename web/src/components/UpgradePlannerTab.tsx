import { useMemo, useState } from "react";
import { decisionFromResult, findingResourceLabel, type Finding, type Report } from "../lib/findings-schema";
import { planVerdict, type HopReport, type PlanReport } from "../lib/plan-schema";

interface UpgradePlannerTabProps {
  planReport: PlanReport;
  onOpenFinding: (finding: Finding) => void;
}

type HopFilter = "current" | "all";
type FindingFilter = "all" | "global-blockers" | "manifest" | "live";

function verdictClass(label: string): string {
  if (label === "NOT READY FOR UPGRADE") return "blocked";
  if (label === "CONDITIONALLY READY") return "warning";
  return "clean";
}

function resultBadgeClass(result: string): string {
  const decision = decisionFromResult(result as Report["result"]);
  return decision === "NO-GO" ? "blocked" : decision === "GO" ? "clean" : "warning";
}

// Provenance resolves the one real ambiguity in "why does this finding
// appear here": CarryForwardNote/HopReport carry no manifest-vs-live
// category field, so provenance is derived from data that actually
// exists — a hop's own HopStatus, and each finding's resource planes.
function findingProvenance(finding: Finding, isExactHop: boolean): string {
  if (!isExactHop) return "Projected — re-run KubePreflight to confirm";
  const planes = new Set(finding.resources.map((resource) => resource.plane));
  if (planes.has("live")) return "Current live observation";
  if (planes.has("manifest")) return "Manifest static issue";
  return "Current live observation";
}

function findingRow(finding: Finding, isExactHop: boolean, onOpenFinding: (finding: Finding) => void) {
  return (
    <li key={finding.fingerprint} className="planner-finding-row" onClick={() => onOpenFinding(finding)}>
      <span className={`severity-pill ${finding.severity.toLowerCase()}`}>{finding.severity}</span>
      {finding.globalBlocker && <span className="global-blocker-badge">GLOBAL API WRITE BLOCKER</span>}
      <strong>{findingResourceLabel(finding)}</strong>
      <span className="finding-provenance">{findingProvenance(finding, isExactHop)}</span>
    </li>
  );
}

// hopRow is the compact Upgrade Path overview — a teaser only. When a hop
// has carry-forward notes, this shows just the "Rescan required" badge;
// the full reasons live exclusively in the expanded "Future hops" section
// below, so the same text never renders twice in the DOM.
function hopRow(hop: HopReport) {
  const carryForward = hop.carryForward ?? [];
  return (
    <li key={`${hop.hop.from}-${hop.hop.to}`} className="hop-row">
      <span className="hop-versions">
        {hop.hop.from} &rarr; {hop.hop.to}
      </span>
      <span className={`badge-${hop.status === "EXACT" ? "current-live" : "projected"}`}>
        {hop.status === "EXACT" ? "Current live" : "Projected"}
      </span>
      {hop.status === "EXACT" && hop.report && (
        <span className={`result-badge ${resultBadgeClass(hop.report.result)}`}>{hop.report.result}</span>
      )}
      {hop.status === "PREDICTED" && hop.report && (
        <span className="hop-counts">
          {hop.report.summary.blockers} blocker(s), {hop.report.summary.warnings} warning(s)
        </span>
      )}
      {carryForward.length > 0 && <span className="badge-rescan-required">Rescan required</span>}
    </li>
  );
}

// Console mirror of the HTML report's Upgrade Path section (commit A):
// verdict banner, one row per hop with Current-live/Projected/
// Rescan-required badges, hop 1 shown expanded, future hops collapsed.
export default function UpgradePlannerTab({ planReport, onOpenFinding }: UpgradePlannerTabProps) {
  const [hopFilter, setHopFilter] = useState<HopFilter>("all");
  const [rescanOnly, setRescanOnly] = useState(false);
  const [findingFilter, setFindingFilter] = useState<FindingFilter>("all");

  const hop1 = planReport.hops[0];
  const futureHops = planReport.hops.slice(1);
  const verdict = planVerdict(hop1?.report);
  const globalBlockerCount = (hop1?.report?.findings ?? []).filter((finding) => finding.globalBlocker).length;

  const visibleHops = useMemo(() => {
    let hops = hopFilter === "current" ? planReport.hops.slice(0, 1) : planReport.hops;
    if (rescanOnly) hops = hops.filter((hop) => (hop.carryForward?.length ?? 0) > 0);
    return hops;
  }, [hopFilter, rescanOnly, planReport.hops]);

  const hop1Findings = useMemo(() => {
    const findings = hop1?.report?.findings ?? [];
    if (findingFilter === "global-blockers") return findings.filter((finding) => finding.globalBlocker);
    if (findingFilter === "manifest") return findings.filter((finding) => finding.resources.some((resource) => resource.plane === "manifest"));
    if (findingFilter === "live") return findings.filter((finding) => finding.resources.some((resource) => resource.plane === "live"));
    return findings;
  }, [hop1, findingFilter]);

  return (
    <div className="tab-panel planner-tab">
      <section className={`plan-verdict-banner ${verdictClass(verdict.label)}`}>
        <h2>{verdict.label}</h2>
        <p>{verdict.reason}</p>
      </section>

      <section className="plan-overview">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Next actionable hop</p>
            <h2>
              {hop1 ? (
                <>
                  {hop1.hop.from} &rarr; {hop1.hop.to}
                </>
              ) : (
                "—"
              )}
            </h2>
          </div>
        </div>
        <dl className="decision-meta">
          <div>
            <dt>Provider</dt>
            <dd>{planReport.provider || "cluster-only"}</dd>
          </div>
          <div>
            <dt>Cluster</dt>
            <dd>{planReport.clusterContext || "Unspecified cluster"}</dd>
          </div>
          <div>
            <dt>From</dt>
            <dd>{planReport.fromVersion}</dd>
          </div>
          <div>
            <dt>To</dt>
            <dd>{planReport.toVersion}</dd>
          </div>
          {hop1?.report && (
            <>
              <div>
                <dt>Blockers</dt>
                <dd>{hop1.report.summary.blockers}</dd>
              </div>
              <div>
                <dt>Warnings</dt>
                <dd>{hop1.report.summary.warnings}</dd>
              </div>
              <div>
                <dt>Global blockers</dt>
                <dd>{globalBlockerCount}</dd>
              </div>
            </>
          )}
        </dl>
      </section>

      <section className="upgrade-path">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Upgrade path</p>
            <h2>
              {planReport.fromVersion} &rarr; {planReport.toVersion}
            </h2>
          </div>
        </div>
        <div className="filters" aria-label="Planner hop filters">
          <label className={`chip ${hopFilter === "current" ? "chip-active" : ""}`}>
            <input type="radio" name="hop-filter" checked={hopFilter === "current"} onChange={() => setHopFilter("current")} />
            Current hop only
          </label>
          <label className={`chip ${hopFilter === "all" ? "chip-active" : ""}`}>
            <input type="radio" name="hop-filter" checked={hopFilter === "all"} onChange={() => setHopFilter("all")} />
            All hops
          </label>
          <label className={`chip ${rescanOnly ? "chip-active" : ""}`}>
            <input type="checkbox" checked={rescanOnly} onChange={() => setRescanOnly((value) => !value)} />
            Rescan required
          </label>
        </div>
        <ol className="upgrade-path-list">{visibleHops.map((hop) => hopRow(hop))}</ol>
      </section>

      {hop1?.report && (
        <section className="next-hop-findings">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Next actionable hop findings</p>
              <h2>
                {hop1.hop.from} &rarr; {hop1.hop.to}
              </h2>
            </div>
          </div>
          <div className="filters" aria-label="Planner finding filters">
            <label className={`chip ${findingFilter === "all" ? "chip-active" : ""}`}>
              <input type="radio" name="finding-filter" checked={findingFilter === "all"} onChange={() => setFindingFilter("all")} />
              All findings
            </label>
            <label className={`chip ${findingFilter === "global-blockers" ? "chip-active" : ""}`}>
              <input
                type="radio"
                name="finding-filter"
                checked={findingFilter === "global-blockers"}
                onChange={() => setFindingFilter("global-blockers")}
              />
              Global blockers
            </label>
            <label className={`chip ${findingFilter === "manifest" ? "chip-active" : ""}`}>
              <input type="radio" name="finding-filter" checked={findingFilter === "manifest"} onChange={() => setFindingFilter("manifest")} />
              Manifest carry-forward
            </label>
            <label className={`chip ${findingFilter === "live" ? "chip-active" : ""}`}>
              <input type="radio" name="finding-filter" checked={findingFilter === "live"} onChange={() => setFindingFilter("live")} />
              Live-state findings
            </label>
          </div>
          <ul className="planner-finding-list">
            {hop1Findings.length === 0 ? (
              <li className="empty-state">No findings match these filters.</li>
            ) : (
              hop1Findings.map((finding) => findingRow(finding, true, onOpenFinding))
            )}
          </ul>
        </section>
      )}

      {futureHops.length > 0 && (
        <details className="future-hops">
          <summary>Future hops ({futureHops.length})</summary>
          <p className="upgrade-path-caption">
            Future-hop findings are projections. Re-run KubePreflight after each completed upgrade step.
          </p>
          {futureHops.map((hop) => (
            <div key={`${hop.hop.from}-${hop.hop.to}`} className="future-hop-detail">
              <h3>
                {hop.hop.from} &rarr; {hop.hop.to}
              </h3>
              {hop.report && hop.report.findings.length > 0 && (
                <ul className="planner-finding-list">{hop.report.findings.map((finding) => findingRow(finding, false, onOpenFinding))}</ul>
              )}
              {(hop.carryForward?.length ?? 0) > 0 && (
                <ul className="carry-forward-list">
                  {(hop.carryForward ?? []).map((note, index) => (
                    <li key={index}>
                      {note.ruleId}: {note.reason} — carried forward from previous hop, rescan required
                    </li>
                  ))}
                </ul>
              )}
            </div>
          ))}
        </details>
      )}
    </div>
  );
}
