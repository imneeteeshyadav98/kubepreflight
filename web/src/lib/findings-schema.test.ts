import { describe, expect, test } from "vitest";
import { categoryExecutionCoverage, clusterDisplayName, compareFindings, deriveAPICompatibilitySummary, deriveUpgradeReadinessSummary, eksAddonStatus, eksEndpointAccessLabel, eksNodegroupHealthLabel, eksNodegroupReadinessClass, eksSupportTypeLabel, eksUpgradeInsightDetails, eksUpgradeInsightStatusClass, evaluationCoverageAdvisory, evaluationCoverageStatus, evaluationCoverageStatusLabel, filterFindings, impactScopesLabel, parseFindingsDocument, priorityPillClass, priorityRank, resultFromSummary, ruleApplicabilityLabel, ruleExecutionCoverageSummary, ruleExecutionDisplayLabel, ruleExecutionDisplayState, ruleExecutionStateLabel, scoreQualification, topRisks, upgradeApplicable, upgradeContext, upgradeDetails, type Finding, type RuleExecutionRecord } from "./findings-schema";

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

test("preserves and labels impact scopes from canonical findings", () => {
  const report = parseFindingsDocument({
    findings: [
      {
        ...baseFinding,
        ruleId: "DRAIN-003",
        severity: "Warning",
        fingerprint: "fp-impact-scopes",
        impactScopes: ["worker_rollout", "node_drain", "workload_restart"],
      },
    ],
  });

  expect(report.findings[0].impactScopes).toEqual(["worker_rollout", "node_drain", "workload_restart"]);
  expect(impactScopesLabel(report.findings[0].impactScopes)).toBe("worker rollout, node drain, workload restart");
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

describe("api compatibility scorecard", () => {
  test("parses provided apiCompatibility without deriving a parallel shape", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      apiCompatibility: {
        status: "Warning",
        upgradeContinue: true,
        removedObjects: 0,
        deprecatedObjects: 1,
        deprecatedFamilies: [{ apiVersion: "policy/v1beta1", kind: "PodDisruptionBudget", count: 1, resources: ["PodDisruptionBudget/apps/api-pdb"] }],
        criticalImpact: false,
        scoreImpact: -5,
      },
    });

    expect(report.apiCompatibility).toMatchObject({
      status: "Warning",
      upgradeContinue: true,
      deprecatedObjects: 1,
      scoreImpact: -5,
    });
    expect(report.apiCompatibility?.deprecatedFamilies?.[0]).toMatchObject({ apiVersion: "policy/v1beta1", kind: "PodDisruptionBudget", count: 1 });
  });

  test("derives removed and deprecated API families for legacy findings-only reports", () => {
    const findings: Finding[] = [
      {
        ...baseFinding,
        ruleId: "API-001",
        severity: "Blocker",
        fingerprint: "fp-api-removed-a",
        evidence: ["apiVersion: policy/v1beta1"],
        resources: [{ plane: "manifest", kind: "PodSecurityPolicy", namespace: "", name: "restricted", scope: "cluster", sourcePath: "manifests/psp.yaml" }],
      },
      {
        ...baseFinding,
        ruleId: "API-001",
        severity: "Blocker",
        fingerprint: "fp-api-removed-b",
        evidence: ["apiVersion: policy/v1beta1"],
        resources: [{ plane: "manifest", kind: "PodSecurityPolicy", namespace: "", name: "baseline", scope: "cluster", sourcePath: "manifests/psp.yaml" }],
      },
      {
        ...baseFinding,
        ruleId: "API-002",
        severity: "Warning",
        fingerprint: "fp-api-deprecated",
        evidence: ["apiVersion: policy/v1beta1"],
        resources: [{ plane: "manifest", kind: "PodDisruptionBudget", namespace: "apps", name: "api-pdb", sourcePath: "manifests/pdb.yaml" }],
      },
    ];

    const summary = deriveAPICompatibilitySummary(findings);

    expect(summary).toMatchObject({
      status: "Failed",
      upgradeContinue: false,
      removedObjects: 2,
      deprecatedObjects: 1,
      criticalImpact: true,
      scoreImpact: -45,
    });
    expect(summary.removedFamilies).toEqual([
      { apiVersion: "policy/v1beta1", kind: "PodSecurityPolicy", count: 2, resources: ["PodSecurityPolicy/baseline", "PodSecurityPolicy/restricted"] },
    ]);
    expect(summary.deprecatedFamilies?.[0]).toMatchObject({ apiVersion: "policy/v1beta1", kind: "PodDisruptionBudget", count: 1 });

    const report = parseFindingsDocument({ findings });
    expect(report.apiCompatibility?.status).toBe("Failed");
    expect(report.apiCompatibility?.removedFamilies?.[0].count).toBe(2);
  });
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

test("builds no-upgrade context when current and target versions match", () => {
  const report = parseFindingsDocument({ currentVersion: "v1.36.1-eks-abcdef", targetVersion: "1.36", findings: [baseFinding] });
  expect(upgradeApplicable(report)).toBe(false);
  expect(upgradeContext(report)).toMatchObject({
    path: "1.36",
    label: "same-minor target",
    line: "Cluster is already running 1.36 — no version upgrade is being assessed.",
  });
});

describe("upgradeApplicable", () => {
  test("false only when current and target normalize to the same major.minor", () => {
    expect(upgradeApplicable({ currentVersion: "1.32", targetVersion: "1.32" })).toBe(false);
    expect(upgradeApplicable({ currentVersion: "v1.32.9-eks-1234567", targetVersion: "1.32.0" })).toBe(false);
    expect(upgradeApplicable({ currentVersion: "1.32", targetVersion: "1.33" })).toBe(true);
  });

  test("defaults true when current version is unknown or unparseable, never guessing", () => {
    expect(upgradeApplicable({ currentVersion: "", targetVersion: "1.36" })).toBe(true);
    expect(upgradeApplicable({ currentVersion: "not-a-version", targetVersion: "1.36" })).toBe(true);
    expect(upgradeApplicable({ currentVersion: "1.32", targetVersion: "not-a-version" })).toBe(true);
  });
});

describe("clusterDisplayName", () => {
  test("prefers eksCluster.clusterName over clusterContext", () => {
    const report = {
      clusterContext: "arn:aws:eks:eu-north-1:123456789012:cluster/exciting-dance-outfit",
      eksCluster: { clusterName: "exciting-dance-outfit", arn: "arn:aws:eks:eu-north-1:123456789012:cluster/exciting-dance-outfit" },
    };
    expect(clusterDisplayName(report)).toEqual({
      short: "exciting-dance-outfit",
      full: "arn:aws:eks:eu-north-1:123456789012:cluster/exciting-dance-outfit",
    });
  });

  test("eksCluster present but arn empty falls back to clusterContext as full", () => {
    const report = { clusterContext: "some-context-name", eksCluster: { clusterName: "exciting-dance-outfit" } };
    expect(clusterDisplayName(report)).toEqual({ short: "exciting-dance-outfit", full: "some-context-name" });
  });

  test("no eksCluster, ARN-shaped clusterContext gets shortened", () => {
    const report = { clusterContext: "arn:aws:eks:eu-north-1:123456789012:cluster/exciting-dance-outfit" };
    expect(clusterDisplayName(report)).toEqual({
      short: "exciting-dance-outfit",
      full: "arn:aws:eks:eu-north-1:123456789012:cluster/exciting-dance-outfit",
    });
  });

  test("non-ARN clusterContext is returned unchanged, never blanked", () => {
    expect(clusterDisplayName({ clusterContext: "kind-kp-smoke" })).toEqual({ short: "kind-kp-smoke", full: "" });
    expect(clusterDisplayName({ clusterContext: "arn:aws:iam::123456789012:role/my-role" })).toEqual({
      short: "arn:aws:iam::123456789012:role/my-role",
      full: "",
    });
  });

  test("empty clusterContext stays empty, never guessed", () => {
    expect(clusterDisplayName({ clusterContext: "" })).toEqual({ short: "", full: "" });
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

test("maps deprecated master label findings to node scheduling compatibility upgrade details", () => {
  const report = parseFindingsDocument({
    currentVersion: "1.29",
    targetVersion: "1.30",
    findings: [{
      ...baseFinding,
      ruleId: "NODE-003",
      severity: "Warning",
      priority: "P4",
      affectedScope: "workload",
      canUpgradeContinue: true,
      fingerprint: "fp-node-003",
      resources: [{ plane: "live", kind: "Deployment", namespace: "kp-demo", name: "legacy-pinned" }],
    }],
  });

  const details = upgradeDetails(report);

  expect(details[0].findingLines).toContain("Node scheduling compatibility: 1 warning(s) (NODE-003)");
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
    expect(report.findings[0].canUpgradeContinue).toBe(false);
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

describe("deriveUpgradeReadinessSummary", () => {
  // The full 10-category → rule ID map, mirrored from
  // internal/findings/report.go's categoryByRuleID — one representative
  // rule ID per category, matching the Go table test's granularity.
  const perCategoryRuleId: Record<string, string> = {
    "API Compatibility": "API-001",
    "Extension APIs": "CRD-001",
    "Admission Webhooks": "WH-001",
    "Disruption Safety": "PDB-001",
    "Drain Readiness": "DRAIN-001",
    "Node Readiness": "NODE-001",
    "Add-ons": "ADDON-001",
    CoreDNS: "COREDNS-001",
    "Workload Health": "WORKLOAD-001",
    "EKS Upgrade Insights": "EKS-INSIGHT-001",
  };

  test.each(Object.entries(perCategoryRuleId))("a Blocker finding for %s (%s) marks only that category Failed", (categoryName, ruleId) => {
    const finding: Finding = { ...baseFinding, ruleId, severity: "Blocker", fingerprint: `fp-${ruleId}` };
    const summary = deriveUpgradeReadinessSummary([finding], "BLOCKED");

    expect(summary.verdict).toBe("BLOCKED");
    expect(summary.upgradeContinue).toBe(false);
    expect(summary.categories).toHaveLength(10);
    summary.categories.forEach((category) => {
      if (category.name === categoryName) {
        expect(category).toMatchObject({ status: "Failed", blockerCount: 1, warningCount: 0, ruleIds: [ruleId] });
      } else {
        expect(category).toMatchObject({ status: "Passed", blockerCount: 0, warningCount: 0 });
      }
    });
  });

  test("a Warning-only finding reports Warning without blocking the upgrade", () => {
    const finding: Finding = { ...baseFinding, ruleId: "COREDNS-001", severity: "Warning", fingerprint: "fp-coredns" };
    const summary = deriveUpgradeReadinessSummary([finding], "PASSED_WITH_WARNINGS");
    expect(summary.upgradeContinue).toBe(true);
    const coredns = summary.categories.find((c) => c.name === "CoreDNS");
    expect(coredns).toMatchObject({ status: "Warning", blockerCount: 0, warningCount: 1 });
  });

  test("no findings is a clean, fully-passed scorecard at score 100", () => {
    const summary = deriveUpgradeReadinessSummary([], "CLEAN");
    expect(summary).toMatchObject({ verdict: "CLEAN", upgradeContinue: true, readinessScore: 100 });
    expect(summary.categories.every((c) => c.status === "Passed")).toBe(true);
  });

  test("score formula matches the Go implementation: 15 per failed category, capped per category, floored at 0", () => {
    const single = deriveUpgradeReadinessSummary([{ ...baseFinding, ruleId: "WH-001", severity: "Blocker", fingerprint: "fp-1" }], "BLOCKED");
    expect(single.readinessScore).toBe(85);

    const many = deriveUpgradeReadinessSummary(
      Object.values(perCategoryRuleId).map((ruleId, i) => ({ ...baseFinding, ruleId, severity: "Blocker" as const, fingerprint: `fp-${i}` })),
      "BLOCKED",
    );
    expect(many.readinessScore).toBe(0);
  });

  test("parseFindingsDocument wires upgradeReadiness in via the derive fallback when the raw document has no precomputed field", () => {
    const report = parseFindingsDocument({ findings: [{ ...baseFinding, ruleId: "PDB-001", severity: "Blocker" }] });
    expect(report.upgradeReadiness?.verdict).toBe(report.result);
    const disruption = report.upgradeReadiness?.categories.find((c) => c.name === "Disruption Safety");
    expect(disruption).toMatchObject({ status: "Failed", blockerCount: 1 });
  });

  test("parseFindingsDocument prefers a precomputed upgradeReadiness field over deriving one", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      upgradeReadiness: {
        verdict: "CLEAN",
        upgradeContinue: true,
        readinessScore: 42,
        categories: [{ name: "Disruption Safety", status: "Passed", blockerCount: 0, warningCount: 0, ruleIds: [] }],
      },
    });
    // The precomputed field wins even though it disagrees with what the
    // findings would derive — proves normalize takes priority over derive.
    expect(report.upgradeReadiness?.readinessScore).toBe(42);
  });
});

describe("rule execution coverage (PR 4: Console evaluation-coverage UI)", () => {
  const nativeExecutions: RuleExecutionRecord[] = [
    { ruleId: "API-001", applicability: "applicable", state: "evaluated" },
    { ruleId: "API-002", applicability: "applicable", state: "evaluated" },
    { ruleId: "PDB-001", applicability: "applicable", state: "insufficient_evidence", reason: "PDB collector returned partial data" },
    { ruleId: "PDB-002", applicability: "applicable", state: "failed", reason: "rule panicked" },
    { ruleId: "CRD-001", applicability: "not_applicable", state: "not_evaluated", reason: "not registered for this scan mode" },
  ];

  test("a native report parses ruleExecutions verbatim and leaves ruleExecutionsNormalized false", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], ruleExecutions: nativeExecutions });
    expect(report.ruleExecutionsNormalized).toBe(false);
    expect(report.ruleExecutions).toHaveLength(5);
    expect(report.ruleExecutions?.[0]).toEqual({ ruleId: "API-001", applicability: "applicable", state: "evaluated", reason: undefined });
    expect(report.ruleExecutions?.[2].reason).toBe("PDB collector returned partial data");
  });

  test("ruleExecutionsNormalized: false is preserved explicitly (not just defaulted)", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], ruleExecutions: nativeExecutions, ruleExecutionsNormalized: false });
    expect(report.ruleExecutionsNormalized).toBe(false);
  });

  test("a normalized-legacy report parses ruleExecutionsNormalized: true and is labeled as such by the coverage summary", () => {
    const report = parseFindingsDocument({
      findings: [baseFinding],
      ruleExecutions: nativeExecutions,
      ruleExecutionsNormalized: true,
    });
    expect(report.ruleExecutionsNormalized).toBe(true);
    expect(ruleExecutionCoverageSummary(report).source).toBe("normalized-legacy");
  });

  test("a native report (ruleExecutionsNormalized omitted) is labeled 'native', never 'normalized-legacy'", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], ruleExecutions: nativeExecutions });
    expect(ruleExecutionCoverageSummary(report).source).toBe("native");
  });

  test("an old v1.0 report with no ruleExecutions field at all loads without crashing, with no misleading 'fully evaluated' implication", () => {
    const report = parseFindingsDocument({ findings: [baseFinding] });
    expect(report.ruleExecutions).toBeUndefined();
    expect(report.ruleExecutionsNormalized).toBe(false);
    const summary = ruleExecutionCoverageSummary(report);
    expect(summary).toEqual({
      source: "unavailable",
      total: 0,
      counts: { evaluated: 0, not_evaluated: 0, insufficient_evidence: 0, failed: 0, not_applicable: 0 },
    });
  });

  test("a present-but-empty ruleExecutions array is also treated as 'unavailable', not zero-coverage-native", () => {
    const report = parseFindingsDocument({ findings: [baseFinding], ruleExecutions: [] });
    expect(report.ruleExecutions).toEqual([]);
    expect(ruleExecutionCoverageSummary(report).source).toBe("unavailable");
  });

  test("complete coverage (every rule evaluated) renders as all-evaluated counts", () => {
    const allEvaluated: RuleExecutionRecord[] = [
      { ruleId: "API-001", applicability: "applicable", state: "evaluated" },
      { ruleId: "API-002", applicability: "applicable", state: "evaluated" },
      { ruleId: "PDB-001", applicability: "applicable", state: "evaluated" },
    ];
    const report = parseFindingsDocument({ findings: [], ruleExecutions: allEvaluated });
    const summary = ruleExecutionCoverageSummary(report);
    expect(summary.total).toBe(3);
    expect(summary.counts).toEqual({ evaluated: 3, not_evaluated: 0, insufficient_evidence: 0, failed: 0, not_applicable: 0 });
  });

  test("partial coverage (mix of states) tallies each bucket independently, including not_applicable separate from not_evaluated", () => {
    const report = parseFindingsDocument({ findings: [], ruleExecutions: nativeExecutions });
    const summary = ruleExecutionCoverageSummary(report);
    expect(summary.total).toBe(5);
    expect(summary.counts).toEqual({ evaluated: 2, not_evaluated: 0, insufficient_evidence: 1, failed: 1, not_applicable: 1 });
  });

  test("an unrecognized/malformed state is conservatively normalized to not_evaluated, never evaluated", () => {
    const report = parseFindingsDocument({
      findings: [],
      ruleExecutions: [{ ruleId: "NODE-001", applicability: "applicable", state: "bogus-state" }],
    });
    expect(report.ruleExecutions?.[0].state).toBe("not_evaluated");
  });

  test("an entry missing ruleId is dropped rather than producing a broken record", () => {
    const report = parseFindingsDocument({
      findings: [],
      ruleExecutions: [{ applicability: "applicable", state: "evaluated" }, { ruleId: "NODE-001", applicability: "applicable", state: "evaluated" }],
    });
    expect(report.ruleExecutions).toHaveLength(1);
    expect(report.ruleExecutions?.[0].ruleId).toBe("NODE-001");
  });

  test("display labels use exactly the specified strings, never 'Unknown' as an umbrella term", () => {
    expect(ruleExecutionDisplayLabel("evaluated")).toBe("Evaluated");
    expect(ruleExecutionDisplayLabel("not_evaluated")).toBe("Not evaluated");
    expect(ruleExecutionDisplayLabel("insufficient_evidence")).toBe("Insufficient evidence");
    expect(ruleExecutionDisplayLabel("failed")).toBe("Failed");
    expect(ruleExecutionDisplayLabel("not_applicable")).toBe("Not applicable");
    expect(ruleExecutionStateLabel("not_evaluated")).toBe("Not evaluated");
    expect(ruleApplicabilityLabel("not_applicable")).toBe("Not applicable");
    expect(ruleApplicabilityLabel("applicable")).toBe("Applicable");
  });

  test("ruleExecutionDisplayState folds not_applicable ahead of the raw state for filtering/summary purposes", () => {
    expect(ruleExecutionDisplayState({ applicability: "not_applicable", state: "not_evaluated" })).toBe("not_applicable");
    expect(ruleExecutionDisplayState({ applicability: "applicable", state: "failed" })).toBe("failed");
  });

  describe("categoryExecutionCoverage", () => {
    test("returns 'unavailable' when the report has no ruleExecutions at all", () => {
      expect(categoryExecutionCoverage("Disruption Safety", undefined)).toEqual({ state: "unavailable", evaluatedCount: 0, totalApplicable: 0 });
      expect(categoryExecutionCoverage("Disruption Safety", [])).toEqual({ state: "unavailable", evaluatedCount: 0, totalApplicable: 0 });
    });

    test("returns 'full' when every rule mapped to the category evaluated", () => {
      const executions: RuleExecutionRecord[] = [
        { ruleId: "PDB-001", applicability: "applicable", state: "evaluated" },
        { ruleId: "PDB-002", applicability: "applicable", state: "evaluated" },
      ];
      expect(categoryExecutionCoverage("Disruption Safety", executions)).toEqual({ state: "full", evaluatedCount: 2, totalApplicable: 2 });
    });

    test("returns 'none' when zero rules in the category evaluated — the case a bare 'Passed' pill would otherwise hide", () => {
      const executions: RuleExecutionRecord[] = [
        { ruleId: "PDB-001", applicability: "applicable", state: "not_evaluated", reason: "not registered for this scan mode" },
        { ruleId: "PDB-002", applicability: "applicable", state: "not_evaluated", reason: "not registered for this scan mode" },
      ];
      expect(categoryExecutionCoverage("Disruption Safety", executions)).toEqual({ state: "none", evaluatedCount: 0, totalApplicable: 2 });
    });

    test("returns 'partial' for a mix of evaluated and not_evaluated within one category", () => {
      const executions: RuleExecutionRecord[] = [
        { ruleId: "PDB-001", applicability: "applicable", state: "evaluated" },
        { ruleId: "PDB-002", applicability: "applicable", state: "not_evaluated" },
      ];
      expect(categoryExecutionCoverage("Disruption Safety", executions)).toEqual({ state: "partial", evaluatedCount: 1, totalApplicable: 2 });
    });

    test("returns 'not_applicable' when every rule mapped to the category was out of scope for this scan mode", () => {
      const executions: RuleExecutionRecord[] = [
        { ruleId: "PDB-001", applicability: "not_applicable", state: "not_evaluated" },
        { ruleId: "PDB-002", applicability: "not_applicable", state: "not_evaluated" },
      ];
      expect(categoryExecutionCoverage("Disruption Safety", executions)).toEqual({ state: "not_applicable", evaluatedCount: 0, totalApplicable: 0 });
    });
  });

  test("normalizing a legacy report with the exact backfill reasons/states internal/comparison/normalize.go produces parses without error", () => {
    // Mirrors normalizeRuleExecutions' (Go) exact output shape for a legacy
    // document: every rule in the universe gets a record, applicability is
    // always "applicable" (legacy normalization never claims to know
    // applicability), and only rule IDs with a matching finding are
    // "evaluated" — every other rule ID is "not_evaluated", never assumed
    // clean.
    const report = parseFindingsDocument({
      findings: [{ ...baseFinding, ruleId: "PDB-001" }],
      ruleExecutionsNormalized: true,
      ruleExecutions: [
        { ruleId: "PDB-001", applicability: "applicable", state: "evaluated", reason: "legacy report contains a finding from this rule; execution metadata was backfilled from finding presence, not a native execution record" },
        { ruleId: "PDB-002", applicability: "applicable", state: "not_evaluated", reason: "legacy report does not contain native rule-execution metadata; absence of a finding was not treated as evaluated-and-clean" },
      ],
    });
    expect(report.ruleExecutionsNormalized).toBe(true);
    expect(report.ruleExecutions?.find((r) => r.ruleId === "PDB-001")?.state).toBe("evaluated");
    expect(report.ruleExecutions?.find((r) => r.ruleId === "PDB-002")?.state).toBe("not_evaluated");
  });

  // PR 6: the 4-value evaluationCoverageStatus classification and its
  // score-qualification/advisory text -- mirrors
  // internal/report/evaluation_coverage.go's Go-side classification
  // (BuildEvaluationCoverage/EvaluationCoverageStatus/Advisory/
  // ScoreQualification) in meaning, derived from the exact same
  // ruleExecutionCoverageSummary counts every other coverage surface here
  // already reads from.
  describe("evaluationCoverageStatus / evaluationCoverageAdvisory / scoreQualification (PR 6)", () => {
    test("test 1: complete native coverage -> complete", () => {
      const report = parseFindingsDocument({ findings: [], ruleExecutions: [{ ruleId: "API-001", applicability: "applicable", state: "evaluated" }] });
      const summary = ruleExecutionCoverageSummary(report);
      expect(evaluationCoverageStatus(summary)).toBe("complete");
      expect(evaluationCoverageAdvisory(evaluationCoverageStatus(summary), summary)).toBe("");
    });

    test("test 2/3/4: any of not_evaluated/insufficient_evidence/failed alone -> partial", () => {
      for (const state of ["not_evaluated", "insufficient_evidence", "failed"] as const) {
        const report = parseFindingsDocument({
          findings: [],
          ruleExecutions: [
            { ruleId: "API-001", applicability: "applicable", state: "evaluated" },
            { ruleId: "PDB-001", applicability: "applicable", state },
          ],
        });
        const summary = ruleExecutionCoverageSummary(report);
        expect(evaluationCoverageStatus(summary)).toBe("partial");
      }
    });

    test("test 5: not_applicable rules alone never cause partial", () => {
      const report = parseFindingsDocument({
        findings: [],
        ruleExecutions: [
          { ruleId: "API-001", applicability: "applicable", state: "evaluated" },
          { ruleId: "CRD-001", applicability: "not_applicable", state: "not_evaluated" },
        ],
      });
      const summary = ruleExecutionCoverageSummary(report);
      expect(evaluationCoverageStatus(summary)).toBe("complete");
    });

    test("test 6: no rule-execution metadata at all -> unavailable", () => {
      const report = parseFindingsDocument({ findings: [] });
      const summary = ruleExecutionCoverageSummary(report);
      expect(evaluationCoverageStatus(summary)).toBe("unavailable");
    });

    test("test 7: ruleExecutionsNormalized true -> normalized_legacy, even with every backfilled rule reading evaluated", () => {
      const report = parseFindingsDocument({
        findings: [],
        ruleExecutions: [{ ruleId: "API-001", applicability: "applicable", state: "evaluated" }],
        ruleExecutionsNormalized: true,
      });
      const summary = ruleExecutionCoverageSummary(report);
      expect(evaluationCoverageStatus(summary)).toBe("normalized_legacy");
      expect(evaluationCoverageStatus(summary)).not.toBe("complete");
    });

    test("test 10/11: advisory text present when partial, absent when complete", () => {
      const partialReport = parseFindingsDocument({
        findings: [],
        ruleExecutions: [
          { ruleId: "API-001", applicability: "applicable", state: "evaluated" },
          { ruleId: "PDB-001", applicability: "applicable", state: "failed" },
        ],
      });
      const partialSummary = ruleExecutionCoverageSummary(partialReport);
      const partialStatus = evaluationCoverageStatus(partialSummary);
      expect(evaluationCoverageAdvisory(partialStatus, partialSummary)).toContain("not fully evaluated");

      const completeReport = parseFindingsDocument({ findings: [], ruleExecutions: [{ ruleId: "API-001", applicability: "applicable", state: "evaluated" }] });
      const completeSummary = ruleExecutionCoverageSummary(completeReport);
      expect(evaluationCoverageAdvisory(evaluationCoverageStatus(completeSummary), completeSummary)).toBe("");
    });

    test("test 12: a distinct normalized-legacy advisory is shown, never the plain partial-coverage wording", () => {
      const normalizedReport = parseFindingsDocument({
        findings: [],
        ruleExecutions: [{ ruleId: "API-001", applicability: "applicable", state: "evaluated" }],
        ruleExecutionsNormalized: true,
      });
      const normalizedSummary = ruleExecutionCoverageSummary(normalizedReport);
      const normalizedAdvisory = evaluationCoverageAdvisory(evaluationCoverageStatus(normalizedSummary), normalizedSummary);
      expect(normalizedAdvisory).not.toBe("");
      expect(normalizedAdvisory).toContain("normalized");
      expect(normalizedAdvisory).not.toContain("not fully evaluated");
    });

    test("scoreQualification text matches the Go side's required substrings", () => {
      expect(scoreQualification).toContain("not penalized");
      expect(scoreQualification).toContain("evaluated checks");
    });

    test("evaluationCoverageStatusLabel renders the exact 4 fixed labels", () => {
      expect(evaluationCoverageStatusLabel("complete")).toBe("Complete");
      expect(evaluationCoverageStatusLabel("partial")).toBe("Partial");
      expect(evaluationCoverageStatusLabel("unavailable")).toBe("Unavailable");
      expect(evaluationCoverageStatusLabel("normalized_legacy")).toBe("Normalized legacy");
    });
  });
});
