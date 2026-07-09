import { describe, expect, test } from "vitest";
import { compareFindings, eksAddonStatus, eksEndpointAccessLabel, eksNodegroupHealthLabel, eksNodegroupReadinessClass, eksSupportTypeLabel, eksUpgradeInsightDetails, eksUpgradeInsightStatusClass, filterFindings, parseFindingsDocument, priorityPillClass, priorityRank, resultFromSummary, topRisks, upgradeContext, upgradeDetails, type Finding } from "./findings-schema";

const baseFinding: Finding = {
  ruleId: "PDB-001",
  severity: "Blocker",
  confidence: "STATIC_CERTAIN",
  message: "PDB blocks drain",
  resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "payments", name: "critical-pdb" }],
  evidence: ["disruptionsAllowed: 0"],
  remediation: "Scale replicas.",
  fingerprint: "fp-1",
};

test("parses canonical findings and derives the decision", () => {
  const report = parseFindingsDocument({ targetVersion: "1.36", findings: [baseFinding], summary: { blockers: 999 } });
  expect(report.summary.blockers).toBe(1);
  expect(report.result).toBe("BLOCKED");
});

test("accepts the legacy singular resource shape", () => {
  const { resources, ...withoutResources } = baseFinding;
  const report = parseFindingsDocument({ findings: [{ ...withoutResources, resource: resources[0] }] });
  expect(report.findings[0].resources[0].name).toBe("critical-pdb");
});

test("rejects malformed documents instead of rendering partial data", () => {
  expect(() => parseFindingsDocument({ findings: [{ ...baseFinding, resources: [] }] })).toThrow(/no resources/);
  expect(() => parseFindingsDocument("not json")).toThrow(/Invalid JSON/);
});

test("filters by namespace, confidence, severity, and search", () => {
  const findings = parseFindingsDocument({
    findings: [
      baseFinding,
      { ...baseFinding, ruleId: "WH-001", severity: "Warning", fingerprint: "fp-2", resources: [{ plane: "live", kind: "Webhook", name: "guard", namespace: "" }] },
    ],
  }).findings;
  expect(filterFindings(findings, { namespace: "payments" }).length).toBe(1);
  expect(filterFindings(findings, { severities: ["Warning"] })[0].ruleId).toBe("WH-001");
  expect(filterFindings(findings, { severities: [] })).toHaveLength(0);
  expect(filterFindings(findings, { confidence: "STATIC_CERTAIN" }).length).toBe(2);
  expect(filterFindings(findings, { search: "critical-pdb" })[0].ruleId).toBe("PDB-001");
});

test("maps summaries to stable result labels", () => {
  expect(resultFromSummary({ blockers: 0, warnings: 0, infos: 0 })).toBe("CLEAN");
  expect(resultFromSummary({ blockers: 0, warnings: 1, infos: 0 })).toBe("PASSED_WITH_WARNINGS");
});

test("partial coverage produces an incomplete result without inventing findings", () => {
  const report = parseFindingsDocument({ findings: [], coverage: { kubernetes: { status: "partial", errors: ["pods: forbidden"] } } });
  expect(report.result).toBe("INCOMPLETE");
  expect(report.schemaVersion).toBe("legacy");
});

test("normalizes current version and builds one-minor upgrade context", () => {
  const report = parseFindingsDocument({ currentVersion: "v1.29.6-eks-1234567", targetVersion: "1.30", findings: [baseFinding] });
  expect(report.currentVersion).toBe("1.29");
  expect(upgradeContext(report)).toMatchObject({
    path: "1.29 → 1.30",
    label: "one-minor upgrade",
    line: "This scan checks readiness for upgrading from 1.29 to 1.30.",
  });
});

test("builds multi-minor upgrade context", () => {
  const report = parseFindingsDocument({ currentVersion: "1.32", targetVersion: "1.36", findings: [baseFinding] });
  expect(upgradeContext(report)).toMatchObject({
    path: "1.32 → 1.33 → 1.34 → 1.35 → 1.36",
    label: "multi-minor upgrade path",
  });
});

test("keeps current version unknown when absent", () => {
  const report = parseFindingsDocument({ targetVersion: "1.36", findings: [baseFinding] });
  expect(report.currentVersion).toBe("Unknown");
  expect(upgradeContext(report)).toMatchObject({
    path: "Unknown → 1.36",
    label: "current version unknown",
  });
});

test("builds single-hop upgrade details from current findings", () => {
  const report = parseFindingsDocument({
    currentVersion: "1.29",
    targetVersion: "1.30",
    findings: [baseFinding],
  });
  const details = upgradeDetails(report);
  expect(details).toHaveLength(1);
  expect(details[0]).toMatchObject({
    from: "1.29",
    to: "1.30",
    statusLabel: "Blocked",
    statusClass: "blocked",
  });
  expect(details[0].findingLines).toContain("PDB and drain safety: 1 blocker(s) (PDB-001)");
  expect(details[0].checks).toContain("Release notes review for the target minor");
});

test("maps unhealthy workload findings to workload health upgrade details", () => {
  const report = parseFindingsDocument({
    currentVersion: "1.29",
    targetVersion: "1.30",
    findings: [{
      ...baseFinding,
      ruleId: "WORKLOAD-001",
      severity: "Warning",
      priority: "P4",
      affectedScope: "workload",
      canUpgradeContinue: true,
      fingerprint: "fp-workload-001",
      resources: [{ plane: "live", kind: "Pod", namespace: "kp-demo", name: "unhealthy-image-app-abc" }],
    }],
  });

  const details = upgradeDetails(report);

  expect(details[0].findingLines).toContain("Workload health: 1 warning(s) (WORKLOAD-001)");
});

test("marks future hop upgrade details as planned and requiring re-scan", () => {
  const report = parseFindingsDocument({
    currentVersion: "1.32",
    targetVersion: "1.36",
    findings: [baseFinding],
  });
  const details = upgradeDetails(report);
  expect(details.map((hop) => `${hop.from}->${hop.to}`)).toEqual(["1.32->1.33", "1.33->1.34", "1.34->1.35", "1.35->1.36"]);
  expect(report.summary.blockers).toBe(1);
  expect(details[0].statusLabel).toBe("Planned, hop-specific scan recommended");
  expect(details[0].statusClass).toBe("rescan-required");
  expect(details[0].assessment).toContain("Findings were evaluated against final target 1.36, not this individual hop.");
  expect(details[0].findingLines).toContain("Overall target blockers remain listed in this report, but they are not proof that this intermediate hop is blocked.");
  expect(details[0].findingLines).not.toContain("PDB and drain safety: 1 blocker(s) (PDB-001)");
  expect(details.slice(1).every((hop) => hop.statusLabel === "Planned, re-scan required")).toBe(true);
  expect(details[1].findingLines).toContain("Findings were evaluated against final target 1.36; current findings are not projected as proof for this future cluster state.");
});

// Guards the exact regression found in review: resultFromSummary must
// check incomplete coverage BEFORE the blocker count, not after — a scan
// with real blockers AND partial coverage must still report INCOMPLETE at
// the top level, mirroring Go's Report.resultAndExitCode() exactly.
test("incomplete coverage outranks a real blocker count, not just a clean report", () => {
  const report = parseFindingsDocument({
    findings: [{ ...baseFinding }],
    coverage: { kubernetes: { status: "partial", errors: ["pods: forbidden"] } },
  });
  expect(report.summary.blockers).toBe(1);
  expect(report.result).toBe("INCOMPLETE");
});

describe("resource identity fallbacks", () => {
  test("defaults plane from sourcePath/providerId when absent", () => {
    const report = parseFindingsDocument({
      findings: [{ ...baseFinding, resources: [{ kind: "Deployment", name: "api", namespace: "", sourcePath: "deploy/api.yaml" }] }],
    });
    expect(report.findings[0].resources[0].plane).toBe("manifest");
  });
});

describe("EKS cluster metadata", () => {
  test("absent when the document has no eksCluster field (cluster-only scan)", () => {
    const report = parseFindingsDocument({ findings: [baseFinding] });
    expect(report.eksCluster).toBeUndefined();
  });

  test("parses a full eksCluster object", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksCluster: {
        clusterName: "prod-cluster",
        region: "ap-south-1",
        version: "1.29",
        platformVersion: "eks.5",
        status: "ACTIVE",
        supportType: "EXTENDED",
        endpointAccess: "public",
        arn: "arn:aws:eks:ap-south-1:123456789012:cluster/prod-cluster",
      },
    });
    expect(report.eksCluster).toMatchObject({
      clusterName: "prod-cluster",
      region: "ap-south-1",
      version: "1.29",
      platformVersion: "eks.5",
      status: "ACTIVE",
      supportType: "EXTENDED",
      endpointAccess: "public",
    });
  });

  test("drops a malformed eksCluster value instead of passing it through untyped", () => {
    expect(parseFindingsDocument({ findings: [baseFinding], eksCluster: "not-an-object" }).eksCluster).toBeUndefined();
    expect(parseFindingsDocument({ findings: [baseFinding], eksCluster: null }).eksCluster).toBeUndefined();
    // A present-but-empty object has nothing usable to show either.
    expect(parseFindingsDocument({ findings: [baseFinding], eksCluster: {} }).eksCluster).toBeUndefined();
  });

  test("ignores non-string fields inside an otherwise-valid eksCluster object", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksCluster: { region: "ap-south-1", version: 129 },
    });
    expect(report.eksCluster).toEqual({ region: "ap-south-1" });
  });

  test("eksSupportTypeLabel/eksEndpointAccessLabel map known values and hide unknown ones", () => {
    expect(eksSupportTypeLabel("EXTENDED")).toBe("Extended support");
    expect(eksSupportTypeLabel("STANDARD")).toBe("Standard support");
    expect(eksSupportTypeLabel(undefined)).toBe("");
    expect(eksSupportTypeLabel("")).toBe("");

    expect(eksEndpointAccessLabel("public")).toBe("Public");
    expect(eksEndpointAccessLabel("private")).toBe("Private");
    expect(eksEndpointAccessLabel("public_and_private")).toBe("Public + private");
    expect(eksEndpointAccessLabel("unknown")).toBe("");
    expect(eksEndpointAccessLabel(undefined)).toBe("");
  });
});

describe("EKS add-on inventory", () => {
  test("absent when the document has no eksAddons field", () => {
    const report = parseFindingsDocument({ findings: [baseFinding] });
    expect(report.eksAddons).toBeUndefined();
  });

  test("absent for an empty eksAddons array", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], eksAddons: [] });
    expect(report.eksAddons).toBeUndefined();
  });

  test("parses a full add-on inventory", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksAddons: [
        { name: "vpc-cni", currentVersion: "v1.18.1-eksbuild.1", compatibleVersions: ["v1.18.1-eksbuild.1"], compatible: true },
        { name: "coredns", currentVersion: "v1.10.1-eksbuild.1", compatibleVersions: ["v1.11.0-eksbuild.1"], compatible: false },
        { name: "kube-proxy", currentVersion: "v1.29.0-eksbuild.1", compatible: false, verificationUnavailable: true },
      ],
    });
    expect(report.eksAddons).toHaveLength(3);
    expect(report.eksAddons?.[0]).toMatchObject({ name: "vpc-cni", compatible: true });
    expect(report.eksAddons?.[2]).toMatchObject({ name: "kube-proxy", verificationUnavailable: true });
  });

  test("drops entries with no usable name", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksAddons: [{ currentVersion: "v1.0.0" }, { name: "vpc-cni", compatible: true }],
    });
    expect(report.eksAddons).toHaveLength(1);
    expect(report.eksAddons?.[0].name).toBe("vpc-cni");
  });

  test("eksAddonStatus mirrors the three-state classification", () => {
    expect(eksAddonStatus({ name: "a", compatible: true })).toEqual({ label: "Compatible", className: "clean" });
    expect(eksAddonStatus({ name: "a", compatible: false })).toEqual({ label: "Needs update", className: "blocked" });
    expect(eksAddonStatus({ name: "a", compatible: false, verificationUnavailable: true })).toEqual({ label: "Verification unavailable", className: "warn" });
  });
});

describe("EKS managed node group inventory", () => {
  test("absent when the document has no eksNodegroups field", () => {
    const report = parseFindingsDocument({ findings: [baseFinding] });
    expect(report.eksNodegroups).toBeUndefined();
  });

  test("empty array is preserved for explicit no managed node groups", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], eksNodegroups: [] });
    expect(report.eksNodegroups).toEqual([]);
  });

  test("parses node group readiness inventory", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
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
        launchTemplate: true,
        healthIssues: [{ code: "AccessDenied", message: "node role cannot call API", resourceIds: ["i-123"] }],
        readinessStatus: "Review required",
      }],
    });
    expect(report.eksNodegroups).toHaveLength(1);
    expect(report.eksNodegroups?.[0]).toMatchObject({ name: "ng-app", desiredSize: 3, launchTemplate: true });
    expect(eksNodegroupHealthLabel(report.eksNodegroups![0])).toBe("AccessDenied");
    expect(eksNodegroupReadinessClass(report.eksNodegroups![0])).toBe("warn");
  });

  test("drops entries with no usable name", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksNodegroups: [{ status: "ACTIVE" }, { name: "ng-app", readinessStatus: "Ready with review" }],
    });
    expect(report.eksNodegroups).toHaveLength(1);
    expect(report.eksNodegroups?.[0].name).toBe("ng-app");
  });
});

describe("EKS Upgrade Insights inventory", () => {
  test("absent when the document has no eksUpgradeInsights field", () => {
    const report = parseFindingsDocument({ findings: [baseFinding] });
    expect(report.eksUpgradeInsights).toBeUndefined();
  });

  test("empty array is preserved for explicit no insights", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], eksUpgradeInsights: [] });
    expect(report.eksUpgradeInsights).toEqual([]);
  });

  test("parses insight inventory including PASSING status", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksUpgradeInsights: [{
        id: "insight-1",
        name: "Deprecated API usage",
        category: "UPGRADE_READINESS",
        status: "PASSING",
        kubernetesVersion: "1.34",
        lastRefreshTime: "2026-06-01T00:00:00Z",
        recommendation: "No action required.",
        additionalInfo: { docs: "https://docs.aws.amazon.com/eks/" },
        deprecationDetails: ["usage: policy/v1beta1/podsecuritypolicies"],
        addonCompatibilityDetails: ["vpc-cni compatible versions: v1.18.1-eksbuild.1"],
      }],
    });
    expect(report.eksUpgradeInsights).toHaveLength(1);
    expect(report.eksUpgradeInsights?.[0]).toMatchObject({ id: "insight-1", status: "PASSING" });
    expect(eksUpgradeInsightStatusClass(report.eksUpgradeInsights![0])).toBe("clean");
    expect(eksUpgradeInsightDetails(report.eksUpgradeInsights![0])).toContain("vpc-cni compatible versions");
  });

  test("drops entries with no usable id or name", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      eksUpgradeInsights: [{ id: "missing-name", status: "ERROR" }, { id: "insight-1", name: "Deprecated API usage", status: "ERROR" }],
    });
    expect(report.eksUpgradeInsights).toHaveLength(1);
    expect(report.eksUpgradeInsights?.[0].id).toBe("insight-1");
  });
});

describe("upgrade risk prioritization", () => {
  test("priorityRank orders P1 through P4, unknown last", () => {
    expect(priorityRank("P1")).toBeLessThan(priorityRank("P2"));
    expect(priorityRank("P2")).toBeLessThan(priorityRank("P3"));
    expect(priorityRank("P3")).toBeLessThan(priorityRank("P4"));
    expect(priorityRank("P4")).toBeLessThan(priorityRank(undefined));
    expect(priorityRank("P4")).toBeLessThan(priorityRank("not-a-real-priority"));
  });

  test("priorityPillClass maps known priorities and falls back to p4", () => {
    expect(priorityPillClass("P1")).toBe("p1");
    expect(priorityPillClass("P2")).toBe("p2");
    expect(priorityPillClass("P3")).toBe("p3");
    expect(priorityPillClass("P4")).toBe("p4");
    expect(priorityPillClass(undefined)).toBe("p4");
    expect(priorityPillClass("garbage")).toBe("p4");
  });

  test("parseFindingsDocument defaults priority/canUpgradeContinue for a pre-priority legacy findings.json", () => {
    const { priority: _priority, priorityReason: _reason, affectedScope: _scope, canUpgradeContinue: _continue, ...legacyFinding } = baseFinding as Finding & { priority?: string };
    const report = parseFindingsDocument({ findings: [legacyFinding] });
    expect(report.findings[0].priority).toBe("");
    expect(report.findings[0].canUpgradeContinue).toBe(true);
  });

  test("parseFindingsDocument carries priority fields through when present", () => {
    const report = parseFindingsDocument({
      findings: [{ ...baseFinding, priority: "P1", priorityReason: "Global blocker.", affectedScope: "global", canUpgradeContinue: false }],
    });
    expect(report.findings[0]).toMatchObject({ priority: "P1", priorityReason: "Global blocker.", affectedScope: "global", canUpgradeContinue: false });
  });

  // Mirrors Go's TestFilterAndSort_PriorityOutranksRuleIDWithinSameSeverity
  // (internal/report/view_test.go): three Blocker-severity findings whose
  // rule-ID order (API-001, PDB-001, WH-002) is the *opposite* of their
  // priority order (WH-002/P1, API-001/P2, PDB-001/P3) — proving Priority
  // actually overrides the old rule-ID/resource tie-break, not just
  // coincidentally agreeing with it.
  test("compareFindings sorts Priority ahead of rule ID within the same severity", () => {
    const wh002: Finding = { ...baseFinding, ruleId: "WH-002", fingerprint: "fp-wh002", priority: "P1", resources: [{ plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name: "catch-all-guard" }] };
    const api001: Finding = { ...baseFinding, ruleId: "API-001", fingerprint: "fp-api001", priority: "P2", resources: [{ plane: "manifest", kind: "PodDisruptionBudget", namespace: "prod-like", name: "old-pdb-api" }] };
    const pdb001: Finding = { ...baseFinding, ruleId: "PDB-001", fingerprint: "fp-pdb001", priority: "P3", resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "prod-like", name: "payment-api-pdb" }] };

    const sorted = [api001, pdb001, wh002].sort(compareFindings);
    expect(sorted.map((f) => f.ruleId)).toEqual(["WH-002", "API-001", "PDB-001"]);

    // topRisks and filterFindings both go through compareFindings — same
    // guarantee end to end, not just in the raw comparator.
    expect(topRisks([api001, pdb001, wh002], 3).map((f) => f.ruleId)).toEqual(["WH-002", "API-001", "PDB-001"]);
    expect(filterFindings([api001, pdb001, wh002], { severities: undefined, search: "", confidence: "", namespace: "" }).map((f) => f.ruleId)).toEqual(["WH-002", "API-001", "PDB-001"]);
  });
});
