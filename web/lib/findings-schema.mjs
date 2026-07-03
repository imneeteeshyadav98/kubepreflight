const SEVERITIES = ["Blocker", "Warning", "Info"];

export function parseFindingsDocument(input) {
  let value = input;
  if (typeof input === "string") {
    try { value = JSON.parse(input); } catch (error) {
      throw new Error(`Invalid JSON: ${error.message}`);
    }
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("The file must contain a JSON object.");
  if (!Array.isArray(value.findings)) throw new Error("Missing required findings[] array.");

  const findings = value.findings.map((finding, index) => normalizeFinding(finding, index));
  const summary = deriveSummary(findings);
  return {
    ...value,
    targetVersion: stringOr(value.targetVersion, "Unknown"),
    clusterContext: stringOr(value.clusterContext, "Unspecified cluster"),
    provider: stringOr(value.provider, "cluster-only"),
    scannedAt: stringOr(value.scannedAt, ""),
    assumptions: Array.isArray(value.assumptions) ? value.assumptions.map(String) : [],
    namespaceAllowlist: Array.isArray(value.namespaceAllowlist) ? value.namespaceAllowlist.map(String) : [],
    findings,
    summary,
    result: resultFromSummary(summary),
  };
}

function normalizeFinding(finding, index) {
  if (!finding || typeof finding !== "object") throw new Error(`findings[${index}] must be an object.`);
  const severity = stringOr(finding.severity, "Info");
  if (!SEVERITIES.includes(severity)) throw new Error(`findings[${index}].severity is not Blocker, Warning, or Info.`);
  const resources = Array.isArray(finding.resources)
    ? finding.resources.map((resource) => normalizeResource(resource))
    : finding.resource ? [normalizeResource(finding.resource)] : [];
  if (!resources.length) throw new Error(`findings[${index}] has no resources[].`);
  return {
    ...finding,
    ruleId: stringOr(finding.ruleId, `UNKNOWN-${index + 1}`),
    severity,
    confidence: stringOr(finding.confidence, "UNSPECIFIED"),
    message: stringOr(finding.message, "No finding message supplied."),
    evidence: Array.isArray(finding.evidence) ? finding.evidence.map(String) : [],
    remediation: stringOr(finding.remediation, "No remediation supplied."),
    fingerprint: stringOr(finding.fingerprint, "unavailable"),
    resources,
  };
}

function normalizeResource(resource) {
  const value = resource && typeof resource === "object" ? resource : {};
  return {
    ...value,
    plane: stringOr(value.plane, value.sourcePath ? "manifest" : value.providerId ? "aws" : "live"),
    kind: stringOr(value.kind, "Resource"),
    namespace: stringOr(value.namespace, ""),
    name: stringOr(value.name || value.providerName, "unnamed"),
  };
}

export function deriveSummary(findings) {
  return findings.reduce((summary, finding) => {
    if (finding.severity === "Blocker") summary.blockers += 1;
    else if (finding.severity === "Warning") summary.warnings += 1;
    else summary.infos += 1;
    return summary;
  }, { blockers: 0, warnings: 0, infos: 0 });
}

export function resultFromSummary(summary) {
  if (summary.blockers > 0) return "BLOCKED";
  if (summary.warnings > 0) return "PASSED_WITH_WARNINGS";
  return "CLEAN";
}

export function resourceLabel(resource) {
  const prefix = resource.namespace ? `${resource.namespace}/` : "";
  return `${resource.kind}/${prefix}${resource.name}`;
}

export function findingResourceLabel(finding) {
  const labels = [...new Set(finding.resources.map(resourceLabel))];
  return labels.join(", ");
}

export function filterFindings(findings, filters) {
  const query = (filters.search || "").trim().toLowerCase();
  return findings.filter((finding) => {
    const namespaces = finding.resources.map((resource) => resource.namespace || "cluster-scoped");
    const haystack = [finding.ruleId, finding.message, findingResourceLabel(finding), ...namespaces].join(" ").toLowerCase();
    return (!query || haystack.includes(query)) &&
      (!filters.severity || finding.severity === filters.severity) &&
      (!filters.confidence || finding.confidence === filters.confidence) &&
      (!filters.namespace || namespaces.includes(filters.namespace));
  });
}

export function uniqueValues(findings, selector) {
  return [...new Set(findings.flatMap(selector).filter(Boolean))].sort();
}

function stringOr(value, fallback) {
  return typeof value === "string" && value.trim() ? value : fallback;
}
