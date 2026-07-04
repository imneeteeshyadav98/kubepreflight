import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import App from "./App";

const sampleDoc = {
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

function mockFetchSequence(responses: Array<{ ok: boolean; status?: number; body?: unknown }>) {
  let call = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(() => {
      const response = responses[Math.min(call, responses.length - 1)];
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
    mockFetchSequence([{ ok: false, status: 404 }, { ok: true, body: sampleDoc }]);
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
