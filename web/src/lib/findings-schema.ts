// Parses and validates a KubePreflight findings.json document. Ported from
// the original vanilla-JS Console (web/lib/findings-schema.mjs) with types
// added; the validation rules and defaulting behavior are unchanged.

export type Severity = "Blocker" | "Warning" | "Info";
const SEVERITIES: Severity[] = ["Blocker", "Warning", "Info"];

export interface ResourceReference {
  plane: string;
  kind: string;
  namespace: string;
  name: string;
  uid?: string;
  sourcePath?: string;
  providerId?: string;
  providerName?: string;
  scope?: string;
  [key: string]: unknown;
}

export interface Finding {
  ruleId: string;
  severity: Severity;
  confidence: string;
  message: string;
  evidence: string[];
  remediation: string;
  fingerprint: string;
  resources: ResourceReference[];
  // globalBlocker marks a finding that can block other remediation
  // commands (e.g. a fail-closed webhook with no healthy backend) — see
  // findings.Finding.GlobalBlocker (Go). Already carried through parsing
  // via normalizeFinding's spread; this just gives it a real type.
  globalBlocker?: boolean;
  remediationDetail?: RemediationDetail;
  [key: string]: unknown;
}

export interface RemediationChange { field?: string; current?: string; required?: string }
export interface RemediationAction { label: string; steps?: string[]; command?: string; risky?: boolean }
export interface RemediationDetail {
  affectedFile?: string;
  changes?: RemediationChange[];
  diff?: string;
  safeFix?: RemediationAction;
  emergency?: RemediationAction;
  breakGlass?: RemediationAction;
  verifyCommand?: string;
  expectedResult?: string;
}

export interface Summary {
  blockers: number;
  warnings: number;
  infos: number;
}

export type Result = "CLEAN" | "PASSED_WITH_WARNINGS" | "BLOCKED" | "INCOMPLETE";

export interface PlaneCoverage { status: "complete" | "partial" | "skipped"; errors: string[] }
export interface ScanCoverage { kubernetes: PlaneCoverage; aws: PlaneCoverage; manifests: PlaneCoverage }

export interface Report {
  schemaVersion: string;
  targetVersion: string;
  clusterContext: string;
  provider: string;
  scannedAt: string;
  assumptions: string[];
  namespaceAllowlist: string[];
  findings: Finding[];
  summary: Summary;
  result: Result;
  coverage: ScanCoverage;
  [key: string]: unknown;
}

export function parseFindingsDocument(input: unknown): Report {
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
  if (!Array.isArray(raw.findings)) throw new Error("Missing required findings[] array.");

  const findings = raw.findings.map((finding, index) => normalizeFinding(finding, index));
  const summary = deriveSummary(findings);
  const coverage = normalizeCoverage(raw.coverage);
  return {
    ...raw,
    schemaVersion: stringOr(raw.schemaVersion, "legacy"),
    targetVersion: stringOr(raw.targetVersion, "Unknown"),
    clusterContext: stringOr(raw.clusterContext, "Unspecified cluster"),
    provider: stringOr(raw.provider, "cluster-only"),
    scannedAt: stringOr(raw.scannedAt, ""),
    assumptions: Array.isArray(raw.assumptions) ? raw.assumptions.map(String) : [],
    namespaceAllowlist: Array.isArray(raw.namespaceAllowlist) ? raw.namespaceAllowlist.map(String) : [],
    findings,
    summary,
    coverage,
    result: resultFromSummary(summary, Object.values(coverage).some((plane) => plane.status === "partial")),
  };
}

function normalizeFinding(finding: unknown, index: number): Finding {
  if (!finding || typeof finding !== "object") throw new Error(`findings[${index}] must be an object.`);
  const raw = finding as Record<string, unknown>;
  const severity = stringOr(raw.severity, "Info");
  if (!SEVERITIES.includes(severity as Severity)) {
    throw new Error(`findings[${index}].severity is not Blocker, Warning, or Info.`);
  }
  const resources = Array.isArray(raw.resources)
    ? raw.resources.map((resource) => normalizeResource(resource))
    : raw.resource
      ? [normalizeResource(raw.resource)]
      : [];
  if (!resources.length) throw new Error(`findings[${index}] has no resources[].`);
	const remediationDetail = raw.remediationDetail === undefined ? undefined : normalizeRemediationDetail(raw.remediationDetail, index);
  return {
    ...raw,
    ruleId: stringOr(raw.ruleId, `UNKNOWN-${index + 1}`),
    severity: severity as Severity,
    confidence: stringOr(raw.confidence, "UNSPECIFIED"),
    message: stringOr(raw.message, "No finding message supplied."),
    evidence: Array.isArray(raw.evidence) ? raw.evidence.map(String) : [],
    remediation: stringOr(raw.remediation, "No remediation supplied."),
    fingerprint: stringOr(raw.fingerprint, "unavailable"),
    resources,
		...(remediationDetail ? { remediationDetail } : {}),
  };
}

function normalizeRemediationDetail(value: unknown, findingIndex: number): RemediationDetail {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(`findings[${findingIndex}].remediationDetail must be an object.`);
  const raw = value as Record<string, unknown>;
  const normalizeAction = (action: unknown, field: string): RemediationAction | undefined => {
    if (action === undefined) return undefined;
    if (!action || typeof action !== "object" || Array.isArray(action)) throw new Error(`findings[${findingIndex}].remediationDetail.${field} must be an object.`);
    const item = action as Record<string, unknown>;
    return { label: stringOr(item.label, field), steps: Array.isArray(item.steps) ? item.steps.map(String) : undefined, command: typeof item.command === "string" ? item.command : undefined, risky: item.risky === true };
  };
  return {
    affectedFile: typeof raw.affectedFile === "string" ? raw.affectedFile : undefined,
    changes: Array.isArray(raw.changes) ? raw.changes.map((change) => {
      const item = change && typeof change === "object" ? change as Record<string, unknown> : {};
      return { field: typeof item.field === "string" ? item.field : undefined, current: typeof item.current === "string" ? item.current : undefined, required: typeof item.required === "string" ? item.required : undefined };
    }) : undefined,
    diff: typeof raw.diff === "string" ? raw.diff : undefined,
    safeFix: normalizeAction(raw.safeFix, "safeFix"),
    emergency: normalizeAction(raw.emergency, "emergency"),
    breakGlass: normalizeAction(raw.breakGlass, "breakGlass"),
    verifyCommand: typeof raw.verifyCommand === "string" ? raw.verifyCommand : undefined,
    expectedResult: typeof raw.expectedResult === "string" ? raw.expectedResult : undefined,
  };
}

function normalizeResource(resource: unknown): ResourceReference {
  const value = (resource && typeof resource === "object" ? resource : {}) as Record<string, unknown>;
  return {
    ...value,
    plane: stringOr(value.plane, value.sourcePath ? "manifest" : value.providerId ? "aws" : "live"),
    kind: stringOr(value.kind, "Resource"),
    namespace: stringOr(value.namespace, ""),
    name: stringOr((value.name as string) || (value.providerName as string), "unnamed"),
  };
}

export function deriveSummary(findings: Finding[]): Summary {
  return findings.reduce(
    (summary, finding) => {
      if (finding.severity === "Blocker") summary.blockers += 1;
      else if (finding.severity === "Warning") summary.warnings += 1;
      else summary.infos += 1;
      return summary;
    },
    { blockers: 0, warnings: 0, infos: 0 },
  );
}

// resultFromSummary is the shared priority order for the overall result:
// incomplete coverage outranks blockers — an assessment built on partial
// evidence is never a fully-trusted BLOCKED or CLEAN result, even when
// real blockers were found with the evidence that WAS collected (those
// stay fully visible in Summary/Findings; only this top-level label
// defers to INCOMPLETE). Mirrors Go's Report.resultAndExitCode() exactly
// (internal/findings/report.go) so the two can't disagree.
export function resultFromSummary(summary: Summary, incomplete = false): Result {
  if (incomplete) return "INCOMPLETE";
  if (summary.blockers > 0) return "BLOCKED";
  if (summary.warnings > 0) return "PASSED_WITH_WARNINGS";
  return "CLEAN";
}

// Display-only derivations below — pure functions over an already-parsed
// Report/Finding, no effect on validation, scoring, or what the CLI itself
// computes. GO/REVIEW/NO-GO is a Console-only presentation label; the
// authoritative machine-readable value is still Result (CLEAN/
// PASSED_WITH_WARNINGS/BLOCKED), unchanged.

export type Decision = "GO" | "REVIEW" | "NO-GO";

export function decisionFromResult(result: Result): Decision {
  if (result === "BLOCKED" || result === "INCOMPLETE") return "NO-GO";
  if (result === "PASSED_WITH_WARNINGS") return "REVIEW";
  return "GO";
}

export function decisionSummaryLine(summary: Summary, incomplete = false): string {
  if (incomplete) {
    if (summary.blockers > 0) {
      return `Assessment incomplete — ${summary.blockers} blocker${summary.blockers === 1 ? "" : "s"} observed with available evidence. Resolve coverage errors and rerun.`;
    }
    return "Assessment incomplete — evidence collection was incomplete. Resolve coverage errors and rerun.";
  }
  if (summary.blockers > 0) {
    return `${summary.blockers} blocker${summary.blockers === 1 ? "" : "s"} found — fix required before the change window.`;
  }
  if (summary.warnings > 0) {
    return `${summary.warnings} warning${summary.warnings === 1 ? "" : "s"} found — review before the change window.`;
  }
  return "No blockers or warnings — safe to proceed.";
}

const SEVERITY_RANK: Record<Severity, number> = { Blocker: 0, Warning: 1, Info: 2 };

// topRisks: the highest-severity findings first (ties broken by rule ID),
// truncated to `limit` — used for the Console's Top Risks strip and
// report.html's executive summary. Not a scoring model, just "worst
// findings first," matching the same severity-then-rule-ID order every
// other renderer (terminal/Markdown/HTML) already sorts by.
export function topRisks(findings: Finding[], limit = 3): Finding[] {
  return [...findings].sort((a, b) => Number(!!b.globalBlocker) - Number(!!a.globalBlocker) || SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] || a.ruleId.localeCompare(b.ruleId)).slice(0, limit);
}

export function firstSentence(value: string): string {
  const firstLine = value.split("\n").find((line) => line.trim()) || value;
  return firstLine.length > 240 ? `${firstLine.slice(0, 237)}…` : firstLine;
}

export function resourceLabel(resource: ResourceReference): string {
  const prefix = resource.namespace ? `${resource.namespace}/` : "";
  return `${resource.kind}/${prefix}${resource.name}`;
}

export function findingResourceLabel(finding: Finding): string {
  const labels = [...new Set(finding.resources.map(resourceLabel))];
  return labels.join(", ");
}

export interface FindingFilters {
  search?: string;
  // Active severity chips. undefined = no severity filter applied (every
  // severity shown); an explicit array (including []) filters strictly by
  // membership, so deselecting every chip shows zero findings — same
  // semantics as report.html's severity checkboxes.
  severities?: Severity[];
  confidence?: string;
  namespace?: string;
}

export function filterFindings(findings: Finding[], filters: FindingFilters): Finding[] {
  const query = (filters.search || "").trim().toLowerCase();
  return findings.filter((finding) => {
    const namespaces = finding.resources.map((resource) => resource.namespace || "cluster-scoped");
    const haystack = [finding.ruleId, finding.message, findingResourceLabel(finding), ...namespaces].join(" ").toLowerCase();
    return (
      (!query || haystack.includes(query)) &&
      (!filters.severities || filters.severities.includes(finding.severity)) &&
      (!filters.confidence || finding.confidence === filters.confidence) &&
      (!filters.namespace || namespaces.includes(filters.namespace))
    );
  });
}

export function uniqueValues(findings: Finding[], selector: (finding: Finding) => string[]): string[] {
  return [...new Set(findings.flatMap(selector).filter(Boolean))].sort();
}

function stringOr(value: unknown, fallback: string): string {
  return typeof value === "string" && value.trim() ? value : fallback;
}

function normalizeCoverage(value: unknown): ScanCoverage {
  const raw = value && typeof value === "object" ? value as Record<string, unknown> : {};
  return {
    kubernetes: normalizePlaneCoverage(raw.kubernetes, "complete"),
    aws: normalizePlaneCoverage(raw.aws, "skipped"),
    manifests: normalizePlaneCoverage(raw.manifests, "skipped"),
  };
}

function normalizePlaneCoverage(value: unknown, fallback: PlaneCoverage["status"]): PlaneCoverage {
  const raw = value && typeof value === "object" ? value as Record<string, unknown> : {};
  const status = raw.status === "complete" || raw.status === "partial" || raw.status === "skipped" ? raw.status : fallback;
  return { status, errors: Array.isArray(raw.errors) ? raw.errors.map(String) : [] };
}
