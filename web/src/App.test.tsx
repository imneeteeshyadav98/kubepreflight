import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import App from "./App";

const sampleDoc = {
  currentVersion: "1.32",
  targetVersion: "1.36",
  clusterContext: "kind-kubepreflight-demo",
  provider: "cluster-only",
  scannedAt: "2026-07-03T12:00:00Z",
  findings: [
    {
      ruleId: "PDB-001",
      severity: "Blocker",
      confidence: "STATIC_CERTAIN",
      message: "PDB blocks drain",
      resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "payments", name: "critical-pdb" }],
      evidence: ["disruptionsAllowed: 0"],
	  remediation: "Scale replicas.",
	  remediationDetail: {
		changes: [{ field: "disruptionsAllowed", current: "0", required: ">= 1" }],
		safeFix: { label: "Safe fix", steps: ["Inspect workload ownership."], command: "kubectl get pdb critical-pdb -n payments" },
		verifyCommand: "kubectl describe pdb critical-pdb -n payments",
		expectedResult: "Allowed disruptions >= 1",
	  },
      fingerprint: "fp-1",
    },
    {
      ruleId: "WH-001",
      severity: "Warning",
      confidence: "STATIC_CERTAIN",
      message: "Catch-all webhook",
      resources: [{ plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name: "guard" }],
      evidence: ["scope: apiGroups=[\"*\"]"],
      remediation: "Narrow scope.",
      fingerprint: "fp-2",
    },
  ],
};

const samplePlanDoc = {
  fromVersion: "1.29",
  toVersion: "1.31",
  generatedAt: "2026-07-05T00:00:00Z",
  hops: [
    {
      hop: { index: 1, from: "1.29", to: "1.30" },
      status: "EXACT",
      report: {
        findings: [
          {
            ruleId: "WH-002",
            severity: "Blocker",
            confidence: "STATIC_CERTAIN",
            message: "webhook is fail-closed with zero endpoints",
            resources: [{ plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name: "guard" }],
            evidence: [],
            remediation: "restore backend health",
            fingerprint: "fp-wh002",
            globalBlocker: true,
          },
        ],
        summary: { blockers: 1, warnings: 0, infos: 0 },
      },
    },
    {
      hop: { index: 2, from: "1.30", to: "1.31" },
      status: "PREDICTED",
      carryForward: [{ ruleId: "PDB-001", reason: "PDBs may be fixed before this hop is reached", recommendedCommand: "kubepreflight scan --target-version 1.31" }],
    },
  ],
};

const cleanDoc = {
  currentVersion: "1.35",
  targetVersion: "1.36",
  clusterContext: "clean-cluster",
  provider: "cluster-only",
  scannedAt: "2026-07-03T12:00:00Z",
  findings: [],
  summary: { blockers: 0, warnings: 0, infos: 0 },
};

function mockFetchSequence(responses: Array<{ ok: boolean; status?: number; body?: unknown }>) {
  let call = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(() => {
	  const response = responses[call] ?? { ok: false, status: 404 };
      call += 1;
      return Promise.resolve({
        ok: response.ok,
        status: response.status ?? (response.ok ? 200 : 404),
        text: () => Promise.resolve(JSON.stringify(response.body ?? {})),
      } as Response);
    }),
  );
}

function setLocation(search: string) {
  window.history.pushState({}, "", `/console/${search}`);
}

function findingsBody() {
  return document.getElementById("findings-body") as HTMLElement;
}

async function goToTab(user: ReturnType<typeof userEvent.setup>, name: RegExp) {
  await user.click(screen.getByRole("tab", { name }));
}

beforeEach(() => {
  setLocation("");
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("auto-load from location", () => {
  test("loads findings from an explicit ?findings= param", async () => {
    setLocation("?findings=/report-worst.json");
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.queryByText("Turn scan output into a decision surface.")).not.toBeInTheDocument();
  });

  test("shows current version and multi-minor upgrade path in the summary hero", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.getByText("This scan checks readiness for upgrading from 1.32 to 1.36.")).toBeInTheDocument();
    expect(screen.getByText("1.32")).toBeInTheDocument();
    expect(screen.getByText("1.32 → 1.33 → 1.34 → 1.35 → 1.36")).toBeInTheDocument();
    expect(screen.getByText("multi-minor upgrade path")).toBeInTheDocument();
  });

  test("shows advisory per-hop upgrade details on the Summary tab", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("Upgrade path details")).toBeInTheDocument());
    expect(screen.getByText("1.32 → 1.33")).toBeInTheDocument();
    expect(screen.getByText("1.35 → 1.36")).toBeInTheDocument();
    expect(screen.getByText("Planned, hop-specific scan recommended")).toBeInTheDocument();
    expect(screen.getAllByText("Planned, re-scan required").length).toBeGreaterThan(0);
    expect(screen.getByText(/Findings were evaluated against final target 1.36, not this individual hop/)).toBeInTheDocument();
    expect(screen.getByText("Show checks to review")).toBeInTheDocument();
    expect(screen.getByText(/Re-scan after each hop before treating the next hop as assessed/)).toBeInTheDocument();
    expect(screen.getByText(/1 blocker found — fix required before the change window/)).toBeInTheDocument();
    expect(screen.queryByText(/PDB and drain safety: 1 blocker/)).not.toBeInTheDocument();
  });

  test("shows unknown current version without inferring from kubelet evidence", async () => {
    mockFetchSequence([
      {
        ok: true,
        body: {
          ...sampleDoc,
          currentVersion: undefined,
          findings: [{ ...sampleDoc.findings[0], evidence: ["kubelet version: v1.32.2"] }],
        },
      },
    ]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.getByText("This scan checks readiness for target 1.36; current control-plane version is unknown.")).toBeInTheDocument();
    expect(screen.getByText("Unknown → 1.36")).toBeInTheDocument();
    expect(screen.getByText("current version unknown")).toBeInTheDocument();
    expect(screen.getByText("Current control-plane version was not available from the Kubernetes server version API. Node/kubelet versions are evaluated separately.")).toBeInTheDocument();
  });

  test("shows a readable error when an explicit ?findings= target 404s", async () => {
    setLocation("?findings=/does-not-exist.json");
    mockFetchSequence([{ ok: false, status: 404 }]);

    render(<App />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("/does-not-exist.json"));
    expect(screen.getByText("Turn scan output into a decision surface.")).toBeInTheDocument();
  });

  test("falls back to /findings.json when no ?findings= param is present", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(fetch).toHaveBeenCalledWith("/findings.json", expect.anything());
  });

  test("stays on the empty import state, without an error, when /findings.json is missing and no param was given", async () => {
    mockFetchSequence([{ ok: false, status: 404 }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("Turn scan output into a decision surface.")).toBeInTheDocument());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  test("lands on the Summary tab, not a specific finding, on load", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    expect(screen.getByRole("tab", { name: /Summary/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Top risks")).toBeInTheDocument();
  });
});

describe("error banner", () => {
  // Regression: the error banner used to live inside ImportPanel, which
  // React unmounts once a report is loaded — so re-importing a bad file
  // after a good one was already showing failed completely silently, with
  // no visible feedback anywhere on the page. The banner now renders at
  // the App level regardless of report state. Caught by the real-server
  // browser smoke test (web/tests/browser_smoke.py), not by mocked fetch.
  test("stays visible when a malformed file is imported after a report is already loaded", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    const file = new File(["{ not valid json"], "bad.json", { type: "application/json" });
    const input = document.getElementById("file-input") as HTMLInputElement;
    const user = userEvent.setup();
    await user.upload(input, file);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Invalid JSON"));
    // The previously loaded report must still be visible — a failed
    // re-import shouldn't blank out a working workspace.
    expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument();
  });
});

describe("import-panel affordances", () => {
  // Only reachable when no report is loaded yet — the real production
  // server always has a findings.json once a scan has run (Start()
  // requires it), so this state and these buttons are unreachable through
  // the actual reportserver integration. Covered here at the component
  // level instead of in web/tests/browser_smoke.py.
  test("bundled worst-case demo button loads the packaged demo report", async () => {
	  mockFetchSequence([{ ok: false, status: 404 }, { ok: false, status: 404 }, { ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("Turn scan output into a decision surface.")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Load worst-case demo" }));

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(fetch).toHaveBeenLastCalledWith("../demo/sample-output/findings.json", expect.anything());
  });

  test("clean-state preview button renders a synthetic CLEAN report with a success panel, not an empty table or tabs", async () => {
    mockFetchSequence([{ ok: false, status: 404 }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("Turn scan output into a decision surface.")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Preview clean state" }));

    await waitFor(() => expect(screen.getByText("payments-prod")).toBeInTheDocument());
    expect(screen.getByText("GO")).toBeInTheDocument();
    expect(screen.getByText("No blockers found")).toBeInTheDocument();
    // Zero findings: no tabs, nothing to switch between.
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  });
});

describe("decision strip", () => {
  test("shows NO-GO, the result badge, and a why-blocked line for a BLOCKED report", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    expect(screen.getByText("NO-GO")).toBeInTheDocument();
    expect(screen.getByText("BLOCKED")).toBeInTheDocument();
    expect(screen.getByText("1 blocker found — fix required before the change window.")).toBeInTheDocument();
  });
});

describe("incomplete coverage", () => {
	test("never renders the clean-state claim when evidence collection failed", async () => {
		const incomplete = { ...cleanDoc, coverage: { kubernetes: { status: "partial", errors: ["pods: forbidden"] } } };
		mockFetchSequence([{ ok: true, body: incomplete }, { ok: false, status: 404 }]);
		render(<App />);
		await waitFor(() => expect(screen.getByText("INCOMPLETE")).toBeInTheDocument());
		expect(screen.queryByText("No blockers found")).not.toBeInTheDocument();
		expect(screen.getByText("Assessment incomplete")).toBeInTheDocument();
	});
});

describe("single-page layout", () => {
  // The whole point of this pass: switching sections must not grow the
  // document. Only one tab panel is mounted at a time.
  test("only the active tab's content is mounted", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    // Summary tab: no findings table, no Evidence appendix.
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.queryByText("Evidence appendix")).not.toBeInTheDocument();

    const user = userEvent.setup();
    await goToTab(user, /Findings/);
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.queryByText("Top risks")).not.toBeInTheDocument();

    await goToTab(user, /Evidence/);
    expect(screen.getByText("Evidence appendix")).toBeInTheDocument();
    expect(screen.queryByLabelText("Finding filters")).not.toBeInTheDocument();
  });
});

describe("summary tab", () => {
  test("shows the highest-severity findings first in Top Risks, and clicking one opens it in the Findings tab detail pane", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const topRisks = document.getElementById("top-risks") as HTMLElement;
    expect(within(topRisks).getByText("PDB-001")).toBeInTheDocument();
    const upgradeDetails = document.querySelector(".upgrade-path-details") as HTMLElement;
    expect(topRisks.compareDocumentPosition(upgradeDetails) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    const user = userEvent.setup();
    await user.click(within(topRisks).getByText("PDB-001"));

    // Clicking a risk navigates to the Findings tab with that finding
    // selected in the inline detail pane — no modal.
    await waitFor(() => expect(screen.getByRole("tab", { name: /Findings/ })).toHaveAttribute("aria-selected", "true"));
    expect(document.getElementById("finding-detail")).toBeInTheDocument();
    expect(within(document.getElementById("finding-detail") as HTMLElement).getByText("PDB-001")).toBeInTheDocument();
  });

  test("shows a preview of the top 3 next actions with a link to the full list", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    expect(screen.getByText("Top next actions")).toBeInTheDocument();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /View all/ }));
    expect(screen.getByRole("tab", { name: /Next Actions/ })).toHaveAttribute("aria-selected", "true");
  });
});

describe("next actions tab", () => {
  test("groups actionable findings by severity and copies remediation per item", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Next Actions/);

    const actions = document.getElementById("actions") as HTMLElement;
    expect(within(actions).getByText("Blockers (1)")).toBeInTheDocument();
    expect(within(actions).getByText("Warnings (1)")).toBeInTheDocument();

    Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn().mockResolvedValue(undefined) }, configurable: true });
    const copyButtons = within(actions).getAllByRole("button", { name: "Copy" });
    await user.click(copyButtons[0]);
    expect(navigator.clipboard.writeText).toHaveBeenCalled();
  });

  // Guards the exact regression found in review: a group's "related"
  // sub-list must never re-list its own primary finding underneath itself.
  test("a group with only one finding shows no related sub-list at all", async () => {
    const soloFindingDoc = {
      ...sampleDoc,
      findings: [sampleDoc.findings[0]], // PDB-001 alone — no shared-resource group
    };
    mockFetchSequence([{ ok: true, body: soloFindingDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Next Actions/);

    const actions = document.getElementById("actions") as HTMLElement;
    expect(within(actions).queryByText(/PDB-001: /)).not.toBeInTheDocument();
    expect(actions.querySelector(".evidence-list")).toBeNull();
  });

  test("a group with a primary and two related findings shows only the two related findings, never the primary, with no duplicates", async () => {
    const guardResource = { plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name: "guard" };
    const groupedDoc = {
      ...sampleDoc,
      findings: [
        { ruleId: "WH-002", severity: "Blocker", confidence: "STATIC_CERTAIN", message: "webhook down", resources: [guardResource], evidence: [], remediation: "Restore backend health.", fingerprint: "fp-wh002" },
        { ruleId: "WH-001", severity: "Warning", confidence: "STATIC_CERTAIN", message: "catch-all scope", resources: [guardResource], evidence: [], remediation: "Narrow scope.", fingerprint: "fp-wh001" },
        { ruleId: "WH-003", severity: "Warning", confidence: "STATIC_CERTAIN", message: "no timeout set", resources: [guardResource], evidence: [], remediation: "Set a timeout.", fingerprint: "fp-wh003" },
      ],
    };
    mockFetchSequence([{ ok: true, body: groupedDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Next Actions/);

    const actions = document.getElementById("actions") as HTMLElement;
    const relatedList = actions.querySelector(".evidence-list") as HTMLElement;
    expect(relatedList).not.toBeNull();
    const relatedButtons = within(relatedList).getAllByRole("button");
    // Exactly the two related findings — never the primary (WH-002), and
    // never duplicated.
    expect(relatedButtons).toHaveLength(2);
    expect(relatedButtons.map((button) => button.textContent)).toEqual([
      expect.stringContaining("WH-001"),
      expect.stringContaining("WH-003"),
    ]);
    expect(within(relatedList).queryByText(/WH-002/)).not.toBeInTheDocument();
  });
});

describe("findings tab detail pane", () => {
  test("opening the tab auto-selects the highest-severity finding and keeps the compact list mobile-first", async () => {
    const warningFirst = { ...sampleDoc, findings: [sampleDoc.findings[1], sampleDoc.findings[0]] };
    mockFetchSequence([{ ok: true, body: warningFirst }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);

    await waitFor(() => expect(document.getElementById("dialog-rule")).toHaveTextContent("PDB-001"));
    expect(screen.queryByText("Select a finding from the list to see its evidence and remediation.")).not.toBeInTheDocument();
    expect(document.querySelector(".findings-list-pane")).not.toHaveClass("mobile-hidden");
    expect(document.querySelector(".findings-detail-pane")).toHaveClass("mobile-hidden");

    const table = screen.getByRole("table");
    expect(within(table).getAllByRole("columnheader")).toHaveLength(3);
    expect(within(table).queryByRole("columnheader", { name: "Confidence" })).not.toBeInTheDocument();
    expect(within(table).queryByRole("columnheader", { name: "Plane" })).not.toBeInTheDocument();
  });

	  test("selecting a row opens mobile detail, Back restores the list, and copy finding JSON still works", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);
    await waitFor(() => expect(document.getElementById("dialog-rule")).toHaveTextContent("PDB-001"));

    Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn().mockResolvedValue(undefined) }, configurable: true });
    await user.click(within(findingsBody()).getByText("WH-001"));

    await waitFor(() => expect(document.getElementById("finding-detail")).toBeInTheDocument());
    expect(document.querySelector(".findings-list-pane")).toHaveClass("mobile-hidden");
    expect(document.querySelector(".findings-detail-pane")).not.toHaveClass("mobile-hidden");
    await user.click(screen.getByRole("button", { name: "Copy finding JSON" }));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(expect.stringContaining("\"ruleId\": \"WH-001\""));
    await waitFor(() => expect(screen.getByRole("button", { name: "Copied" })).toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: /Back to list/ }));
    expect(document.querySelector(".findings-list-pane")).not.toHaveClass("mobile-hidden");
    expect(document.querySelector(".findings-detail-pane")).toHaveClass("mobile-hidden");
    expect(within(findingsBody()).getByRole("button", { name: "Open WH-001 details" })).toHaveAttribute("aria-selected", "true");
	  });

	  test("renders structured remediation with change, safe-fix, and verification blocks", async () => {
		mockFetchSequence([{ ok: true, body: sampleDoc }]);
		render(<App />);
		await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
		const user = userEvent.setup();
		await goToTab(user, /Findings/);
		await waitFor(() => expect(screen.getByText("Change required")).toBeInTheDocument());
		expect(screen.getByText("disruptionsAllowed")).toBeInTheDocument();
		expect(screen.getByText("Safe fix")).toBeInTheDocument();
		expect(screen.getByText("Expected: Allowed disruptions >= 1")).toBeInTheDocument();
	  });
});

describe("filters", () => {
  test("severity chips narrow the findings table without changing the summary counts", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);
    expect(screen.getByText("2 of 2 findings")).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: "Blocker" }));

    expect(screen.getByText("1 of 2 findings")).toBeInTheDocument();
    expect(within(findingsBody()).getByText("WH-001")).toBeInTheDocument();
    expect(within(findingsBody()).queryByText("PDB-001")).not.toBeInTheDocument();
    await waitFor(() => expect(document.getElementById("dialog-rule")).toHaveTextContent("WH-001"));
    // Summary cards reflect the whole report, not the filtered findings
    // table — both blocker and warning counts stay at 1 even though the
    // table itself now shows only the Warning finding.
    expect(screen.getByText("1", { selector: "#metric-blockers" })).toBeInTheDocument();
    expect(screen.getByText("1", { selector: "#metric-warnings" })).toBeInTheDocument();
  });

  test("deselecting every severity chip shows zero findings, not every finding", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);
    await user.click(screen.getByRole("checkbox", { name: "Blocker" }));
    await user.click(screen.getByRole("checkbox", { name: "Warning" }));
    await user.click(screen.getByRole("checkbox", { name: "Info" }));

    expect(screen.getByText("0 of 2 findings")).toBeInTheDocument();
  });

  test("resource/message search filters the table", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);
    await user.type(screen.getByLabelText("Search"), "critical-pdb");

    await waitFor(() => expect(screen.getByText("1 of 2 findings")).toBeInTheDocument());
    expect(within(findingsBody()).getByText("PDB-001")).toBeInTheDocument();
    expect(within(findingsBody()).queryByText("WH-001")).not.toBeInTheDocument();
  });

  test("clear filters button restores every severity chip and clears text filters", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);
    await user.type(screen.getByLabelText("Search"), "critical-pdb");
    await user.click(screen.getByRole("checkbox", { name: "Warning" }));
    await user.click(screen.getByRole("button", { name: "Clear filters" }));

    expect(screen.getByText("2 of 2 findings")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Blocker" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Warning" })).toBeChecked();
  });
});

describe("upgrade planner", () => {
  test("no Planner tab and no error when upgrade-plan.json is absent", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }, { ok: false, status: 404 }]);
    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.queryByRole("tab", { name: /Upgrade Planner/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  test("Planner tab appears and renders verdict/hop rows when a plan is present", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }, { ok: true, body: samplePlanDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await waitFor(() => expect(screen.getByRole("tab", { name: /Upgrade Planner/ })).toBeInTheDocument());
    await goToTab(user, /Upgrade Planner/);

    expect(screen.getByText("NOT READY FOR UPGRADE")).toBeInTheDocument();
    expect(screen.getByText("Global API write blocker detected")).toBeInTheDocument();
    expect(screen.getByText("Current live")).toBeInTheDocument();
    expect(screen.getByText("Projected")).toBeInTheDocument();
    // "Rescan required" appears both as a filter chip label and as the hop
    // badge — assert at least one instance rather than a single unique node.
    expect(screen.getAllByText("Rescan required").length).toBeGreaterThan(0);
    expect(screen.getByText(/PDB-001: PDBs may be fixed/)).toBeInTheDocument();
    expect(screen.getByText("GLOBAL API WRITE BLOCKER")).toBeInTheDocument();
  });

  test("future hops are collapsed by default with the projection caption", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }, { ok: true, body: samplePlanDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await waitFor(() => expect(screen.getByRole("tab", { name: /Upgrade Planner/ })).toBeInTheDocument());
    await goToTab(user, /Upgrade Planner/);

    const details = screen.getByText(/Future hops/).closest("details");
    expect(details).not.toBeNull();
    expect(details).not.toHaveAttribute("open");
    expect(screen.getByText(/Future-hop findings are projections/)).toBeInTheDocument();
  });

  test("a clean hop-1 report with a plan present still shows Tabs, not the clean-state panel", async () => {
    mockFetchSequence([{ ok: true, body: cleanDoc }, { ok: true, body: samplePlanDoc }]);
    render(<App />);

    await waitFor(() => expect(screen.getByText("clean-cluster")).toBeInTheDocument());
    expect(screen.getByRole("tablist")).toBeInTheDocument();
    expect(screen.queryByText("Turn scan output into a decision surface.")).not.toBeInTheDocument();
  });

  test("a clean hop-1 report with no plan still shows the clean-state panel, unchanged", async () => {
    mockFetchSequence([{ ok: true, body: cleanDoc }, { ok: false, status: 404 }]);
    render(<App />);

    await waitFor(() => expect(screen.getByText("clean-cluster")).toBeInTheDocument());
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });
});
