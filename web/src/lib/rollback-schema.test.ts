import { parseRollbackAssessment, rollbackDecisionLabel, rollbackStatusClass } from "./rollback-schema";

test("parses rollback assessment document", () => {
  const assessment = parseRollbackAssessment(
    JSON.stringify({
      schemaVersion: "kubepreflight.io/rollback-assessment/v1alpha1",
      mode: "post_upgrade_readiness",
      cluster: { name: "prod", currentVersion: "1.36", rollbackTargetVersion: "1.35", provider: "eks" },
      eligibility: { status: "eligible", reasonCodes: [] },
      readiness: { status: "high_risk", blockers: 0, warnings: 1, unknowns: 0 },
      recommendation: { decision: "fix_forward_preferred", confidence: "medium", reasonCodes: ["MANAGED_NODEGROUP_ROLLBACK_REQUIRED"] },
      evidence: { complete: true },
      checks: [{ id: "managed-nodegroups", title: "Managed node groups", status: "warning", reasonCodes: ["MANAGED_NODEGROUP_ROLLBACK_REQUIRED"], evidence: ["nodegroup apps version: 1.36"] }],
    }),
  );

  expect(assessment.schemaVersion).toBe("kubepreflight.io/rollback-assessment/v1alpha1");
  expect(assessment.recommendation.decision).toBe("fix_forward_preferred");
  expect(assessment.checks[0].reasonCodes).toEqual(["MANAGED_NODEGROUP_ROLLBACK_REQUIRED"]);
});

test("rejects unsupported rollback assessment schema", () => {
  expect(() => parseRollbackAssessment(JSON.stringify({ schemaVersion: "unknown" }))).toThrow(/Unsupported rollback assessment schema/);
});

test("formats rollback labels and classes", () => {
  expect(rollbackDecisionLabel("operator_decision_required")).toBe("operator decision required");
  expect(rollbackStatusClass("rollback_preferred")).toBe("clean");
  expect(rollbackStatusClass("do_not_proceed")).toBe("blocked");
  expect(rollbackStatusClass("fix_forward_preferred")).toBe("warning");
});
