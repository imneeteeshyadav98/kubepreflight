import assert from "node:assert/strict";
import test from "node:test";
import { filterFindings, parseFindingsDocument, resultFromSummary } from "../lib/findings-schema.mjs";

const baseFinding = {
  ruleId: "PDB-001", severity: "Blocker", confidence: "STATIC_CERTAIN", message: "PDB blocks drain",
  resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "payments", name: "critical-pdb" }],
  evidence: ["disruptionsAllowed: 0"], remediation: "Scale replicas.", fingerprint: "fp-1",
};

test("parses canonical findings and derives the decision", () => {
  const report = parseFindingsDocument({ targetVersion: "1.36", findings: [baseFinding], summary: { blockers: 999 } });
  assert.equal(report.summary.blockers, 1);
  assert.equal(report.result, "BLOCKED");
});

test("accepts the legacy singular resource shape", () => {
  const { resources, ...withoutResources } = baseFinding;
  const report = parseFindingsDocument({ findings: [{ ...withoutResources, resource: resources[0] }] });
  assert.equal(report.findings[0].resources[0].name, "critical-pdb");
});

test("rejects malformed documents instead of rendering partial data", () => {
  assert.throws(() => parseFindingsDocument({ findings: [{ ...baseFinding, resources: [] }] }), /no resources/);
  assert.throws(() => parseFindingsDocument("not json"), /Invalid JSON/);
});

test("filters by namespace, confidence, severity, and search", () => {
  const findings = parseFindingsDocument({ findings: [baseFinding, { ...baseFinding, ruleId: "WH-001", severity: "Warning", fingerprint: "fp-2", resources: [{ plane: "live", kind: "Webhook", name: "guard" }] }] }).findings;
  assert.equal(filterFindings(findings, { namespace: "payments" }).length, 1);
  assert.equal(filterFindings(findings, { severity: "Warning" })[0].ruleId, "WH-001");
  assert.equal(filterFindings(findings, { confidence: "STATIC_CERTAIN" }).length, 2);
  assert.equal(filterFindings(findings, { search: "critical-pdb" })[0].ruleId, "PDB-001");
});

test("maps summaries to stable result labels", () => {
  assert.equal(resultFromSummary({ blockers: 0, warnings: 0 }), "CLEAN");
  assert.equal(resultFromSummary({ blockers: 0, warnings: 1 }), "PASSED_WITH_WARNINGS");
});
