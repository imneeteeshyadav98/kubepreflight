import { findingResourceLabel, type Finding, type ResourceReference, type Severity } from "./findings-schema";

export interface ActionGroupModel {
  findings: Finding[];
  primary: Finding;
  resourceLabel: string;
  ruleIds: string[];
  severity: Severity;
}

const rank: Record<Severity, number> = { Blocker: 0, Warning: 1, Info: 2 };

// resourceKey mirrors Go's ResourceReference.ConceptKey()/OccurrenceKey()
// (internal/findings/finding.go): prefer the conceptual Kind+Namespace+Name
// identity when it's safe to correlate (a namespaced resource needs a real
// namespace — an omitted one can't be safely correlated, matching Go's
// ConceptKey rule exactly), and fall back to a plane-specific occurrence
// key otherwise. The occurrence fallback always includes kind/namespace/
// name alongside sourcePath/uid — a bare sourcePath (or uid, or name)
// alone would let two *different* resources declared in the same manifest
// file (or otherwise sharing that one field) collapse onto the same key
// and incorrectly merge into one Next Action group.
function resourceKey(resource: ResourceReference): string {
  const hasSafeConceptIdentity = Boolean(resource.kind) && Boolean(resource.name) && !(resource.scope === "namespaced" && !resource.namespace);
  if ((resource.plane === "live" || resource.plane === "manifest") && hasSafeConceptIdentity) {
    return ["k8s", resource.kind, resource.namespace, resource.name].join("|");
  }
  if (resource.plane === "live") {
    return ["occurrence", "live", resource.uid, resource.kind, resource.namespace, resource.name].join("|");
  }
  if (resource.plane === "manifest") {
    return ["occurrence", "manifest", resource.sourcePath, resource.kind, resource.namespace, resource.name].join("|");
  }
  if (resource.plane === "aws") {
    return ["occurrence", "aws", resource.providerId, resource.kind].join("|");
  }
  return ["occurrence", resource.plane, resource.kind, resource.namespace, resource.name].join("|");
}

function resourceKeys(finding: Finding): Set<string> {
  return new Set(finding.resources.map(resourceKey));
}

function intersects(a: Set<string>, b: Set<string>): boolean {
  return [...a].some((key) => b.has(key));
}

export function buildActionGroups(findings: Finding[]): ActionGroupModel[] {
  const groups: { findings: Finding[]; keys: Set<string> }[] = [];
  findings.filter((finding) => finding.remediation).forEach((finding) => {
    const keys = resourceKeys(finding);
    const matching = groups.filter((group) => intersects(group.keys, keys));
    if (matching.length === 0) {
      groups.push({ findings: [finding], keys });
      return;
    }
    const target = matching[0];
    target.findings.push(finding);
    keys.forEach((key) => target.keys.add(key));
    matching.slice(1).forEach((group) => {
      target.findings.push(...group.findings);
      group.keys.forEach((key) => target.keys.add(key));
      groups.splice(groups.indexOf(group), 1);
    });
  });

  return groups.map((group) => {
    const ordered = [...group.findings].sort((a, b) => rank[a.severity] - rank[b.severity] || a.ruleId.localeCompare(b.ruleId));
    const primary = ordered[0];
    return {
      findings: ordered,
      primary,
      resourceLabel: findingResourceLabel(primary),
      ruleIds: [...new Set(ordered.map((finding) => finding.ruleId))].sort(),
      severity: primary.severity,
    };
  }).sort((a, b) => Number(!a.findings.some((finding) => finding.globalBlocker)) - Number(!b.findings.some((finding) => finding.globalBlocker)) || rank[a.severity] - rank[b.severity] || a.resourceLabel.localeCompare(b.resourceLabel));
}
