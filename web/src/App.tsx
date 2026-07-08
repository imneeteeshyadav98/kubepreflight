import { useCallback, useEffect, useState, type ChangeEvent } from "react";
import Header from "./components/Header";
import Sidebar from "./components/Sidebar";
import ImportPanel from "./components/ImportPanel";
import DecisionHero from "./components/DecisionHero";
import MetricsRow from "./components/MetricsRow";
import Tabs, { type TabKey } from "./components/Tabs";
import SummaryTab from "./components/SummaryTab";
import FindingsTab from "./components/FindingsTab";
import NextActionsTab from "./components/NextActionsTab";
import EvidenceTab from "./components/EvidenceTab";
import UpgradePlannerTab from "./components/UpgradePlannerTab";
import CleanStatePanel from "./components/CleanStatePanel";
import { parseFindingsDocument, type Finding, type Report } from "./lib/findings-schema";
import { parsePlanDocument, type PlanReport } from "./lib/plan-schema";
import { emptyFilters, type Filters } from "./types";
import { buildActionGroups } from "./lib/actions";

function cleanDemoDocument(): Record<string, unknown> {
  return {
    currentVersion: "1.35",
    targetVersion: "1.36",
    clusterContext: "payments-prod",
    provider: "eks",
    scannedAt: new Date().toISOString(),
    findings: [],
    summary: { blockers: 0, warnings: 0, infos: 0 },
    assumptions: ["Local preview data — no cluster was contacted."],
  };
}

export default function App() {
  const [report, setReport] = useState<Report | null>(null);
  const [raw, setRaw] = useState<unknown>(null);
  const [sourceName, setSourceName] = useState("");
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [selected, setSelected] = useState<Finding | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>("summary");
  const [error, setError] = useState<string | null>(null);
  const [planReport, setPlanReport] = useState<PlanReport | null>(null);
  const [planError, setPlanError] = useState<string | null>(null);

  const loadReport = useCallback((input: string, name: string) => {
    try {
      const parsedReport = parseFindingsDocument(input);
      setReport(parsedReport);
      setRaw(JSON.parse(input));
      setSourceName(name);
      setFilters(emptyFilters);
      setSelected(null);
      setActiveTab("summary");
      setError(null);
    } catch (err) {
      setError((err as Error).message);
    }
  }, []);

  // After a scan, `kubepreflight scan` starts a local server that prints a
  // Console URL with ?findings= pre-filled (internal/reportserver) so
  // opening it loads the just-completed scan automatically instead of
  // landing on the blank import screen. With no ?findings= param, we still
  // try the conventional /findings.json path the report server always
  // serves the current scan at; a 404 there is expected (no scan has run
  // yet, or the Console was opened by hand) and is not an error — unlike a
  // fetch/parse failure for an explicitly requested ?findings= path.
  useEffect(() => {
    const explicit = new URLSearchParams(location.search).get("findings");
    const candidate = explicit || "/findings.json";
    let cancelled = false;

    (async () => {
      try {
        const response = await fetch(candidate, { cache: "no-store" });
        if (!response.ok) {
          if (explicit && !cancelled) setError(`Could not load ${candidate}: HTTP ${response.status}`);
          return;
        }
        const text = await response.text();
        if (!cancelled) loadReport(text, candidate);
      } catch (err) {
        if (explicit && !cancelled) setError(`Could not load ${candidate}: ${(err as Error).message}`);
      }
    })();

    return () => {
      cancelled = true;
    };
    // Intentionally runs once on mount: the URL is read at load time, same
    // as the report server's printed link is meant to be opened fresh.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Independent of the findings effect above: upgrade-plan.json only
  // exists for a `plan` run, so this fetch is opportunistic the same way
  // — a 404 (no plan file) or a parse failure (e.g. the mocked/served body
  // isn't plan-shaped) is silently tolerated unless the probe was
  // explicit, and never affects the findings pipeline (no shared state
  // with loadReport, to avoid a mount-time race between the two fetches).
  useEffect(() => {
    const explicit = new URLSearchParams(location.search).get("plan");
    const candidate = explicit || "/upgrade-plan.json";
    let cancelled = false;

    (async () => {
      try {
        const response = await fetch(candidate, { cache: "no-store" });
		if (!response.ok) {
		  if ((explicit || response.status !== 404) && !cancelled) setPlanError(`Could not load ${candidate}: HTTP ${response.status}`);
          return;
        }
        const text = await response.text();
        if (cancelled) return;
        try {
          setPlanReport(parsePlanDocument(text));
		  setPlanError(null);
		} catch (err) {
		  setPlanError(`Could not load ${candidate}: ${(err as Error).message}`);
        }
      } catch (err) {
        if (explicit && !cancelled) setPlanError(`Could not load ${candidate}: ${(err as Error).message}`);
      }
    })();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A manually-imported findings.json (file upload, demo, clean-demo) has
  // no corresponding plan file, so these three manual entry points clear
  // any plan data from an earlier auto-loaded session. Deliberately NOT
  // done inside loadReport itself: the auto-load findings/plan fetches
  // both fire on mount with no ordering guarantee between them, so
  // clearing plan state there could wipe out plan data that the parallel
  // plan fetch already set.
  function clearPlan() {
    setPlanReport(null);
    setPlanError(null);
  }

  function handleFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    if (file.size > 10 * 1024 * 1024) {
      setError("File is larger than 10 MB. Use a scan-scoped findings.json.");
      event.target.value = "";
      return;
    }
    clearPlan();
    // FileReader rather than File.text(): more consistent across browsers
    // and test environments (jsdom's File polyfill doesn't implement
    // .text()).
    const reader = new FileReader();
    reader.onload = () => loadReport(String(reader.result), file.name);
    reader.onerror = () => setError(reader.error?.message ?? "Could not read the selected file.");
    reader.readAsText(file);
    event.target.value = "";
  }

  async function loadDemo() {
    clearPlan();
    try {
      const response = await fetch("../demo/sample-output/findings.json", { cache: "no-store" });
      if (!response.ok) throw new Error(`Demo returned HTTP ${response.status}`);
      loadReport(await response.text(), "demo/sample-output/findings.json");
    } catch (err) {
      setError(`Could not load the bundled demo. Serve the repository root, then open /web/: ${(err as Error).message}`);
    }
  }

  function loadClean() {
    clearPlan();
    loadReport(JSON.stringify(cleanDemoDocument()), "clean-demo.json");
  }

  function exportReport() {
    if (!raw) return;
    const blob = new Blob([JSON.stringify(raw, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = sourceName || "findings.json";
    anchor.click();
    URL.revokeObjectURL(url);
  }

  // Unified "look at this finding" action for every entry point (Top
  // Risks, Next Actions, the findings table itself): always land on the
  // Findings tab with the finding selected in the detail pane, rather
  // than maintaining a separate modal/dialog UI on top of the tabs.
  function openFinding(finding: Finding) {
    setSelected(finding);
    setActiveTab("findings");
  }

	const actionableCount = report ? buildActionGroups(report.findings).length : 0;

  return (
    <div className="app-shell">
      <Sidebar />
      <main id="top">
        <Header exportDisabled={!report} onFile={handleFile} onExport={exportReport} />

        {error && (
          <p className="error-message" id="error-message" role="alert">
            {error}
          </p>
        )}
        {planError && (
          <p className="error-message" id="plan-error-message" role="alert">
            {planError}
          </p>
        )}

        {!report && <ImportPanel onFile={handleFile} onLoadDemo={loadDemo} onLoadClean={loadClean} />}

        {report && (
          <div id="workspace" className="dashboard-shell">
            <DecisionHero report={report} />
            <MetricsRow report={report} />

			{report.result === "CLEAN" && report.findings.length === 0 && !planReport ? (
              <CleanStatePanel onLoadDemo={loadDemo} />
            ) : (
              <>
                <Tabs
                  active={activeTab}
                  onChange={setActiveTab}
                  findingsCount={report.findings.length}
                  actionsCount={actionableCount}
                  hasPlan={!!planReport}
                />
				<div className="tab-content" role="tabpanel" id={`panel-${activeTab}`} aria-labelledby={`tab-${activeTab}`}>
                  {activeTab === "summary" && <SummaryTab report={report} onOpenFinding={openFinding} onViewAllActions={() => setActiveTab("actions")} />}
                  {activeTab === "findings" && (
                    <FindingsTab
                      report={report}
                      filters={filters}
                      onFiltersChange={setFilters}
                      onReset={() => setFilters(emptyFilters)}
                      selected={selected}
                      onSelectFinding={setSelected}
                      onClearSelection={() => setSelected(null)}
                    />
                  )}
                  {activeTab === "actions" && <NextActionsTab report={report} onOpenFinding={openFinding} />}
                  {activeTab === "evidence" && <EvidenceTab report={report} />}
                  {activeTab === "planner" && planReport && <UpgradePlannerTab planReport={planReport} onOpenFinding={openFinding} />}
                </div>
              </>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
