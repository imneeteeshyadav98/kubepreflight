import { describe, expect, test } from "vitest";
import { compareReports, type Comparison } from "./comparison-schema";
import { parseFindingsDocument, type Finding, type Report, type RuleExecutionRecord } from "./findings-schema";

function baseFinding(overrides: Partial<Finding> = {}): Finding {
  return {
    ruleId: "PDB-001",
    severity: "Blocker",
    confidence: "OBSERVED",
    message: "PDB blocks drain",
    resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "default", name: "api" }],
    evidence: ["disruptionsAllowed: 0"],
    remediation: "Scale replicas.",
    fingerprint: "fp-pdb-001",
    priority: "P3",
    priorityReason: "Node drain may fail.",
    affectedScope: "workload",
    canUpgradeContinue: false,
    ...overrides,
  };
}

function report(findings: Finding[], targetVersion = "1.36"): Report {
  return parseFindingsDocument({ targetVersion, clusterContext: "test", findings, summary: {} });
}

// reportWithExecutions builds a report carrying an explicit ruleExecutions
// array, for tests exercising compareReports' not_re_evaluated
// classification -- mirrors internal/comparison/compare_test.go's
// cmpReport()+manual RuleExecutions assignment.
function reportWithExecutions(findings: Finding[], ruleExecutions: RuleExecutionRecord[], targetVersion = "1.36"): Report {
  return parseFindingsDocument({ targetVersion, clusterContext: "test", findings, summary: {}, ruleExecutions });
}

describe("compareReports", () => {
  test("empty vs empty", () => {
    const c = compareReports(report([]), report([]));
    expect(c.new).toHaveLength(0);
    expect(c.resolved).toHaveLength(0);
    expect(c.changed).toHaveLength(0);
    expect(c.unchanged).toHaveLength(0);
    expect(c.summary.verdictChanged).toBe(false);
  });

  test("finding added shows up as new", () => {
    const f = baseFinding();
    const c = compareReports(report([]), report([f]));
    expect(c.new.map((x) => x.fingerprint)).toEqual([f.fingerprint]);
    expect(c.summary.new).toBe(1);
    expect(c.summary.newBlockers).toBe(1);
  });

  test("finding resolved shows up as resolved when the rule proves it evaluated cleanly", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "evaluated" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved.map((x) => x.fingerprint)).toEqual([f.fingerprint]);
    expect(c.summary.resolved).toBe(1);
    expect(c.summary.resolvedBlockers).toBe(1);
    expect(c.not_re_evaluated).toHaveLength(0);
    expect(c.summary.not_re_evaluated).toBe(0);
  });

  test("finding absent with no ruleExecutions data at all is not_re_evaluated, not resolved", () => {
    const f = baseFinding();
    const c = compareReports(report([f]), report([]));
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated.map((x) => x.fingerprint)).toEqual([f.fingerprint]);
    expect(c.summary.resolved).toBe(0);
    expect(c.summary.not_re_evaluated).toBe(1);
  });

  test("finding absent when the rule was not_evaluated is not_re_evaluated", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "not_evaluated" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated.map((x) => x.fingerprint)).toEqual([f.fingerprint]);
  });

  test("finding absent when the rule hit insufficient_evidence is not_re_evaluated", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "insufficient_evidence" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated).toHaveLength(1);
  });

  test("finding absent when the rule failed is not_re_evaluated", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "failed" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated).toHaveLength(1);
  });

  test("finding absent when the rule is not_applicable is not_re_evaluated", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "not_applicable", state: "evaluated" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated).toHaveLength(1);
  });

  test("finding absent with no matching rule-execution record for that rule ID is not_re_evaluated", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "WH-001", applicability: "applicable", state: "evaluated" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated).toHaveLength(1);
  });

  test("unknown historical rule ID absent from current's canonical records is not_re_evaluated", () => {
    const f = baseFinding({ ruleId: "RETIRED-999", fingerprint: "fp-retired" });
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "evaluated" }]);
    const c = compareReports(report([f]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated).toHaveLength(1);
  });

  test("duplicate baseline findings sharing a rule ID are each independently classified", () => {
    const a = baseFinding({ fingerprint: "fp-a", resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "default", name: "api-a" }] });
    const b = baseFinding({ fingerprint: "fp-b", resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "default", name: "api-b" }] });
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "not_evaluated" }]);
    const c = compareReports(report([a, b]), current);
    expect(c.resolved).toHaveLength(0);
    expect(c.not_re_evaluated.map((f) => f.fingerprint).sort()).toEqual(["fp-a", "fp-b"]);
  });

  test("identical finding is unchanged", () => {
    const f = baseFinding();
    const c = compareReports(report([f]), report([f]));
    expect(c.unchanged).toHaveLength(1);
    expect(c.new).toHaveLength(0);
    expect(c.resolved).toHaveLength(0);
    expect(c.changed).toHaveLength(0);
  });

  test("severity change is tracked", () => {
    const before = baseFinding({ severity: "Blocker" });
    const after = baseFinding({ severity: "Warning" });
    const c = compareReports(report([before]), report([after]));
    expect(c.changed).toHaveLength(1);
    expect(c.changed[0].changes.severity).toEqual({ before: "Blocker", after: "Warning" });
  });

  test("priority change is tracked", () => {
    const before = baseFinding({ priority: "P3" });
    const after = baseFinding({ priority: "P1" });
    const c = compareReports(report([before]), report([after]));
    expect(c.changed[0].changes.priority).toEqual({ before: "P3", after: "P1" });
  });

  test("canUpgradeContinue change is tracked", () => {
    const before = baseFinding({ canUpgradeContinue: true });
    const after = baseFinding({ canUpgradeContinue: false });
    const c = compareReports(report([before]), report([after]));
    expect(c.changed[0].changes.canUpgradeContinue).toEqual({ before: "true", after: "false" });
  });

  test("evidence/remediation text changes are never tracked as Changed", () => {
    const before = baseFinding({ evidence: ["old evidence"], remediation: "old remediation text" });
    const after = baseFinding({ evidence: ["new evidence"], remediation: "new remediation text" });
    const c = compareReports(report([before]), report([after]));
    expect(c.changed).toHaveLength(0);
    expect(c.unchanged).toHaveLength(1);
  });

  test("multiple simultaneous changes on one fingerprint", () => {
    const before = baseFinding({ severity: "Warning", priority: "P4", canUpgradeContinue: true });
    const after = baseFinding({ severity: "Blocker", priority: "P1", canUpgradeContinue: false });
    const c = compareReports(report([before]), report([after]));
    expect(c.changed).toHaveLength(1);
    expect(Object.keys(c.changed[0].changes).sort()).toEqual(["canUpgradeContinue", "priority", "severity", "upgradeGate"]);
  });

  test("readiness score and verdict movement", () => {
    const blocker = baseFinding();
    const c = compareReports(report([]), report([blocker]));
    expect(c.summary.verdictChanged).toBe(true);
    expect(c.summary.baselineVerdict).toBe("CLEAN");
    expect(c.summary.currentVerdict).toBe("BLOCKED");
    expect(c.summary.readinessScoreDelta).toBeLessThan(0);
  });

  test("duplicate fingerprints throw", () => {
    const f = baseFinding();
    expect(() => compareReports(report([f, f]), report([]))).toThrow(/duplicate fingerprint/);
  });

  test("input ordering does not affect output ordering", () => {
    const a = baseFinding({ ruleId: "PDB-001", fingerprint: "fp-a", resources: [{ plane: "live", kind: "PodDisruptionBudget", namespace: "default", name: "api" }] });
    const b = baseFinding({ ruleId: "WH-001", severity: "Warning", priority: "P4", fingerprint: "fp-b", resources: [{ plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name: "guard" }] });
    const d = baseFinding({ ruleId: "NODE-001", fingerprint: "fp-d", resources: [{ plane: "live", kind: "Node", namespace: "", name: "node-1" }] });

    const forward = compareReports(report([]), report([a, b, d]));
    const reversed = compareReports(report([]), report([d, b, a]));
    expect(forward.new.map((f) => f.fingerprint)).toEqual(reversed.new.map((f) => f.fingerprint));
  });

  test("target-version mismatch produces a warning", () => {
    const c = compareReports(report([], "1.35"), report([], "1.36"));
    expect(c.warnings.length).toBeGreaterThan(0);
    expect(c.warnings[0]).toMatch(/target-version/);
  });

  test("blockers sort before non-blockers within new/resolved", () => {
    const warning = baseFinding({ ruleId: "AAA-001", severity: "Warning", priority: "P4", fingerprint: "fp-warn", resources: [{ plane: "live", kind: "X", namespace: "", name: "a" }] });
    const blocker = baseFinding({ ruleId: "ZZZ-001", severity: "Blocker", priority: "P4", fingerprint: "fp-block", resources: [{ plane: "live", kind: "X", namespace: "", name: "z" }] });
    const c = compareReports(report([]), report([warning, blocker]));
    expect(c.new.map((f) => f.fingerprint)).toEqual(["fp-block", "fp-warn"]);
  });

  test("real deterministic golden output shape", () => {
    const f = baseFinding();
    const c: Comparison = compareReports(report([]), report([f]));
    expect(c.schemaVersion).toBe("kubepreflight.io/scan-comparison/v1");
    expect(c.summary).toMatchObject({
      baselineVerdict: "CLEAN",
      currentVerdict: "BLOCKED",
      verdictChanged: true,
      new: 1,
      resolved: 0,
      changed: 0,
      unchanged: 0,
      newBlockers: 1,
      resolvedBlockers: 0,
      not_re_evaluated: 0,
    });
    expect(c.not_re_evaluated).toEqual([]);
  });

  test("JSON round-trip preserves the not_re_evaluated bucket under its locked snake_case key", () => {
    const f = baseFinding();
    const current = reportWithExecutions([], [{ ruleId: "PDB-001", applicability: "applicable", state: "failed" }]);
    const c = compareReports(report([f]), current);

    const raw = JSON.stringify(c);
    expect(raw).toContain('"not_re_evaluated":[');
    const roundTripped = JSON.parse(raw) as Comparison;
    expect(roundTripped.not_re_evaluated.map((x) => x.fingerprint)).toEqual([f.fingerprint]);
    expect(roundTripped.summary.not_re_evaluated).toBe(1);
  });
});
