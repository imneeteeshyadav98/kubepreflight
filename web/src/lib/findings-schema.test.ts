import { describe, expect, test } from "vitest";
import { filterFindings, parseFindingsDocument, resultFromSummary, upgradeContext, upgradeDetails, type Finding } from "./findings-schema";

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
