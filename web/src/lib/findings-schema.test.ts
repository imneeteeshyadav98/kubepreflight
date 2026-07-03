import { describe, expect, test } from "vitest";
import { filterFindings, parseFindingsDocument, resultFromSummary, type Finding } from "./findings-schema";

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
  expect(filterFindings(findings, { severity: "Warning" })[0].ruleId).toBe("WH-001");
  expect(filterFindings(findings, { confidence: "STATIC_CERTAIN" }).length).toBe(2);
  expect(filterFindings(findings, { search: "critical-pdb" })[0].ruleId).toBe("PDB-001");
});

test("maps summaries to stable result labels", () => {
  expect(resultFromSummary({ blockers: 0, warnings: 0, infos: 0 })).toBe("CLEAN");
  expect(resultFromSummary({ blockers: 0, warnings: 1, infos: 0 })).toBe("PASSED_WITH_WARNINGS");
});

describe("resource identity fallbacks", () => {
  test("defaults plane from sourcePath/providerId when absent", () => {
    const report = parseFindingsDocument({
      findings: [{ ...baseFinding, resources: [{ kind: "Deployment", name: "api", namespace: "", sourcePath: "deploy/api.yaml" }] }],
    });
    expect(report.findings[0].resources[0].plane).toBe("manifest");
  });
});
