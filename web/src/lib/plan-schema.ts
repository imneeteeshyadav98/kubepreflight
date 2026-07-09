// Parses and validates a KubePreflight upgrade-plan.json document. Mirrors
// findings-schema.ts's style exactly (dual string/object entry, plain
// Error throws, defensive-not-exhaustive checks) and reuses
// parseFindingsDocument for each hop's nested report rather than
// reimplementing finding validation.

import { parseFindingsDocument, type Report } from "./findings-schema";

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

export interface UpgradeActionPlan {
  schemaVersion: string;
  verdict: string;
  generatedAt: string;
  phases: ActionPhase[];
}

export interface ActionPhase {
  id: string;
  title: string;
  description?: string;
  gate?: string;
  actions: PlanAction[];
}

export interface PlanAction {
  id: string;
  title: string;
  required: boolean;
  status: string;
  reason?: string;
  sourceRuleIds?: string[];
  successCriteria?: string[];
  commands?: string[];
}

export interface PlanReport {
  schemaVersion: string;
  clusterContext?: string;
  provider?: string;
  fromVersion: string;
  fromVersionSource?: string;
  toVersion: string;
  generatedAt: string;
  hops: HopReport[];
  actionPlan?: UpgradeActionPlan;
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

	const hops = raw.hops.map((hop, index) => normalizeHopReport(hop, index));
	if (hops[0].status !== "EXACT" || !hops[0].report) throw new Error("hops[0] must be an EXACT hop with a report.");
	hops.forEach((hop, index) => {
	  if (hop.hop.index !== index + 1) throw new Error(`hops[${index}].hop.index must be ${index + 1}.`);
	  if (index > 0 && (hop.status !== "PREDICTED" || hops[index - 1].hop.to !== hop.hop.from)) throw new Error(`hops[${index}] must be PREDICTED and continue the previous hop.`);
	});
	return {
    ...raw,
		schemaVersion: stringOr(raw.schemaVersion, "legacy"),
    clusterContext: stringOrUndefined(raw.clusterContext),
    provider: stringOrUndefined(raw.provider),
    fromVersion: stringOr(raw.fromVersion, "Unknown"),
    fromVersionSource: stringOrUndefined(raw.fromVersionSource),
    toVersion: stringOr(raw.toVersion, "Unknown"),
    generatedAt: stringOr(raw.generatedAt, ""),
		hops,
    actionPlan: normalizeActionPlan(raw.actionPlan),
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

function normalizeActionPlan(value: unknown): UpgradeActionPlan | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const raw = value as Record<string, unknown>;
  return {
    schemaVersion: stringOr(raw.schemaVersion, "legacy"),
    verdict: stringOr(raw.verdict, "UNKNOWN"),
    generatedAt: stringOr(raw.generatedAt, ""),
    phases: Array.isArray(raw.phases) ? raw.phases.map(normalizeActionPhase) : [],
  };
}

function normalizeActionPhase(value: unknown): ActionPhase {
  const raw = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  return {
    id: stringOr(raw.id, ""),
    title: stringOr(raw.title, "Untitled phase"),
    description: stringOrUndefined(raw.description),
    gate: stringOrUndefined(raw.gate),
    actions: Array.isArray(raw.actions) ? raw.actions.map(normalizePlanAction) : [],
  };
}

function normalizePlanAction(value: unknown): PlanAction {
  const raw = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  return {
    id: stringOr(raw.id, ""),
    title: stringOr(raw.title, "Untitled action"),
    required: typeof raw.required === "boolean" ? raw.required : false,
    status: stringOr(raw.status, "manual"),
    reason: stringOrUndefined(raw.reason),
    sourceRuleIds: stringArrayOrUndefined(raw.sourceRuleIds),
    successCriteria: stringArrayOrUndefined(raw.successCriteria),
    commands: stringArrayOrUndefined(raw.commands),
  };
}

// planVerdict mirrors PlanReport.Verdict() (internal/plan/plan.go) exactly
// — same priority order: incomplete coverage outranks even a global
// blocker finding (an assessment built on partial evidence isn't a
// fully-trusted "not ready" verdict either, though any blockers found
// with the evidence that WAS collected are still named in the reason,
// never hidden), which outranks a global blocker, which outranks the
// generic blocker count, which outranks warnings, which outranks clean.
export function planVerdict(hop1Report?: Report): { label: string; reason: string } {
  if (!hop1Report) {
    return { label: "READY", reason: "No known upgrade blockers detected" };
  }

  if (hop1Report.result === "INCOMPLETE") {
    if (hop1Report.summary.blockers > 0) {
      return {
        label: "ASSESSMENT INCOMPLETE",
        reason: `Assessment incomplete; ${hop1Report.summary.blockers} blocker(s) observed with available evidence. One or more evidence sources could not be collected — resolve coverage errors and rerun.`,
      };
    }
    return { label: "ASSESSMENT INCOMPLETE", reason: "One or more evidence sources could not be collected; resolve coverage errors and rerun" };
  }

  if (hop1Report.findings.some((finding) => finding.globalBlocker)) {
    return { label: "NOT READY FOR UPGRADE", reason: "Global API write blocker detected" };
  }
  if (hop1Report.result === "BLOCKED") {
    return { label: "NOT READY FOR UPGRADE", reason: `${hop1Report.summary.blockers} blocker(s) found` };
  }
  if (hop1Report.result === "PASSED_WITH_WARNINGS") {
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

function stringArrayOrUndefined(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined;
  return value.filter((item): item is string => typeof item === "string");
}
