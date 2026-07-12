// Client-side port of internal/comparison (Go) -- diffs two already-parsed
// Report documents by Finding.fingerprint, which parseFindingsDocument
// (findings-schema.ts) already carries on every Finding. No hashing is
// reimplemented here: fingerprints are computed server-side by the CLI and
// simply matched as opaque strings, the same identity internal/comparison/
// compare.go itself matches on.
import type { Finding, Report } from "./findings-schema";

export const COMPARISON_SCHEMA_VERSION = "kubepreflight.io/scan-comparison/v1";

export interface FieldChange {
  before: string;
  after: string;
}

export interface ChangedFinding {
  fingerprint: string;
  ruleId: string;
  resources: Finding["resources"];
  changes: Record<string, FieldChange>;
}

export interface ComparisonSummary {
  baselineVerdict: string;
  currentVerdict: string;
  verdictChanged: boolean;
  baselineReadinessScore: number;
  currentReadinessScore: number;
  readinessScoreDelta: number;
  new: number;
  resolved: number;
  changed: number;
  unchanged: number;
  newBlockers: number;
  resolvedBlockers: number;
}

export interface Comparison {
  schemaVersion: string;
  warnings: string[];
  summary: ComparisonSummary;
  new: Finding[];
  resolved: Finding[];
  changed: ChangedFinding[];
  unchanged: Finding[];
}

// compareReports mirrors internal/comparison.Compare exactly: matching by
// fingerprint only (never message/remediation text or array position),
// tracking the same field set (severity, priority, confidence,
// canUpgradeContinue, affectedScope, ruleId, resource identity), and the
// same target-version-mismatch warning -- a baseline and current scan at
// different --target-version values will make every genuinely-unchanged
// finding look like a resolved+new pair, since fingerprints are scoped to
// target version (see FingerprintV2, Go).
export function compareReports(baseline: Report, current: Report): Comparison {
  const baselineByFP = indexByFingerprint(baseline.findings);
  const currentByFP = indexByFingerprint(current.findings);

  const warnings: string[] = [];
  if (baseline.targetVersion !== current.targetVersion) {
    warnings.push(
      `baseline was scanned at target-version "${baseline.targetVersion}" and current at "${current.targetVersion}" -- fingerprints are scoped to target version, so genuinely unchanged findings will show up as a new+resolved pair instead of unchanged. Re-scan both at the same target version for an accurate diff.`,
    );
  }

  const newFindings: Finding[] = [];
  const changed: ChangedFinding[] = [];
  const unchanged: Finding[] = [];

  for (const [fingerprint, cf] of currentByFP) {
    const bf = baselineByFP.get(fingerprint);
    if (!bf) {
      newFindings.push(cf);
      continue;
    }
    const changes = diffFinding(bf, cf);
    if (Object.keys(changes).length > 0) {
      changed.push({ fingerprint, ruleId: cf.ruleId, resources: cf.resources, changes });
    } else {
      unchanged.push(cf);
    }
  }

  const resolved: Finding[] = [];
  for (const [fingerprint, bf] of baselineByFP) {
    if (!currentByFP.has(fingerprint)) resolved.push(bf);
  }

  sortBlockerFirst(newFindings);
  sortBlockerFirst(resolved);
  unchanged.sort((a, b) => compareEntry(entryKey(a), entryKey(b)));
  changed.sort((a, b) => compareEntry(changedKey(a), changedKey(b)));

  const summary = buildSummary(baseline, current, newFindings, resolved, changed, unchanged);

  return { schemaVersion: COMPARISON_SCHEMA_VERSION, warnings, summary, new: newFindings, resolved, changed, unchanged };
}

function indexByFingerprint(findings: Finding[]): Map<string, Finding> {
  const byFP = new Map<string, Finding>();
  for (const f of findings) {
    if (!f.fingerprint || f.fingerprint === "unavailable") {
      throw new Error(`finding ${f.ruleId} has no fingerprint -- cannot compare without stable identity`);
    }
    if (byFP.has(f.fingerprint)) {
      throw new Error(`duplicate fingerprint "${f.fingerprint}" (rule ${f.ruleId}) -- a findings.json document must not contain two findings with the same fingerprint`);
    }
    byFP.set(f.fingerprint, f);
  }
  return byFP;
}

// diffFinding compares only decision-relevant fields -- never message,
// evidence, or remediation text, so a wording change between kubepreflight
// versions is never mistaken for the underlying issue actually changing.
function diffFinding(before: Finding, after: Finding): Record<string, FieldChange> {
  const changes: Record<string, FieldChange> = {};
  if (before.severity !== after.severity) changes.severity = { before: before.severity, after: after.severity };
  if ((before.priority ?? "") !== (after.priority ?? "")) changes.priority = { before: before.priority ?? "", after: after.priority ?? "" };
  if (before.confidence !== after.confidence) changes.confidence = { before: before.confidence, after: after.confidence };
  if ((before.canUpgradeContinue ?? true) !== (after.canUpgradeContinue ?? true)) {
    changes.canUpgradeContinue = { before: String(before.canUpgradeContinue ?? true), after: String(after.canUpgradeContinue ?? true) };
  }
  if ((before.affectedScope ?? "") !== (after.affectedScope ?? "")) changes.affectedScope = { before: before.affectedScope ?? "", after: after.affectedScope ?? "" };
  // Defensive only -- the fingerprint these two findings already share
  // hashes both ruleId and each resource's identity server-side, so a real
  // difference here should be unreachable in practice.
  if (before.ruleId !== after.ruleId) changes.ruleId = { before: before.ruleId, after: after.ruleId };
  const beforeResource = resourceIdentity(before.resources);
  const afterResource = resourceIdentity(after.resources);
  if (beforeResource !== afterResource) changes.resource = { before: beforeResource, after: afterResource };
  return changes;
}

function resourceIdentity(resources: Finding["resources"]): string {
  return resources.map((r) => `${r.plane}:${r.kind}:${r.namespace}/${r.name}`).join(";");
}

function buildSummary(
  baseline: Report,
  current: Report,
  newFindings: Finding[],
  resolved: Finding[],
  changed: ChangedFinding[],
  unchanged: Finding[],
): ComparisonSummary {
  const baselineVerdict = baseline.result;
  const currentVerdict = current.result;
  const baselineReadinessScore = baseline.upgradeReadiness?.readinessScore ?? 0;
  const currentReadinessScore = current.upgradeReadiness?.readinessScore ?? 0;
  return {
    baselineVerdict,
    currentVerdict,
    verdictChanged: baselineVerdict !== currentVerdict,
    baselineReadinessScore,
    currentReadinessScore,
    readinessScoreDelta: currentReadinessScore - baselineReadinessScore,
    new: newFindings.length,
    resolved: resolved.length,
    changed: changed.length,
    unchanged: unchanged.length,
    newBlockers: newFindings.filter((f) => f.severity === "Blocker").length,
    resolvedBlockers: resolved.filter((f) => f.severity === "Blocker").length,
  };
}

// --- Deterministic ordering, mirroring internal/comparison/sort.go ---

const SEVERITY_RANK: Record<string, number> = { Blocker: 0, Warning: 1, Info: 2 };

interface SortKey {
  priority: string;
  severityRank: number;
  ruleId: string;
  namespace: string;
  name: string;
  fingerprint: string;
}

function priorityForSort(priority: string): string {
  return priority === "" ? "P9" : priority;
}

function firstResourceIdentity(resources: Finding["resources"]): { namespace: string; name: string } {
  if (resources.length === 0) return { namespace: "", name: "" };
  return { namespace: resources[0].namespace, name: resources[0].name };
}

function entryKey(f: Finding): SortKey {
  const { namespace, name } = firstResourceIdentity(f.resources);
  return {
    priority: priorityForSort(f.priority ?? ""),
    severityRank: SEVERITY_RANK[f.severity] ?? 2,
    ruleId: f.ruleId,
    namespace,
    name,
    fingerprint: f.fingerprint,
  };
}

function changedKey(c: ChangedFinding): SortKey {
  const { namespace, name } = firstResourceIdentity(c.resources);
  return {
    priority: priorityForSort(c.changes.priority?.after ?? ""),
    severityRank: 0,
    ruleId: c.ruleId,
    namespace,
    name,
    fingerprint: c.fingerprint,
  };
}

function compareEntry(a: SortKey, b: SortKey): number {
  return (
    a.priority.localeCompare(b.priority) ||
    a.severityRank - b.severityRank ||
    a.ruleId.localeCompare(b.ruleId) ||
    a.namespace.localeCompare(b.namespace) ||
    a.name.localeCompare(b.name) ||
    a.fingerprint.localeCompare(b.fingerprint)
  );
}

function sortBlockerFirst(findings: Finding[]): void {
  findings.sort((a, b) => {
    const aBlocker = a.severity === "Blocker";
    const bBlocker = b.severity === "Blocker";
    if (aBlocker !== bBlocker) return aBlocker ? -1 : 1;
    return compareEntry(entryKey(a), entryKey(b));
  });
}
