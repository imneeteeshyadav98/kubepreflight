import { useCallback, useEffect, useMemo, useState, type ChangeEvent } from "react";
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
import ComparisonTab from "./components/ComparisonTab";
import CleanStatePanel from "./components/CleanStatePanel";
import { parseFindingsDocument, type Finding, type Report } from "./lib/findings-schema";
import { parsePlanDocument, type PlanReport } from "./lib/plan-schema";
import { compareReports } from "./lib/comparison-schema";
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

// worstCaseDemoDocument is entirely self-contained client-side data — no
// fetch, no dependency on a committed or locally-generated file — so
// "Load worst-case demo" always works, in the shipped product as much as
// in a repo checkout. Spans P1 through P4 (see internal/findings/
// priority.go for the real mapping this mirrors) specifically so a first-
// time viewer sees the priority engine's actual value, not just that
// findings exist.
function worstCaseDemoDocument(): Record<string, unknown> {
  return {
    currentVersion: "1.29",
    targetVersion: "1.32",
    clusterContext: "payments-prod",
    provider: "eks",
    scannedAt: new Date().toISOString(),
    assumptions: ["Local preview data — no cluster was contacted."],
    summary: { blockers: 3, warnings: 1, infos: 0 },
    findings: [
      {
        ruleId: "WH-002",
        severity: "Blocker",
        confidence: "OBSERVED",
        message: `ValidatingWebhookConfiguration "checkout-guard" is fail-closed with a catch-all apiGroups/resources scope and zero ready backend endpoints — matching API writes will be rejected`,
        evidence: ["webhook index: 0", "ready endpoint address count: 0", "failurePolicy: Fail"],
        remediation: "Restore the webhook backend, then verify ready endpoints before removing any temporary mitigation.",
        fingerprint: "demo-worst-case-wh-002",
        resources: [{ plane: "live", kind: "ValidatingWebhookConfiguration", scope: "cluster", namespace: "", name: "checkout-guard" }],
        globalBlocker: true,
        priority: "P1",
        priorityReason: "May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.",
        affectedScope: "global",
        canUpgradeContinue: false,
      },
      {
        ruleId: "API-001",
        severity: "Blocker",
        confidence: "STATIC_CERTAIN",
        message: `PodSecurityPolicy "payments-restricted" (apiVersion policy/v1beta1) still exists at a version removed in Kubernetes 1.25 — target version 1.32 will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright`,
        evidence: ["apiVersion: policy/v1beta1", "removed in: Kubernetes 1.25", "target version: 1.32"],
        remediation: "Migrate to Pod Security Admission or a policy engine (Kyverno/Gatekeeper) before upgrading past 1.25.",
        fingerprint: "demo-worst-case-api-001",
        resources: [{ plane: "live", kind: "PodSecurityPolicy", scope: "cluster", namespace: "", name: "payments-restricted" }],
        priority: "P2",
        priorityReason: "Resource or behavior may fail after the target Kubernetes upgrade.",
        affectedScope: "workload",
        canUpgradeContinue: false,
      },
      {
        ruleId: "PDB-001",
        severity: "Blocker",
        confidence: "OBSERVED",
        message: `PodDisruptionBudget payments-prod/checkout-pdb: disruptionsAllowed=0 (minAvailable: 3, currentHealthy: 3) — matching pods cannot currently be voluntarily evicted, so a node drain or node upgrade can stall or fail`,
        evidence: ["disruptionsAllowed: 0", "minAvailable: 3", "currentHealthy: 3"],
        remediation: "Scale up replicas to create eviction headroom, or temporarily relax this PodDisruptionBudget for the change window.",
        fingerprint: "demo-worst-case-pdb-001",
        resources: [{ plane: "live", kind: "PodDisruptionBudget", scope: "namespaced", namespace: "payments-prod", name: "checkout-pdb" }],
        priority: "P3",
        priorityReason: "Node drain may fail during maintenance or a managed node group upgrade.",
        affectedScope: "workload",
        canUpgradeContinue: false,
      },
      {
        ruleId: "WORKLOAD-001",
        severity: "Warning",
        confidence: "OBSERVED",
        message: "Workload has unhealthy pods before upgrade: 1 pod in CrashLoopBackOff. This workload was unhealthy before the upgrade, which can make post-upgrade validation ambiguous.",
        evidence: ["namespace: payments-prod", "pod: checkout-worker-6b9f8c-2x4lp", "reason: CrashLoopBackOff"],
        remediation: "Inspect the unhealthy workload before the upgrade — confirm whether this predates the change window or needs a fix first.",
        fingerprint: "demo-worst-case-workload-001",
        resources: [{ plane: "live", kind: "Pod", scope: "namespaced", namespace: "payments-prod", name: "checkout-worker-6b9f8c-2x4lp" }],
        priority: "P4",
        priorityReason: "Upgrade should not begin while workloads, nodes, or critical add-ons are unhealthy.",
        affectedScope: "workload",
        canUpgradeContinue: true,
      },
    ],
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
  const [baselineReport, setBaselineReport] = useState<Report | null>(null);
  const [baselineName, setBaselineName] = useState("");
  const [baselineError, setBaselineError] = useState<string | null>(null);

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

  // A baseline is only meaningful relative to the "current" report it was
  // uploaded to compare against — loading a different current report (new
  // file, demo, clean state) makes any earlier baseline stale, the same
  // reasoning clearPlan already applies to plan data.
  function clearBaseline() {
    setBaselineReport(null);
    setBaselineName("");
    setBaselineError(null);
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
    clearBaseline();
    // FileReader rather than File.text(): more consistent across browsers
    // and test environments (jsdom's File polyfill doesn't implement
    // .text()).
    const reader = new FileReader();
    reader.onload = () => loadReport(String(reader.result), file.name);
    reader.onerror = () => setError(reader.error?.message ?? "Could not read the selected file.");
    reader.readAsText(file);
    event.target.value = "";
  }

  function handleBaselineFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    if (file.size > 10 * 1024 * 1024) {
      setBaselineError("File is larger than 10 MB. Use a scan-scoped findings.json.");
      event.target.value = "";
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      try {
        setBaselineReport(parseFindingsDocument(String(reader.result)));
        setBaselineName(file.name);
        setBaselineError(null);
      } catch (err) {
        setBaselineError((err as Error).message);
      }
    };
    reader.onerror = () => setBaselineError(reader.error?.message ?? "Could not read the selected file.");
    reader.readAsText(file);
    event.target.value = "";
  }

  function loadDemo() {
    clearPlan();
    clearBaseline();
    loadReport(JSON.stringify(worstCaseDemoDocument()), "worst-case-demo.json");
  }

  function loadClean() {
    clearPlan();
    clearBaseline();
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

  function openEvidence(finding: Finding) {
    setSelected(finding);
    setActiveTab("evidence");
  }

  // Upgrade Readiness scorecard rule-ID chips: switch to Findings and
  // reuse the existing free-text search filter verbatim (search already
  // matches on ruleId, see filterFindings) rather than inventing a new
  // rule-ID filter dimension just for this one entry point.
  function jumpToRule(ruleId: string) {
    setFilters({ ...emptyFilters, search: ruleId });
    setActiveTab("findings");
  }

  // compareReports can throw (duplicate/missing fingerprint) -- caught here
  // rather than in the file handler, since the same recomputation must also
  // happen if `report` itself changes while a baseline is already loaded.
  const { comparison, comparisonError } = useMemo(() => {
    if (!report || !baselineReport) return { comparison: null, comparisonError: null };
    try {
      return { comparison: compareReports(baselineReport, report), comparisonError: null };
    } catch (err) {
      return { comparison: null, comparisonError: (err as Error).message };
    }
  }, [report, baselineReport]);

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
                  {activeTab === "summary" && <SummaryTab report={report} onOpenFinding={openFinding} onViewEvidence={openEvidence} onViewAllActions={() => setActiveTab("actions")} onJumpToRule={jumpToRule} />}
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
                  {activeTab === "evidence" && <EvidenceTab report={report} selected={selected} />}
                  {activeTab === "planner" && planReport && <UpgradePlannerTab planReport={planReport} onOpenFinding={openFinding} />}
                  {activeTab === "compare" && (
                    <ComparisonTab
                      report={report}
                      baselineName={baselineName}
                      comparison={comparison}
                      error={baselineError ?? comparisonError}
                      onBaselineFile={handleBaselineFile}
                      onClearBaseline={clearBaseline}
                      onOpenFinding={openFinding}
                    />
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
