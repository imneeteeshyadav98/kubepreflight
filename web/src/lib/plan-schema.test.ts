import { expect, test } from "vitest";
import type { Finding } from "./findings-schema";
import { parsePlanDocument, planVerdict } from "./plan-schema";

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

const hop1 = {
  hop: { index: 1, from: "1.29", to: "1.30" },
  status: "EXACT",
  report: { targetVersion: "1.30", findings: [baseFinding], summary: { blockers: 1, warnings: 0, infos: 0 } },
};

const hop2 = {
  hop: { index: 2, from: "1.30", to: "1.31" },
  status: "PREDICTED",
  carryForward: [{ ruleId: "NODE-001", reason: "nodes may be replaced before this hop", recommendedCommand: "kubepreflight scan --target-version 1.31" }],
};

test("parses a canonical plan document", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.31",
    generatedAt: "2026-07-05T00:00:00Z",
    actionPlan: {
      schemaVersion: "kubepreflight.io/upgrade-action-plan/v1",
      verdict: "BLOCKED",
      generatedAt: "2026-07-05T00:00:00Z",
      phases: [{ id: "phase-1-critical-blockers", title: "Phase 1 - Critical Blockers", actions: [{ id: "fix-api-compatibility", title: "Fix removed APIs", required: true, status: "required", sourceRuleIds: ["API-001"] }] }],
    },
    hops: [hop1, hop2],
  });
  expect(plan.hops).toHaveLength(2);
  expect(plan.hops[0].status).toBe("EXACT");
  expect(plan.hops[0].report?.findings[0].ruleId).toBe("PDB-001");
  expect(plan.hops[1].carryForward?.[0].ruleId).toBe("NODE-001");
  expect(plan.actionPlan?.phases[0].actions[0].sourceRuleIds).toEqual(["API-001"]);
});

test("a predicted future hop with no report does not throw — report is legitimately nil-able", () => {
	const plan = parsePlanDocument({
	  fromVersion: "1.29",
	  toVersion: "1.31",
	  hops: [hop1, { hop: { index: 2, from: "1.30", to: "1.31" }, status: "PREDICTED" }],
	});
	expect(plan.hops[1].report).toBeUndefined();
});

test("rejects malformed plan documents instead of rendering partial data", () => {
  expect(() => parsePlanDocument({ fromVersion: "1.29", toVersion: "1.30" })).toThrow(/hops/);
  expect(() => parsePlanDocument({ fromVersion: "1.29", toVersion: "1.30", hops: [] })).toThrow(/hops/);
  expect(() => parsePlanDocument("not json")).toThrow(/Invalid JSON/);
  expect(() =>
    parsePlanDocument({ fromVersion: "1.29", toVersion: "1.30", hops: [{ hop: { from: "1.29", to: "1.30" }, status: "UNKNOWN" }] }),
  ).toThrow(/status/);
});

test("a malformed nested hop report surfaces parseFindingsDocument's own error, not a new error path", () => {
  expect(() =>
    parsePlanDocument({
      fromVersion: "1.29",
      toVersion: "1.30",
      hops: [{ hop: { index: 1, from: "1.29", to: "1.30" }, status: "EXACT", report: { findings: [{ ...baseFinding, resources: [] }] } }],
    }),
  ).toThrow(/no resources/);
});

test("planVerdict: global blocker beats the generic blocker count", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [
      {
        hop: { index: 1, from: "1.29", to: "1.30" },
        status: "EXACT",
        report: {
          findings: [baseFinding, { ...baseFinding, ruleId: "WH-002", fingerprint: "fp-2", globalBlocker: true }],
          summary: { blockers: 2, warnings: 0, infos: 0 },
        },
      },
    ],
  });
  expect(planVerdict(plan.hops[0].report)).toEqual({ label: "NOT READY FOR UPGRADE", reason: "Global API write blocker detected" });
});

test("planVerdict: blocked without a global blocker", () => {
  const plan = parsePlanDocument({ fromVersion: "1.29", toVersion: "1.30", hops: [hop1] });
  expect(planVerdict(plan.hops[0].report)).toEqual({ label: "NOT READY FOR UPGRADE", reason: "1 blocker(s) found" });
});

test("planVerdict: warnings only", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [
      {
        hop: { index: 1, from: "1.29", to: "1.30" },
        status: "EXACT",
        report: { findings: [{ ...baseFinding, severity: "Warning" }], summary: { blockers: 0, warnings: 1, infos: 0 } },
      },
    ],
  });
  expect(planVerdict(plan.hops[0].report)).toEqual({
    label: "CONDITIONALLY READY",
    reason: "No hard blockers, but 1 warning(s) should be reviewed",
  });
});

// Guards the exact regression found in review: planVerdict must apply the
// same priority order as Go's PlanReport.Verdict() — incomplete coverage
// outranks even a global blocker finding, not just the generic blocker
// count. Before the fix, planVerdict checked globalBlocker/BLOCKED before
// ever consulting "INCOMPLETE", so this exact scenario returned "NOT READY
// FOR UPGRADE" instead of "ASSESSMENT INCOMPLETE".
test("planVerdict: incomplete coverage outranks a global blocker and the generic blocker count", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [
      {
        hop: { index: 1, from: "1.29", to: "1.30" },
        status: "EXACT",
        report: {
          findings: [baseFinding, { ...baseFinding, ruleId: "WH-002", fingerprint: "fp-2", globalBlocker: true }],
          summary: { blockers: 2, warnings: 0, infos: 0 },
          coverage: { kubernetes: { status: "partial", errors: ["pods: forbidden"] } },
        },
      },
    ],
  });
  const verdict = planVerdict(plan.hops[0].report);
  expect(verdict.label).toBe("ASSESSMENT INCOMPLETE");
  expect(verdict.reason).toContain("2 blocker(s) observed with available evidence");
});

test("planVerdict: incomplete coverage with no blockers still reports ASSESSMENT INCOMPLETE", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [
      {
        hop: { index: 1, from: "1.29", to: "1.30" },
        status: "EXACT",
        report: {
          findings: [],
          summary: { blockers: 0, warnings: 0, infos: 0 },
          coverage: { kubernetes: { status: "partial", errors: ["pods: forbidden"] } },
        },
      },
    ],
  });
  expect(planVerdict(plan.hops[0].report)).toEqual({
    label: "ASSESSMENT INCOMPLETE",
    reason: "One or more evidence sources could not be collected; resolve coverage errors and rerun",
  });
});

test("planVerdict: clean and no-hop-1-report both default to READY", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [{ hop: { index: 1, from: "1.29", to: "1.30" }, status: "EXACT", report: { findings: [], summary: { blockers: 0, warnings: 0, infos: 0 } } }],
  });
  expect(planVerdict(plan.hops[0].report)).toEqual({ label: "READY", reason: "No known upgrade blockers detected" });
  expect(planVerdict(undefined)).toEqual({ label: "READY", reason: "No known upgrade blockers detected" });
});
