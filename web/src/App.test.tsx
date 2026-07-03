import { render, screen, waitFor } from "@testing-library/react";
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

  test("clean-state preview button renders a synthetic CLEAN report with no cluster contact", async () => {
    mockFetchSequence([{ ok: false, status: 404 }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("Turn scan output into a decision surface.")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Preview clean state" }));

    await waitFor(() => expect(screen.getByText("payments-prod")).toBeInTheDocument());
    expect(screen.getByText("0 of 0 findings")).toBeInTheDocument();
  });
});

describe("filters", () => {
  test("severity filter narrows the findings table without changing the summary counts", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    expect(screen.getByText("2 of 2 findings")).toBeInTheDocument();

    const user = userEvent.setup();
    await user.selectOptions(screen.getByLabelText("Severity"), "Warning");

    expect(screen.getByText("1 of 2 findings")).toBeInTheDocument();
    expect(screen.getByText("WH-001")).toBeInTheDocument();
    expect(screen.queryByText("PDB-001")).not.toBeInTheDocument();
    // Summary cards reflect the whole report, not the filtered findings
    // table — both blocker and warning counts stay at 1 even though the
    // table itself now shows only the Warning finding.
    expect(screen.getByText("1", { selector: "#metric-blockers" })).toBeInTheDocument();
    expect(screen.getByText("1", { selector: "#metric-warnings" })).toBeInTheDocument();
  });

  test("resource/message search filters the table", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Search"), "critical-pdb");

    await waitFor(() => expect(screen.getByText("1 of 2 findings")).toBeInTheDocument());
    expect(screen.getByText("PDB-001")).toBeInTheDocument();
    expect(screen.queryByText("WH-001")).not.toBeInTheDocument();
  });
});
