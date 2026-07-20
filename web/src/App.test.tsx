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

function largeFindingsDoc(count: number) {
  return {
    ...sampleDoc,
    findings: Array.from({ length: count }, (_, index) => ({
      ruleId: index % 2 === 0 ? "PDB-001" : "WH-001",
      severity: index % 5 === 0 ? "Blocker" : "Warning",
      confidence: "STATIC_CERTAIN",
      message: `Synthetic large report finding workload-${index}`,
      resources: [{ plane: "live", kind: "Deployment", namespace: `ns-${index % 20}`, name: `workload-${index}` }],
      evidence: [`index: ${index}`],
      remediation: "Review workload configuration.",
      fingerprint: `large-fp-${index}`,
    })),
  };
}

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

function textById(id: string): string {
  return document.getElementById(id)?.textContent ?? "";
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

  test("renders cluster-only provider and disabled AWS enrichment with operator-friendly labels", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(textById("provider-name")).toBe("Cluster-only");
    expect(textById("aws-enrichment")).toBe("Off");
  });

  test("renders API compatibility scorecard in the summary tab", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        findings: [{
          ...sampleDoc.findings[0],
          ruleId: "API-001",
          priority: "P2",
          message: "PodSecurityPolicy restricted uses removed API policy/v1beta1",
          evidence: ["apiVersion: policy/v1beta1"],
          resources: [{ plane: "manifest", kind: "PodSecurityPolicy", namespace: "", name: "restricted", scope: "cluster", sourcePath: "manifests/psp.yaml" }],
          fingerprint: "fp-api-001",
        }],
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    const scorecard = screen.getByRole("region", { name: "Kubernetes API compatibility" });
    expect(within(scorecard).getByText("Failed")).toBeInTheDocument();
    expect(within(scorecard).getByText("policy/v1beta1")).toBeInTheDocument();
    expect(within(scorecard).getByText("PodSecurityPolicy")).toBeInTheDocument();
    expect(within(scorecard).getByText("PodSecurityPolicy/restricted")).toBeInTheDocument();
  });

  test("renders EKS provider and enabled AWS enrichment with operator-friendly labels", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        coverage: { aws: { status: "complete", errors: [] } },
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(textById("provider-name")).toBe("EKS");
    expect(textById("aws-enrichment")).toBe("On");
  });

  test("missing provider and AWS enrichment values stay safe", async () => {
    const docWithoutProvider: Record<string, unknown> = { ...sampleDoc };
    delete docWithoutProvider.provider;
    mockFetchSequence([{ ok: true, body: docWithoutProvider }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(textById("provider-name")).toBe("Cluster-only");
    expect(textById("aws-enrichment")).toBe("Off");
  });

  test("shows EKS cluster metadata chips when eksCluster is present", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksCluster: {
          clusterName: "prod-cluster",
          region: "ap-south-1",
          version: "1.32",
          platformVersion: "eks.5",
          status: "ACTIVE",
          supportType: "EXTENDED",
          endpointAccess: "public",
        },
      },
    }]);

    render(<App />);

    // clusterDisplayName prefers eksCluster.clusterName ("prod-cluster")
    // over clusterContext ("kind-kubepreflight-demo") once eksCluster is
    // present -- it's the exact --cluster-name value the operator passed,
    // more authoritative than whatever the kubeconfig context happens to
    // be named.
    await waitFor(() => expect(screen.getByText("prod-cluster")).toBeInTheDocument());
    expect(screen.getByText("ap-south-1")).toBeInTheDocument();
    expect(screen.getByText("eks.5")).toBeInTheDocument();
    expect(screen.getByText("ACTIVE")).toBeInTheDocument();
    expect(screen.getByText("Extended support")).toBeInTheDocument();
    expect(screen.getByText("Public")).toBeInTheDocument();
  });

  test("shows a short cluster name with a copy button for a full EKS ARN, not the raw ARN inline", async () => {
    const arn = "arn:aws:eks:eu-north-1:123456789012:cluster/exciting-dance-outfit";
    mockFetchSequence([{
      ok: true,
      body: { ...sampleDoc, clusterContext: arn, provider: "eks", eksCluster: { clusterName: "exciting-dance-outfit", arn } },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("exciting-dance-outfit")).toBeInTheDocument());
    expect(screen.queryByText(arn)).not.toBeInTheDocument();
    expect(document.getElementById("cluster-name")).toHaveAttribute("title", arn);

    const user = userEvent.setup();
    Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn().mockResolvedValue(undefined) }, configurable: true });
    await user.click(screen.getByRole("button", { name: "Copy ARN" }));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(arn);
  });

  test("shows a plain cluster name with no copy button when there is nothing beyond it to offer", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "Copy ARN" })).not.toBeInTheDocument();
    expect(document.getElementById("cluster-name")).not.toHaveAttribute("title");
  });

  test("hides EKS cluster metadata chips for a cluster-only scan", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.queryByText("Region")).not.toBeInTheDocument();
    expect(screen.queryByText("Platform version")).not.toBeInTheDocument();
    expect(screen.queryByText("EKS status")).not.toBeInTheDocument();
  });

  test("shows the EKS add-on inventory table with all three status states", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksAddons: [
          { name: "vpc-cni", currentVersion: "v1.18.1-eksbuild.1", compatibleVersions: ["v1.18.1-eksbuild.1"], compatible: true },
          { name: "coredns", currentVersion: "v1.10.1-eksbuild.1", compatibleVersions: ["v1.11.0-eksbuild.1"], compatible: false },
          { name: "kube-proxy", currentVersion: "v1.29.0-eksbuild.1", compatible: false, verificationUnavailable: true },
        ],
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("EKS add-ons")).toBeInTheDocument());
    expect(screen.getByText("vpc-cni")).toBeInTheDocument();
    expect(screen.getByText("Compatible")).toBeInTheDocument();
    expect(screen.getByText("coredns")).toBeInTheDocument();
    expect(screen.getByText("Needs update")).toBeInTheDocument();
    expect(screen.getByText("kube-proxy")).toBeInTheDocument();
    expect(screen.getByText("Verification unavailable")).toBeInTheDocument();
  });

  test("hides the EKS add-on inventory table for a cluster-only scan", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());
    expect(screen.queryByText("EKS add-ons")).not.toBeInTheDocument();
  });

  test("shows Priority pills and sorts the Findings tab by Priority ahead of rule ID", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        findings: [
          { ...sampleDoc.findings[0], ruleId: "PDB-001", priority: "P3", priorityReason: "Node drain may fail during maintenance or a managed node group upgrade.", affectedScope: "workload", canUpgradeContinue: false },
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
            priority: "P1",
            priorityReason: "May block kubectl apply/patch/scale, Helm upgrades, and controller reconciliation.",
            affectedScope: "global",
            canUpgradeContinue: false,
          },
        ],
      },
    }]);

    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: /findings/i }));

    // The finding rows are plain <tr> (implicit "row" role) — no role
    // override, so aria-selected stays valid ARIA (see FindingsTab.tsx).
    const rows = within(findingsBody()).getAllByRole("row");
    expect(within(rows[0]).getByText("P1")).toBeInTheDocument();
    expect(within(rows[0]).getByText("WH-002")).toBeInTheDocument();
    expect(within(rows[1]).getByText("P3")).toBeInTheDocument();
    expect(within(rows[1]).getByText("PDB-001")).toBeInTheDocument();

    // Selecting the P1 finding shows the full priority detail block.
    await user.click(rows[0]);
    await waitFor(() => expect(document.getElementById("dialog-priority")).toBeInTheDocument());
    expect(document.getElementById("dialog-priority")).toHaveTextContent("Can upgrade continue: No");
    expect(document.getElementById("dialog-priority")).toHaveTextContent("Affected scope: global");

    // Blocker-severity findings that are not global blockers must not
    // imply the upgrade can continue from the detail panel.
    await user.click(rows[1]);
    await waitFor(() => expect(document.getElementById("dialog-rule")).toHaveTextContent("PDB-001"));
    expect(document.getElementById("dialog-priority")).toHaveTextContent("Can upgrade continue: No");
    expect(document.getElementById("dialog-priority")).toHaveTextContent("Affected scope: workload");
  });

  test("shows the P1-P4 priority legend near Top Risks and in the Findings tab", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const legend = "Priority ranks upgrade urgency: P1 = fix now, P2 = fix before upgrade, P3 = fix before drain/maintenance, P4 = stabilize before starting.";
    expect(screen.getByText(legend)).toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: /findings/i }));
    expect(screen.getByText(legend)).toBeInTheDocument();
  });

  test("hides the priority legend when there are no Top Risks (clean report)", async () => {
    mockFetchSequence([{ ok: true, body: cleanDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("clean-cluster")).toBeInTheDocument());
    expect(screen.queryByText(/Priority ranks upgrade urgency/)).not.toBeInTheDocument();
  });

  test("shows EKS managed node group inventory and empty-state scope wording", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksCluster: { clusterName: "prod", status: "ACTIVE" },
        eksNodegroups: [{
          name: "ng-app",
          status: "ACTIVE",
          version: "1.32",
          releaseVersion: "1.32.7-20260601",
          amiType: "AL2023_x86_64_STANDARD",
          capacityType: "ON_DEMAND",
          desiredSize: 3,
          minSize: 3,
          maxSize: 8,
          maxUnavailable: 1,
          healthIssues: [{ code: "AccessDenied" }],
          readinessStatus: "Review required",
        }],
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("EKS managed node groups")).toBeInTheDocument());
    expect(screen.getByText("ng-app")).toBeInTheDocument();
    expect(screen.getByText("1.32.7-20260601")).toBeInTheDocument();
    expect(screen.getByText("3 / 3 / 8")).toBeInTheDocument();
    expect(screen.getByText("maxUnavailable: 1")).toBeInTheDocument();
    expect(screen.getByText("AccessDenied")).toBeInTheDocument();
    expect(screen.getByText("Review required")).toBeInTheDocument();
  });

  test("shows no-managed-nodegroups explanation for EKS inventory with no node groups", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksCluster: { clusterName: "prod", status: "ACTIVE" },
        eksNodegroups: [],
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByText("EKS managed node groups")).toBeInTheDocument());
    expect(screen.getByText("No EKS managed node groups found. Self-managed nodes are not listed by the EKS ListNodegroups API.")).toBeInTheDocument();
  });

  test("shows EKS Upgrade Insights inventory, empty state, and unavailable state", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksCluster: { clusterName: "prod", status: "ACTIVE" },
        eksUpgradeInsights: [{
          id: "insight-1",
          name: "Deprecated API usage",
          category: "UPGRADE_READINESS",
          status: "PASSING",
          kubernetesVersion: "1.34",
          lastRefreshTime: "2026-06-01T00:00:00Z",
          recommendation: "No action required.",
          deprecationDetails: ["usage: policy/v1beta1/podsecuritypolicies"],
        }],
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByRole("heading", { name: "EKS Upgrade Insights" })).toBeInTheDocument());
    expect(screen.getByText("AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.")).toBeInTheDocument();
    expect(screen.getByText("Deprecated API usage")).toBeInTheDocument();
    expect(screen.getByText("PASSING")).toBeInTheDocument();
    expect(screen.getByText("No action required.")).toBeInTheDocument();
  });

  test("shows no-insights explanation for EKS inventory with no upgrade insights", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksCluster: { clusterName: "prod", status: "ACTIVE" },
        eksUpgradeInsights: [],
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByRole("heading", { name: "EKS Upgrade Insights" })).toBeInTheDocument());
    expect(screen.getByText("No EKS upgrade insights returned.")).toBeInTheDocument();
  });

  test("shows unavailable explanation when EKS Upgrade Insights collection failed", async () => {
    mockFetchSequence([{
      ok: true,
      body: {
        ...sampleDoc,
        provider: "eks",
        eksCluster: { clusterName: "prod", status: "ACTIVE" },
        coverage: { aws: { status: "partial", errors: ["list-insights: access denied"] } },
      },
    }]);

    render(<App />);

    await waitFor(() => expect(screen.getByRole("heading", { name: "EKS Upgrade Insights" })).toBeInTheDocument());
    expect(screen.getByText("EKS Upgrade Insights unavailable. Kubernetes findings are still valid.")).toBeInTheDocument();
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

describe("Compare tab", () => {
  // fp-1 (PDB-001) is Warning here but Blocker in sampleDoc -> Changed.
  // fp-2 (WH-001) doesn't exist here -> shows as New against sampleDoc.
  // fp-3 (NODE-001) doesn't exist in sampleDoc -> shows as Resolved.
  const baselineDoc = {
    currentVersion: "1.32",
    targetVersion: "1.36",
    clusterContext: "kind-kubepreflight-demo",
    provider: "cluster-only",
    scannedAt: "2026-06-01T12:00:00Z",
    findings: [
      {
        ruleId: "PDB-001",
        severity: "Warning",
        confidence: "STATIC_CERTAIN",
        message: "PDB blocks drain",
        resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "payments", name: "critical-pdb" }],
        evidence: ["disruptionsAllowed: 0"],
        remediation: "Scale replicas.",
        fingerprint: "fp-1",
      },
      {
        ruleId: "NODE-001",
        severity: "Blocker",
        confidence: "STATIC_CERTAIN",
        message: "kubelet skew outside supported policy",
        resources: [{ plane: "live", kind: "Node", namespace: "", name: "node-1" }],
        evidence: ["skew: 5"],
        remediation: "Upgrade the node.",
        fingerprint: "fp-3",
      },
    ],
  };

  test("uploading a baseline renders new/resolved/changed counts and a navigable rule chip", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: /Compare/ }));
    expect(screen.getByText(/Upload a baseline findings\.json/)).toBeInTheDocument();

    const file = new File([JSON.stringify(baselineDoc)], "baseline.json", { type: "application/json" });
    const input = document.getElementById("baseline-file-input") as HTMLInputElement;
    await user.upload(input, file);

    await waitFor(() => expect(screen.getByText("baseline.json")).toBeInTheDocument());
    // New: WH-001 (Warning severity) -> 1 new finding, 0 of them blockers.
    expect(document.getElementById("comparison-new-count")).toHaveTextContent("1 (0 blocker(s))");
    // Resolved: NODE-001 (Blocker severity, baseline-only) -> 1 resolved blocker.
    expect(document.getElementById("comparison-resolved-count")).toHaveTextContent("1 (1 blocker(s))");
    // Changed: PDB-001 is Warning in baseline, Blocker in current.
    expect(document.getElementById("comparison-changed-count")).toHaveTextContent("1");

    // The New finding (WH-001) renders as a clickable rule-ID chip that
    // jumps to the Findings tab, same as the Upgrade Readiness scorecard's
    // chips already do.
    const panel = document.getElementById("comparison-panel") as HTMLElement;
    await user.click(within(panel).getByRole("button", { name: "WH-001" }));
    await waitFor(() => expect(screen.getByRole("tab", { name: /Findings/ })).toHaveAttribute("aria-selected", "true"));
  });

  test("large comparisons keep unchanged finding rows bounded", async () => {
    const largeDoc = largeFindingsDoc(1000);
    mockFetchSequence([{ ok: true, body: largeDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: /Compare/ }));
    await user.upload(document.getElementById("baseline-file-input") as HTMLInputElement, new File([JSON.stringify(largeDoc)], "large-baseline.json", { type: "application/json" }));

    await waitFor(() => expect(document.getElementById("comparison-unchanged-count")).toHaveTextContent("1000"));
    expect(document.querySelectorAll(".comparison-findings-table tbody tr")).toHaveLength(250);
    expect(screen.getByText("Showing 250 of 1000")).toBeInTheDocument();

    await user.click(screen.getByText(/Unchanged findings/));
    await user.click(screen.getByRole("button", { name: "Show more" }));
    expect(document.querySelectorAll(".comparison-findings-table tbody tr")).toHaveLength(500);
    expect(screen.getByText("Showing 500 of 1000")).toBeInTheDocument();
  });

  test("changing the current report clears the loaded baseline", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("tab", { name: /Compare/ }));
    const file = new File([JSON.stringify(baselineDoc)], "baseline.json", { type: "application/json" });
    await user.upload(document.getElementById("baseline-file-input") as HTMLInputElement, file);
    await waitFor(() => expect(screen.getByText("baseline.json")).toBeInTheDocument());

    // Re-importing a new current report (via the header's file input) must
    // drop the now-stale baseline, the same way it already drops plan data.
    // Non-empty findings, otherwise the app renders CleanStatePanel instead
    // of the Tabs and there'd be no Compare tab to click at all.
    const newDoc = { ...sampleDoc, clusterContext: "second-cluster" };
    const newFile = new File([JSON.stringify(newDoc)], "second.json", { type: "application/json" });
    await user.upload(document.getElementById("file-input") as HTMLInputElement, newFile);
    await waitFor(() => expect(screen.getByText("second-cluster")).toBeInTheDocument());

    await user.click(screen.getByRole("tab", { name: /Compare/ }));
    expect(screen.getByText(/Upload a baseline findings\.json/)).toBeInTheDocument();
    expect(screen.queryByText("baseline.json")).not.toBeInTheDocument();
  });
});

describe("import-panel affordances", () => {
  // Only reachable when no report is loaded yet — the real production
  // server always has a findings.json once a scan has run (Start()
  // requires it), so this state and these buttons are unreachable through
  // the actual reportserver integration. Covered here at the component
  // level instead of in web/tests/browser_smoke.py.
  test("worst-case demo button loads a self-contained synthetic report with no fetch", async () => {
    // worstCaseDemoDocument() is inline client-side data (App.tsx) — unlike
    // the old bundled demo/sample-output/findings.json fetch this replaced,
    // it has no file dependency, so it works identically in the shipped
    // product and in a repo checkout. The two 404s here are the findings.json/
    // upgrade-plan.json auto-load probes on initial mount, not anything the
    // button click itself triggers.
    mockFetchSequence([{ ok: false, status: 404 }, { ok: false, status: 404 }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("Turn scan output into a decision surface.")).toBeInTheDocument());

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Load worst-case demo" }));

    await waitFor(() => expect(screen.getByText("payments-prod")).toBeInTheDocument());
    expect(screen.getByText("NO-GO")).toBeInTheDocument();
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
    expect(screen.getByText("BLOCKED", { selector: "#result-badge" })).toBeInTheDocument();
    expect(screen.getByText("1 blocker found — fix required before the change window.")).toBeInTheDocument();
  });
});

describe("incomplete coverage", () => {
	test("never renders the clean-state claim when evidence collection failed", async () => {
		const incomplete = { ...cleanDoc, coverage: { kubernetes: { status: "partial", errors: ["pods: forbidden"] } } };
		mockFetchSequence([{ ok: true, body: incomplete }, { ok: false, status: 404 }]);
		render(<App />);
		await waitFor(() => expect(screen.getByText("INCOMPLETE", { selector: "#result-badge" })).toBeInTheDocument());
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

    // Summary tab: no Findings/Evidence tab content.
    expect(document.getElementById("findings-body")).not.toBeInTheDocument();
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
  test("renders the operator Start Here flow and read-only Top Risk action rail", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const startHere = screen.getByRole("region", { name: "Start here" });
    expect(startHere).toBeInTheDocument();
    expect(within(startHere).getByRole("button", { name: "Inspect the PDB, then create eviction headroom." })).toBeInTheDocument();
    expect(screen.getByText("Upgrade gate checklist")).toBeInTheDocument();

    const topRisks = document.getElementById("top-risks") as HTMLElement;
    const pdbCard = within(topRisks).getByRole("button", { name: "Open PDB-001 details" }).closest("article") as HTMLElement;
    expect(within(pdbCard).getByText("Inspect current state first. This does not change the cluster.")).toBeInTheDocument();
    expect(within(pdbCard).getByText("kubectl get pdb critical-pdb -n payments")).toBeInTheDocument();
    expect(within(pdbCard).getByRole("button", { name: "Copy inspect command" })).toBeInTheDocument();
    expect(within(pdbCard).getByRole("button", { name: "View full finding" })).toBeInTheDocument();
    expect(within(pdbCard).getByRole("button", { name: "View evidence" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Run Fix/i })).not.toBeInTheDocument();
  });

  test("Top Risk rail can jump straight to the selected evidence row", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    const topRisks = document.getElementById("top-risks") as HTMLElement;
    const pdbCard = within(topRisks).getByRole("button", { name: "Open PDB-001 details" }).closest("article") as HTMLElement;
    await user.click(within(pdbCard).getByRole("button", { name: "View evidence" }));

    await waitFor(() => expect(screen.getByRole("tab", { name: /Evidence/ })).toHaveAttribute("aria-selected", "true"));
    const evidence = document.getElementById("evidence-appendix") as HTMLElement;
    expect(within(evidence).getByText("PDB-001").closest("tr")).toHaveClass("row-selected");
  });

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

  test("Upgrade Readiness scorecard renders per-category status and a rule-ID chip jumps to Findings filtered by that rule", async () => {
    mockFetchSequence([{ ok: true, body: sampleDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const scorecard = document.querySelector(".upgrade-readiness-panel") as HTMLElement;
    expect(scorecard).toBeInTheDocument();
    // sampleDoc has a Blocker PDB-001 and a Warning WH-001 — Disruption
    // Safety must read Failed, Admission Webhooks Warning, and every other
    // category (e.g. Node Readiness, with no matching finding) Passed.
    expect(within(scorecard).getByText("Disruption Safety").closest("tr")).toHaveTextContent("Failed");
    expect(within(scorecard).getByText("Admission Webhooks").closest("tr")).toHaveTextContent("Warning");
    expect(within(scorecard).getByText("Node Readiness").closest("tr")).toHaveTextContent("Passed");

    const user = userEvent.setup();
    await user.click(within(scorecard).getByRole("button", { name: "WH-001" }));

    await waitFor(() => expect(screen.getByRole("tab", { name: /Findings/ })).toHaveAttribute("aria-selected", "true"));
    expect((document.getElementById("search-filter") as HTMLInputElement).value).toBe("WH-001");
    const visibleRows = document.querySelectorAll("#findings-body tr:not(.hidden)");
    expect(visibleRows).toHaveLength(1);
    expect(visibleRows[0]).toHaveTextContent("WH-001");
  });

  test("same-version scan shows cluster-health framing instead of upgrade-continue language", async () => {
    const sameVersionDoc = { ...sampleDoc, currentVersion: "1.36", targetVersion: "1.36" };
    mockFetchSequence([{ ok: true, body: sameVersionDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    expect(screen.getAllByText(/no version upgrade is being assessed/i).length).toBeGreaterThan(0);

    const readinessPanel = document.querySelector(".upgrade-readiness-panel") as HTMLElement;
    expect(within(readinessPanel).getByText("Cluster Health (no version upgrade assessed)")).toBeInTheDocument();
    expect(within(readinessPanel).getByText("Remediation needed")).toBeInTheDocument();
    expect(within(readinessPanel).queryByText("Upgrade continue")).not.toBeInTheDocument();

    const apiPanel = document.querySelector(".api-compatibility-panel") as HTMLElement;
    expect(within(apiPanel).getByText("Remediation needed")).toBeInTheDocument();
    expect(within(apiPanel).queryByText("Upgrade continue")).not.toBeInTheDocument();

    expect(screen.getByText("Recommended maintenance")).toBeInTheDocument();
    expect(screen.queryByText("Top next actions")).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /View all/ }));
    expect(screen.getByRole("tab", { name: /Next Actions/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("heading", { name: "Recommended maintenance" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Next actions" })).not.toBeInTheDocument();
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

  // Regression for the Console v1 accessibility audit: the card used to be
  // a whole <article role="button"> wrapping the Copy button and related-
  // finding buttons as real, ARIA-illegal nested-interactive descendants.
  // The primary "open this finding" region is now a real <button> sibling
  // to the controls, so this also confirms removing the stopPropagation
  // workaround didn't reintroduce cross-triggering between them.
  test("the card's primary region and a related finding's button open their own finding independently", async () => {
    const guardResource = { plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name: "guard" };
    const groupedDoc = {
      ...sampleDoc,
      findings: [
        { ruleId: "WH-002", severity: "Blocker", confidence: "STATIC_CERTAIN", message: "webhook down", resources: [guardResource], evidence: [], remediation: "Restore backend health.", fingerprint: "fp-wh002" },
        { ruleId: "WH-001", severity: "Warning", confidence: "STATIC_CERTAIN", message: "catch-all scope", resources: [guardResource], evidence: [], remediation: "Narrow scope.", fingerprint: "fp-wh001" },
      ],
    };
    mockFetchSequence([{ ok: true, body: groupedDoc }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Next Actions/);

    const actions = document.getElementById("actions") as HTMLElement;
    const primaryButton = actions.querySelector(".action-item-primary") as HTMLElement;
    expect(primaryButton).not.toBeNull();
    expect(primaryButton.tagName).toBe("BUTTON");
    // The controls (Copy, related findings) must never be descendants of
    // the primary button — that's exactly the nested-interactive defect.
    expect(primaryButton.querySelector("button")).toBeNull();

    await user.click(primaryButton);
    await waitFor(() => expect(screen.getByRole("tab", { name: /Findings/ })).toHaveAttribute("aria-selected", "true"));
    await waitFor(() => expect(document.getElementById("dialog-rule")).toHaveTextContent("WH-002"));

    await user.click(screen.getByRole("tab", { name: /Next Actions/ }));
    const relatedList = document.getElementById("actions")!.querySelector(".evidence-list") as HTMLElement;
    await user.click(within(relatedList).getByRole("button", { name: /WH-001/ }));
    await waitFor(() => expect(document.getElementById("dialog-rule")).toHaveTextContent("WH-001"));
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
    expect(within(table).getAllByRole("columnheader")).toHaveLength(4);
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
    expect(within(findingsBody()).getByRole("row", { name: "Open WH-001 details" })).toHaveAttribute("aria-selected", "true");
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
  test("large findings lists keep the initial DOM bounded while filters still cover the full report", async () => {
    mockFetchSequence([{ ok: true, body: largeFindingsDoc(1000) }]);
    render(<App />);
    await waitFor(() => expect(screen.getByText("kind-kubepreflight-demo")).toBeInTheDocument());

    const user = userEvent.setup();
    await goToTab(user, /Findings/);

    await waitFor(() => expect(screen.getByText("1000 of 1000 findings")).toBeInTheDocument());
    expect(findingsBody().querySelectorAll("tr")).toHaveLength(250);
    expect(screen.getByText("Showing 250 of 1000")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Show more" }));
    expect(findingsBody().querySelectorAll("tr")).toHaveLength(500);
    expect(screen.getByText("Showing 500 of 1000")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Search"), "workload-999");

    await waitFor(() => expect(screen.getByText("1 of 1000 findings")).toBeInTheDocument());
    expect(findingsBody().querySelectorAll("tr")).toHaveLength(1);
    expect(within(findingsBody()).getByText("WH-001")).toBeInTheDocument();
    expect(within(findingsBody()).getByText("Synthetic large report finding workload-999")).toBeInTheDocument();
  });

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
