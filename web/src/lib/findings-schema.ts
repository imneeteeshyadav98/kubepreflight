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
  [key: string]: unknown;
}

export interface Summary {
  blockers: number;
  warnings: number;
  infos: number;
}

export type Result = "CLEAN" | "PASSED_WITH_WARNINGS" | "BLOCKED";

export interface Report {
  targetVersion: string;
  clusterContext: string;
  provider: string;
  scannedAt: string;
  assumptions: string[];
  namespaceAllowlist: string[];
  findings: Finding[];
  summary: Summary;
  result: Result;
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
  return {
    ...raw,
    targetVersion: stringOr(raw.targetVersion, "Unknown"),
    clusterContext: stringOr(raw.clusterContext, "Unspecified cluster"),
    provider: stringOr(raw.provider, "cluster-only"),
    scannedAt: stringOr(raw.scannedAt, ""),
    assumptions: Array.isArray(raw.assumptions) ? raw.assumptions.map(String) : [],
    namespaceAllowlist: Array.isArray(raw.namespaceAllowlist) ? raw.namespaceAllowlist.map(String) : [],
    findings,
    summary,
    result: resultFromSummary(summary),
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

export function resultFromSummary(summary: Summary): Result {
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
  if (result === "BLOCKED") return "NO-GO";
  if (result === "PASSED_WITH_WARNINGS") return "REVIEW";
  return "GO";
}

export function decisionSummaryLine(summary: Summary): string {
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
  return [...findings].sort((a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] || a.ruleId.localeCompare(b.ruleId)).slice(0, limit);
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
