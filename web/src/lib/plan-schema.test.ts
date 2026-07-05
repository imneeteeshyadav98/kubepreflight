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
  const plan = parsePlanDocument({ fromVersion: "1.29", toVersion: "1.31", generatedAt: "2026-07-05T00:00:00Z", hops: [hop1, hop2] });
  expect(plan.hops).toHaveLength(2);
  expect(plan.hops[0].status).toBe("EXACT");
  expect(plan.hops[0].report?.findings[0].ruleId).toBe("PDB-001");
  expect(plan.hops[1].carryForward?.[0].ruleId).toBe("NODE-001");
});

test("a hop with no report does not throw — report is legitimately nil-able", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [{ hop: { index: 1, from: "1.29", to: "1.30" }, status: "PREDICTED" }],
  });
  expect(plan.hops[0].report).toBeUndefined();
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

test("planVerdict: clean and no-hop-1-report both default to READY", () => {
  const plan = parsePlanDocument({
    fromVersion: "1.29",
    toVersion: "1.30",
    hops: [{ hop: { index: 1, from: "1.29", to: "1.30" }, status: "EXACT", report: { findings: [], summary: { blockers: 0, warnings: 0, infos: 0 } } }],
  });
  expect(planVerdict(plan.hops[0].report)).toEqual({ label: "READY", reason: "No known upgrade blockers detected" });
  expect(planVerdict(undefined)).toEqual({ label: "READY", reason: "No known upgrade blockers detected" });
});
