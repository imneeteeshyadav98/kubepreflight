import {
  eligibilityStatusLabel,
  parseRollbackAssessment,
  readinessStatusLabel,
  rollbackDecisionLabel,
  rollbackReasonCodeLabel,
  rollbackStatusClass,
} from "./rollback-schema";

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

// Every ReasonCode constant currently defined in internal/rollback/model.go
// (UX-001 / audit finding F1) must resolve to a real, non-fallback human
// label — this list is the TypeScript mirror of that Go const block. Keep
// both in sync when a new ReasonCode is added there.
const KNOWN_ROLLBACK_REASON_CODES = [
  "UPGRADE_HISTORY_UNAVAILABLE",
  "EKS_UPGRADE_HISTORY_UNAVAILABLE",
  "UPGRADE_WAS_NOT_IN_PLACE",
  "ROLLBACK_WINDOW_EXPIRED",
  "ROLLBACK_WINDOW_NEAR_EXPIRY",
  "PREVIOUS_VERSION_NOT_N_MINUS_ONE",
  "CLUSTER_NOT_ACTIVE",
  "ROLLBACK_TARGET_UNSUPPORTED",
  "ROLLBACK_TARGET_REQUIRES_EXTENDED_SUPPORT",
  "UPGRADE_POLICY_DISALLOWS_ROLLBACK_TARGET",
  "END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE",
  "END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED",
  "EKS_FEATURE_COMPATIBILITY_UNVERIFIED",
  "EKS_FEATURE_INCOMPATIBLE",
  "INCOMPATIBLE_EKS_FEATURE_ENABLED",
  "EKS_INSIGHTS_UNAVAILABLE",
  "EKS_INSIGHTS_STALE",
  "EKS_INSIGHTS_BLOCKING",
  "MANAGED_NODEGROUP_ROLLBACK_REQUIRED",
  "SELF_MANAGED_NODE_EVIDENCE_UNAVAILABLE",
  "SELF_MANAGED_NODE_ROLLBACK_REQUIRED",
  "WORKER_NODES_WOULD_REMAIN_NEWER_THAN_CONTROL_PLANE",
  "AUTO_MODE_DISRUPTION_RISK",
  "FARGATE_EVIDENCE_UNAVAILABLE",
  "FARGATE_POD_RECREATION_RISK",
  "MANAGED_ADDON_ROLLBACK_REQUIRED",
  "MANAGED_ADDON_COMPATIBILITY_UNKNOWN",
  "SELF_MANAGED_ADDON_COMPATIBILITY_UNKNOWN",
  "NEW_VERSION_API_ADOPTION_RISK",
  "CRD_WEBHOOK_CONTROLLER_RISK",
  "PDB_DISRUPTION_CONSTRAINTS",
  "UNHEALTHY_WORKLOADS_PRESENT",
  "OBSERVABILITY_EVIDENCE_MISSING",
  "APPLICATION_HEALTH_UNKNOWN",
  "FORCE_BYPASS_NOT_RECOMMENDED",
  "ROLLBACK_EVIDENCE_TARGET_MISMATCH",
  "ROLLBACK_EVIDENCE_TARGET_UNKNOWN",
  "ROLLBACK_EVIDENCE_CLUSTER_MISMATCH",
  "ROLLBACK_EVIDENCE_CLUSTER_UNKNOWN",
  "ROLLBACK_EVIDENCE_STALE",
  "ROLLBACK_EVIDENCE_TIMESTAMP_UNKNOWN",
];

test("every known rollback reason code has a real human label", () => {
  expect(KNOWN_ROLLBACK_REASON_CODES.length).toBe(41);
  for (const code of KNOWN_ROLLBACK_REASON_CODES) {
    const label = rollbackReasonCodeLabel(code);
    expect(label.title).not.toBe("Unknown rollback reason");
    expect(label.title.length).toBeGreaterThan(0);
  }
});

test("rollbackReasonCodeLabel falls back safely for an unmapped/future code", () => {
  expect(rollbackReasonCodeLabel("SOME_FUTURE_REASON_CODE_NOT_YET_MAPPED")).toEqual({
    title: "Unknown rollback reason",
    explanation: "",
  });
});

test("the acceptance-criteria example resolves as specified", () => {
  const label = rollbackReasonCodeLabel("ROLLBACK_EVIDENCE_TARGET_MISMATCH");
  expect(label.title).toBe("Evidence was scanned for a different target version");
});

test("eligibility and readiness statuses render human labels with a safe fallback", () => {
  expect(eligibilityStatusLabel("eligible")).toBe("Eligible");
  expect(eligibilityStatusLabel("unavailable")).toBe("Unavailable");
  expect(eligibilityStatusLabel("unknown")).toBe("Unknown");
  expect(eligibilityStatusLabel("some_future_status")).toBe("some_future_status");

  expect(readinessStatusLabel("ready")).toBe("Ready");
  expect(readinessStatusLabel("blocked")).toBe("Blocked");
  expect(readinessStatusLabel("high_risk")).toBe("High risk");
  expect(readinessStatusLabel("insufficient_evidence")).toBe("Insufficient evidence");
  expect(readinessStatusLabel("some_future_status")).toBe("some_future_status");
});
