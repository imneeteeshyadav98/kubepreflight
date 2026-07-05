// Parses and validates a KubePreflight upgrade-plan.json document. Mirrors
// findings-schema.ts's style exactly (dual string/object entry, plain
// Error throws, defensive-not-exhaustive checks) and reuses
// parseFindingsDocument for each hop's nested report rather than
// reimplementing finding validation.

import { parseFindingsDocument, resultFromSummary, type Report } from "./findings-schema";

export type HopStatus = "EXACT" | "PREDICTED";
const HOP_STATUSES: HopStatus[] = ["EXACT", "PREDICTED"];

export interface Hop {
  index: number;
  from: string;
  to: string;
}

export interface CarryForwardNote {
  ruleId: string;
  reason: string;
  recommendedCommand: string;
}

export interface HopReport {
  hop: Hop;
  status: HopStatus;
  // report is legitimately absent for a predicted hop with nothing
  // honestly re-evaluated for it — mirrors Go's `json:"report,omitempty"`.
  report?: Report;
  carryForward?: CarryForwardNote[];
}

export interface PlanReport {
  clusterContext?: string;
  provider?: string;
  fromVersion: string;
  fromVersionSource?: string;
  toVersion: string;
  generatedAt: string;
  hops: HopReport[];
}

export function parsePlanDocument(input: unknown): PlanReport {
  let value: unknown = input;
  if (typeof input === "string") {
    try {
      value = JSON.parse(input);
    } catch (error) {
      throw new Error(`Invalid JSON: ${(error as Error).message}`);
    }
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("The file must contain a JSON object.");
  }
  const raw = value as Record<string, unknown>;
  if (!Array.isArray(raw.hops) || raw.hops.length === 0) {
    throw new Error("Missing required hops[] array.");
  }

  return {
    ...raw,
    clusterContext: stringOrUndefined(raw.clusterContext),
    provider: stringOrUndefined(raw.provider),
    fromVersion: stringOr(raw.fromVersion, "Unknown"),
    fromVersionSource: stringOrUndefined(raw.fromVersionSource),
    toVersion: stringOr(raw.toVersion, "Unknown"),
    generatedAt: stringOr(raw.generatedAt, ""),
    hops: raw.hops.map((hop, index) => normalizeHopReport(hop, index)),
  } as PlanReport;
}

function normalizeHopReport(value: unknown, index: number): HopReport {
  if (!value || typeof value !== "object") throw new Error(`hops[${index}] must be an object.`);
  const raw = value as Record<string, unknown>;

  const hopValue = raw.hop as Record<string, unknown> | undefined;
  if (!hopValue || typeof hopValue !== "object") throw new Error(`hops[${index}].hop must be an object.`);
  const hop: Hop = {
    index: typeof hopValue.index === "number" ? hopValue.index : index + 1,
    from: stringOr(hopValue.from, "?"),
    to: stringOr(hopValue.to, "?"),
  };

  const status = stringOr(raw.status, "PREDICTED");
  if (!HOP_STATUSES.includes(status as HopStatus)) {
    throw new Error(`hops[${index}].status is not EXACT or PREDICTED.`);
  }

  const report = raw.report ? parseFindingsDocument(raw.report) : undefined;

  const carryForward = Array.isArray(raw.carryForward)
    ? raw.carryForward.map((note, noteIndex) => normalizeCarryForwardNote(note, index, noteIndex))
    : undefined;

  return { hop, status: status as HopStatus, report, carryForward };
}

function normalizeCarryForwardNote(value: unknown, hopIndex: number, noteIndex: number): CarryForwardNote {
  if (!value || typeof value !== "object") {
    throw new Error(`hops[${hopIndex}].carryForward[${noteIndex}] must be an object.`);
  }
  const raw = value as Record<string, unknown>;
  return {
    ruleId: stringOr(raw.ruleId, "UNKNOWN"),
    reason: stringOr(raw.reason, "No reason supplied."),
    recommendedCommand: stringOr(raw.recommendedCommand, ""),
  };
}

// planVerdict mirrors PlanReport.Verdict() (internal/plan/plan.go) exactly
// — same priority order (global blocker beats the generic blocker count,
// which beats warnings, which beats clean) — so the Console shows the
// identical readiness decision the HTML planner report already shows.
export function planVerdict(hop1Report?: Report): { label: string; reason: string } {
  if (!hop1Report) {
    return { label: "READY", reason: "No known upgrade blockers detected" };
  }
  if (hop1Report.findings.some((finding) => finding.globalBlocker)) {
    return { label: "NOT READY FOR UPGRADE", reason: "Global API write blocker detected" };
  }
  const result = resultFromSummary(hop1Report.summary);
  if (result === "BLOCKED") {
    return { label: "NOT READY FOR UPGRADE", reason: `${hop1Report.summary.blockers} blocker(s) found` };
  }
  if (result === "PASSED_WITH_WARNINGS") {
    return {
      label: "CONDITIONALLY READY",
      reason: `No hard blockers, but ${hop1Report.summary.warnings} warning(s) should be reviewed`,
    };
  }
  return { label: "READY", reason: "No known upgrade blockers detected" };
}

function stringOr(value: unknown, fallback: string): string {
  return typeof value === "string" && value.trim() ? value : fallback;
}

function stringOrUndefined(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}
