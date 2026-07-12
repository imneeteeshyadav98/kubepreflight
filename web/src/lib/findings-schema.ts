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
  // priority/priorityReason/affectedScope/canUpgradeContinue mirror
  // findings.Finding's Priority/PriorityReason/AffectedScope/
  // CanUpgradeContinue (Go, see internal/findings/priority.go) — set on
  // every finding server-side once parsed via parseFindingsDocument
  // (normalizeFinding defaults priority to "" and canUpgradeContinue to
  // true for a pre-priority legacy findings.json). Optional here (like
  // globalBlocker above) only so hand-built Finding fixtures in tests
  // don't all need updating — real parsed data always has them.
  priority?: string;
  priorityReason?: string;
  affectedScope?: string;
  canUpgradeContinue?: boolean;
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

// EKSClusterInfo mirrors findings.EKSClusterInfo (Go). Absent (undefined)
// for every non-EKS scan and for an EKS scan where AWS enrichment was
// unavailable — its absence must never be read as an upgrade blocker, only
// as "this metadata wasn't available."
export interface EKSClusterInfo {
  clusterName?: string;
  region?: string;
  version?: string;
  platformVersion?: string;
  status?: string;
  supportType?: string;
  endpointAccess?: string;
  arn?: string;
}

// EKSAddonInfo mirrors findings.EKSAddonInfo (Go) — the full installed
// add-on inventory, not just the subset ADDON-001 flagged as incompatible
// (a compatible add-on raises no finding and would otherwise be invisible).
export interface EKSAddonInfo {
  name: string;
  currentVersion?: string;
  compatibleVersions?: string[];
  // compatible is meaningless (always false) when verificationUnavailable
  // is true — check verificationUnavailable first.
  compatible: boolean;
  verificationUnavailable?: boolean;
}

export interface EKSNodegroupHealthIssue {
  code?: string;
  message?: string;
  resourceIds?: string[];
}

export interface EKSNodegroupInfo {
  name: string;
  status?: string;
  version?: string;
  releaseVersion?: string;
  amiType?: string;
  capacityType?: string;
  desiredSize?: number;
  minSize?: number;
  maxSize?: number;
  maxUnavailable?: number;
  maxUnavailablePercentage?: number;
  launchTemplate?: boolean;
  healthIssues?: EKSNodegroupHealthIssue[];
  autoScalingGroups?: string[];
  readinessStatus: string;
  notes?: string[];
}

export interface EKSUpgradeInsightInfo {
  id: string;
  name: string;
  category: string;
  status: string;
  kubernetesVersion?: string;
  lastRefreshTime?: string;
  lastTransitionTime?: string;
  description?: string;
  recommendation?: string;
  additionalInfo?: Record<string, string>;
  deprecationDetails?: string[];
  addonCompatibilityDetails?: string[];
}

export type APICompatibilityStatus = "Passed" | "Warning" | "Failed";

export interface APICompatibilityItem {
  apiVersion: string;
  kind: string;
  count: number;
  resources?: string[];
}

export interface APICompatibilitySummary {
  status: APICompatibilityStatus;
  upgradeContinue: boolean;
  removedObjects: number;
  deprecatedObjects: number;
  removedFamilies?: APICompatibilityItem[];
  deprecatedFamilies?: APICompatibilityItem[];
  criticalImpact: boolean;
  scoreImpact: number;
}

export interface Report {
  schemaVersion: string;
  currentVersion: string;
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
  eksCluster?: EKSClusterInfo;
  eksAddons?: EKSAddonInfo[];
  eksNodegroups?: EKSNodegroupInfo[];
  eksUpgradeInsights?: EKSUpgradeInsightInfo[];
  apiCompatibility?: APICompatibilitySummary;
  upgradeReadiness?: UpgradeReadinessSummary;
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
  const apiCompatibility = normalizeAPICompatibility(raw.apiCompatibility) ?? deriveAPICompatibilitySummary(findings);
  const result = resultFromSummary(summary, Object.values(coverage).some((plane) => plane.status === "partial"));
  const upgradeReadiness = normalizeUpgradeReadiness(raw.upgradeReadiness) ?? deriveUpgradeReadinessSummary(findings, result);
  return {
    ...raw,
    schemaVersion: stringOr(raw.schemaVersion, "legacy"),
    currentVersion: normalizeKubernetesVersion(stringOr(raw.currentVersion, "")) ?? "Unknown",
    targetVersion: stringOr(raw.targetVersion, "Unknown"),
    clusterContext: stringOr(raw.clusterContext, "Unspecified cluster"),
    provider: stringOr(raw.provider, "cluster-only"),
    scannedAt: stringOr(raw.scannedAt, ""),
    assumptions: Array.isArray(raw.assumptions) ? raw.assumptions.map(String) : [],
    namespaceAllowlist: Array.isArray(raw.namespaceAllowlist) ? raw.namespaceAllowlist.map(String) : [],
    findings,
    summary,
    coverage,
    eksCluster: normalizeEKSCluster(raw.eksCluster),
    eksAddons: normalizeEKSAddons(raw.eksAddons),
    eksNodegroups: normalizeEKSNodegroups(raw.eksNodegroups),
    eksUpgradeInsights: normalizeEKSUpgradeInsights(raw.eksUpgradeInsights),
    apiCompatibility,
    upgradeReadiness,
    result,
  };
}

function normalizeAPICompatibility(value: unknown): APICompatibilitySummary | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const raw = value as Record<string, unknown>;
  const status = raw.status === "Failed" || raw.status === "Warning" || raw.status === "Passed" ? raw.status : "Passed";
  return {
    status,
    upgradeContinue: typeof raw.upgradeContinue === "boolean" ? raw.upgradeContinue : status !== "Failed",
    removedObjects: numberField(raw.removedObjects) ?? 0,
    deprecatedObjects: numberField(raw.deprecatedObjects) ?? 0,
    removedFamilies: normalizeAPICompatibilityItems(raw.removedFamilies),
    deprecatedFamilies: normalizeAPICompatibilityItems(raw.deprecatedFamilies),
    criticalImpact: raw.criticalImpact === true,
    scoreImpact: numberField(raw.scoreImpact) ?? 0,
  };
}

function normalizeAPICompatibilityItems(value: unknown): APICompatibilityItem[] | undefined {
  if (!Array.isArray(value) || value.length === 0) return undefined;
  const out: APICompatibilityItem[] = [];
  for (const entry of value) {
    if (!entry || typeof entry !== "object") continue;
    const raw = entry as Record<string, unknown>;
    const apiVersion = stringField(raw.apiVersion);
    const kind = stringField(raw.kind);
    if (!apiVersion && !kind) continue;
    out.push({
      apiVersion: apiVersion ?? "",
      kind: kind ?? "",
      count: numberField(raw.count) ?? 0,
      resources: Array.isArray(raw.resources) ? raw.resources.map(String) : undefined,
    });
  }
  return out.length > 0 ? out : undefined;
}

export function deriveAPICompatibilitySummary(findings: Finding[]): APICompatibilitySummary {
  const summary: APICompatibilitySummary = {
    status: "Passed",
    upgradeContinue: true,
    removedObjects: 0,
    deprecatedObjects: 0,
    removedFamilies: undefined,
    deprecatedFamilies: undefined,
    criticalImpact: false,
    scoreImpact: 0,
  };
  const removedFamilies = new Map<string, APICompatibilityItem>();
  const deprecatedFamilies = new Map<string, APICompatibilityItem>();

  findings.forEach((finding) => {
    if (finding.ruleId !== "API-001" && finding.ruleId !== "API-002") return;
    const family = apiCompatibilityFamily(finding);
    if (!family.apiVersion && !family.kind) return;
    if (finding.ruleId === "API-001" && finding.severity === "Blocker") {
      summary.removedObjects += 1;
      addAPICompatibilityFamily(removedFamilies, family, finding);
      summary.status = "Failed";
      summary.upgradeContinue = false;
      if (apiCompatibilityCriticalImpact(finding)) summary.criticalImpact = true;
      return;
    }
    if (finding.ruleId === "API-002" || finding.severity === "Warning") {
      summary.deprecatedObjects += 1;
      addAPICompatibilityFamily(deprecatedFamilies, family, finding);
      if (summary.status === "Passed") summary.status = "Warning";
    }
  });

  summary.removedFamilies = sortedAPICompatibilityFamilies(removedFamilies);
  summary.deprecatedFamilies = sortedAPICompatibilityFamilies(deprecatedFamilies);
  summary.scoreImpact = apiCompatibilityScoreImpact(summary);
  return summary;
}

function apiCompatibilityFamily(finding: Finding): APICompatibilityItem {
  const apiVersion = finding.evidence.find((line) => line.startsWith("apiVersion: "))?.replace("apiVersion: ", "").trim() ?? "";
  return { apiVersion, kind: finding.resources[0]?.kind ?? "", count: 0 };
}

function addAPICompatibilityFamily(families: Map<string, APICompatibilityItem>, family: APICompatibilityItem, finding: Finding): void {
  const key = `${family.apiVersion}\u0000${family.kind}`;
  const item = families.get(key) ?? { apiVersion: family.apiVersion, kind: family.kind, count: 0, resources: [] };
  item.count += 1;
  const labels = new Set(item.resources ?? []);
  finding.resources.forEach((resource) => labels.add(resourceLabel(resource)));
  item.resources = [...labels].sort();
  families.set(key, item);
}

function sortedAPICompatibilityFamilies(families: Map<string, APICompatibilityItem>): APICompatibilityItem[] | undefined {
  const out = [...families.values()].sort((a, b) => a.apiVersion.localeCompare(b.apiVersion) || a.kind.localeCompare(b.kind));
  return out.length > 0 ? out : undefined;
}

function apiCompatibilityCriticalImpact(finding: Finding): boolean {
  if (finding.globalBlocker === true || finding.criticalInfra === true) return true;
  return finding.resources.some((resource) => resource.scope === "cluster");
}

function apiCompatibilityScoreImpact(summary: APICompatibilitySummary): number {
  const removedFamilyCount = summary.removedFamilies?.length ?? 0;
  const deprecatedFamilyCount = summary.deprecatedFamilies?.length ?? 0;
  let impact = 0;
  if (removedFamilyCount > 0) impact -= 25 + Math.max(0, removedFamilyCount - 1) * 10;
  if (summary.criticalImpact) impact -= 15;
  impact -= deprecatedFamilyCount * 5;
  return Math.max(impact, -60);
}

export type UpgradeReadinessCategoryStatus = "Passed" | "Warning" | "Failed";

export interface UpgradeReadinessCategory {
  name: string;
  status: UpgradeReadinessCategoryStatus;
  blockerCount: number;
  warningCount: number;
  ruleIds: string[];
}

export interface UpgradeReadinessSummary {
  verdict: string;
  upgradeContinue: boolean;
  readinessScore: number;
  categories: UpgradeReadinessCategory[];
}

// categoryOrder/categoryByRuleID mirror internal/findings/report.go exactly
// (categoryOrder, categoryByRuleID) — there's no way to share Go code with
// TS, so this is a deliberate, tested duplication, same situation
// deriveAPICompatibilitySummary is already in.
const upgradeReadinessCategoryOrder = [
  "API Compatibility",
  "Extension APIs",
  "Admission Webhooks",
  "Disruption Safety",
  "Drain Readiness",
  "Node Readiness",
  "Add-ons",
  "CoreDNS",
  "Workload Health",
  "EKS Upgrade Insights",
];

const upgradeReadinessCategoryByRuleId: Record<string, string> = {
  "API-001": "API Compatibility",
  "API-002": "API Compatibility",
  "CRD-001": "Extension APIs",
  "CRD-002": "Extension APIs",
  "APISERVICE-001": "Extension APIs",
  "WH-001": "Admission Webhooks",
  "WH-002": "Admission Webhooks",
  "WH-004": "Admission Webhooks",
  "WH-005": "Admission Webhooks",
  "PDB-001": "Disruption Safety",
  "PDB-002": "Disruption Safety",
  "DRAIN-001": "Drain Readiness",
  "DRAIN-002": "Drain Readiness",
  "DRAIN-003": "Drain Readiness",
  "DRAIN-004": "Drain Readiness",
  "NODE-001": "Node Readiness",
  "NODE-002": "Node Readiness",
  "NODE-003": "Node Readiness",
  "NET-002": "Node Readiness",
  "EKS-NG-001": "Node Readiness",
  "EKS-NG-002": "Node Readiness",
  "EKS-NG-003": "Node Readiness",
  "EKS-NG-004": "Node Readiness",
  "ADDON-001": "Add-ons",
  "ADDON-002": "Add-ons",
  "COREDNS-001": "CoreDNS",
  "WORKLOAD-001": "Workload Health",
  "EKS-INSIGHT-001": "EKS Upgrade Insights",
  "EKS-INSIGHT-002": "EKS Upgrade Insights",
  "EKS-INSIGHT-003": "EKS Upgrade Insights",
};

function normalizeUpgradeReadiness(value: unknown): UpgradeReadinessSummary | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const raw = value as Record<string, unknown>;
  if (!Array.isArray(raw.categories)) return undefined;
  const categories: UpgradeReadinessCategory[] = [];
  for (const entry of raw.categories) {
    if (!entry || typeof entry !== "object") continue;
    const rawCat = entry as Record<string, unknown>;
    const status = rawCat.status === "Failed" || rawCat.status === "Warning" || rawCat.status === "Passed" ? rawCat.status : "Passed";
    categories.push({
      name: stringField(rawCat.name) ?? "",
      status,
      blockerCount: numberField(rawCat.blockerCount) ?? 0,
      warningCount: numberField(rawCat.warningCount) ?? 0,
      ruleIds: Array.isArray(rawCat.ruleIds) ? rawCat.ruleIds.map(String) : [],
    });
  }
  return {
    verdict: stringField(raw.verdict) ?? "CLEAN",
    upgradeContinue: typeof raw.upgradeContinue === "boolean" ? raw.upgradeContinue : true,
    readinessScore: numberField(raw.readinessScore) ?? 100,
    categories,
  };
}

// deriveUpgradeReadinessSummary is the client-side fallback for a
// findings.json without a precomputed upgradeReadiness field (e.g. a
// hand-built demo document) — mirrors
// internal/findings/report.go's BuildUpgradeReadinessSummary exactly,
// including the same score formula.
export function deriveUpgradeReadinessSummary(findings: Finding[], verdict: string): UpgradeReadinessSummary {
  const byCategory = new Map<string, UpgradeReadinessCategory>();
  upgradeReadinessCategoryOrder.forEach((name) => byCategory.set(name, { name, status: "Passed", blockerCount: 0, warningCount: 0, ruleIds: [] }));

  findings.forEach((finding) => {
    const name = upgradeReadinessCategoryByRuleId[finding.ruleId];
    if (!name) return;
    const cat = byCategory.get(name)!;
    if (!cat.ruleIds.includes(finding.ruleId)) cat.ruleIds.push(finding.ruleId);
    if (finding.severity === "Blocker") {
      cat.blockerCount += 1;
      cat.status = "Failed";
    } else if (finding.severity === "Warning") {
      cat.warningCount += 1;
      if (cat.status !== "Failed") cat.status = "Warning";
    }
  });

  let score = 100;
  let anyBlocker = false;
  const categories = upgradeReadinessCategoryOrder.map((name) => {
    const cat = byCategory.get(name)!;
    cat.ruleIds.sort();
    if (cat.blockerCount > 0) anyBlocker = true;
    score -= upgradeReadinessCategoryPenalty(cat);
    return cat;
  });

  return {
    verdict,
    upgradeContinue: !anyBlocker,
    readinessScore: Math.max(0, score),
    categories,
  };
}

function upgradeReadinessCategoryPenalty(cat: UpgradeReadinessCategory): number {
  if (cat.status === "Failed") return Math.min(25, 15 + 3 * (cat.blockerCount - 1));
  if (cat.status === "Warning") return Math.min(10, 5 + (cat.warningCount - 1));
  return 0;
}

function normalizeEKSAddons(value: unknown): EKSAddonInfo[] | undefined {
  if (!Array.isArray(value) || value.length === 0) return undefined;
  const out: EKSAddonInfo[] = [];
  for (const entry of value) {
    if (!entry || typeof entry !== "object") continue;
    const raw = entry as Record<string, unknown>;
    if (typeof raw.name !== "string" || !raw.name) continue;
    out.push({
      name: raw.name,
      currentVersion: typeof raw.currentVersion === "string" ? raw.currentVersion : undefined,
      compatibleVersions: Array.isArray(raw.compatibleVersions) ? raw.compatibleVersions.map(String) : undefined,
      compatible: raw.compatible === true,
      verificationUnavailable: raw.verificationUnavailable === true,
    });
  }
  return out.length > 0 ? out : undefined;
}

function normalizeEKSNodegroups(value: unknown): EKSNodegroupInfo[] | undefined {
  if (!Array.isArray(value)) return undefined;
  if (value.length === 0) return [];
  const out: EKSNodegroupInfo[] = [];
  for (const entry of value) {
    if (!entry || typeof entry !== "object") continue;
    const raw = entry as Record<string, unknown>;
    if (typeof raw.name !== "string" || !raw.name) continue;
    out.push({
      name: raw.name,
      status: stringField(raw.status),
      version: stringField(raw.version),
      releaseVersion: stringField(raw.releaseVersion),
      amiType: stringField(raw.amiType),
      capacityType: stringField(raw.capacityType),
      desiredSize: numberField(raw.desiredSize),
      minSize: numberField(raw.minSize),
      maxSize: numberField(raw.maxSize),
      maxUnavailable: numberField(raw.maxUnavailable),
      maxUnavailablePercentage: numberField(raw.maxUnavailablePercentage),
      launchTemplate: raw.launchTemplate === true,
      healthIssues: normalizeEKSNodegroupHealthIssues(raw.healthIssues),
      autoScalingGroups: Array.isArray(raw.autoScalingGroups) ? raw.autoScalingGroups.map(String) : undefined,
      readinessStatus: stringOr(raw.readinessStatus, "Ready with review"),
      notes: Array.isArray(raw.notes) ? raw.notes.map(String) : undefined,
    });
  }
  return out;
}

function normalizeEKSNodegroupHealthIssues(value: unknown): EKSNodegroupHealthIssue[] | undefined {
  if (!Array.isArray(value) || value.length === 0) return undefined;
  const out: EKSNodegroupHealthIssue[] = [];
  for (const entry of value) {
    if (!entry || typeof entry !== "object") continue;
    const raw = entry as Record<string, unknown>;
    out.push({
      code: stringField(raw.code),
      message: stringField(raw.message),
      resourceIds: Array.isArray(raw.resourceIds) ? raw.resourceIds.map(String) : undefined,
    });
  }
  return out.length > 0 ? out : undefined;
}

function normalizeEKSUpgradeInsights(value: unknown): EKSUpgradeInsightInfo[] | undefined {
  if (!Array.isArray(value)) return undefined;
  if (value.length === 0) return [];
  const out: EKSUpgradeInsightInfo[] = [];
  for (const entry of value) {
    if (!entry || typeof entry !== "object") continue;
    const raw = entry as Record<string, unknown>;
    if (typeof raw.id !== "string" || !raw.id || typeof raw.name !== "string" || !raw.name) continue;
    out.push({
      id: raw.id,
      name: raw.name,
      category: stringOr(raw.category, "UPGRADE_READINESS"),
      status: stringOr(raw.status, "UNKNOWN"),
      kubernetesVersion: stringField(raw.kubernetesVersion),
      lastRefreshTime: stringField(raw.lastRefreshTime),
      lastTransitionTime: stringField(raw.lastTransitionTime),
      description: stringField(raw.description),
      recommendation: stringField(raw.recommendation),
      additionalInfo: normalizeStringMap(raw.additionalInfo),
      deprecationDetails: Array.isArray(raw.deprecationDetails) ? raw.deprecationDetails.map(String) : undefined,
      addonCompatibilityDetails: Array.isArray(raw.addonCompatibilityDetails) ? raw.addonCompatibilityDetails.map(String) : undefined,
    });
  }
  return out;
}

function normalizeStringMap(value: unknown): Record<string, string> | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const out: Record<string, string> = {};
  for (const [key, rawValue] of Object.entries(value as Record<string, unknown>)) {
    if (typeof rawValue === "string") out[key] = rawValue;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

function stringField(value: unknown): string | undefined {
  return typeof value === "string" && value ? value : undefined;
}

function numberField(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

// eksSupportTypeLabel/eksEndpointAccessLabel mirror internal/report/html.go's
// eksSupportTypeLabel/eksEndpointAccessLabel — same friendly-label mapping,
// kept in sync by hand since report.html and the Console independently
// render the same findings.json field.
export function eksSupportTypeLabel(supportType?: string): string {
  switch (supportType) {
    case "EXTENDED":
      return "Extended support";
    case "STANDARD":
      return "Standard support";
    default:
      return "";
  }
}

export function eksEndpointAccessLabel(access?: string): string {
  switch (access) {
    case "public":
      return "Public";
    case "private":
      return "Private";
    case "public_and_private":
      return "Public + private";
    default:
      return "";
  }
}

// eksAddonStatus mirrors internal/report/html.go's eksAddonStatus — same
// three-state classification (compatible / needs update / verification
// unavailable), kept in sync by hand.
export function eksAddonStatus(addon: EKSAddonInfo): { label: string; className: "clean" | "warn" | "blocked" } {
  if (addon.verificationUnavailable) return { label: "Verification unavailable", className: "warn" };
  if (addon.compatible) return { label: "Compatible", className: "clean" };
  return { label: "Needs update", className: "blocked" };
}

export function eksNodegroupHealthLabel(nodegroup: EKSNodegroupInfo): string {
  if (!nodegroup.healthIssues || nodegroup.healthIssues.length === 0) return "Healthy";
  const codes = nodegroup.healthIssues.map((issue) => issue.code).filter(Boolean);
  return codes.length > 0 ? codes.join(", ") : `${nodegroup.healthIssues.length} issue(s)`;
}

export function eksNodegroupReadinessClass(nodegroup: EKSNodegroupInfo): "clean" | "warn" {
  return nodegroup.readinessStatus.toLowerCase().includes("required") ? "warn" : "clean";
}

export function eksUpgradeInsightStatusClass(insight: EKSUpgradeInsightInfo): "clean" | "warn" | "info" {
  switch (insight.status.toUpperCase()) {
    case "ERROR":
    case "WARNING":
      return "warn";
    case "UNKNOWN":
      return "info";
    default:
      return "clean";
  }
}

export function eksUpgradeInsightDetails(insight: EKSUpgradeInsightInfo): string {
  const parts = [
    ...(insight.deprecationDetails ?? []),
    ...(insight.addonCompatibilityDetails ?? []),
    ...Object.entries(insight.additionalInfo ?? {}).map(([key, value]) => `${key}: ${value}`),
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(" | ") : insight.description || "—";
}

function normalizeEKSCluster(value: unknown): EKSClusterInfo | undefined {
  if (!value || typeof value !== "object") return undefined;
  const raw = value as Record<string, unknown>;
  const info: EKSClusterInfo = {};
  if (typeof raw.clusterName === "string") info.clusterName = raw.clusterName;
  if (typeof raw.region === "string") info.region = raw.region;
  if (typeof raw.version === "string") info.version = raw.version;
  if (typeof raw.platformVersion === "string") info.platformVersion = raw.platformVersion;
  if (typeof raw.status === "string") info.status = raw.status;
  if (typeof raw.supportType === "string") info.supportType = raw.supportType;
  if (typeof raw.endpointAccess === "string") info.endpointAccess = raw.endpointAccess;
  if (typeof raw.arn === "string") info.arn = raw.arn;
  return Object.keys(info).length > 0 ? info : undefined;
}

export interface UpgradeContext {
  current: string;
  target: string;
  path: string;
  versions?: string[];
  label: string;
  line: string;
  note?: string;
}

export interface UpgradeDetailHop {
  from: string;
  to: string;
  statusLabel: string;
  statusClass: "blocked" | "warning" | "current-live" | "rescan-required";
  assessment: string;
  checks: string[];
  findingLines: string[];
}

export function normalizeKubernetesVersion(value: string): string | null {
  const match = value.trim().replace(/^v/, "").match(/^(\d+)\.(\d+)/);
  return match ? `${Number(match[1])}.${Number(match[2])}` : null;
}

export function upgradeContext(report: Pick<Report, "currentVersion" | "targetVersion">): UpgradeContext {
  const target = report.targetVersion || "Unknown";
  const current = normalizeKubernetesVersion(report.currentVersion) ?? "Unknown";
  const targetVersion = normalizeKubernetesVersion(target);
  if (current !== "Unknown" && targetVersion) {
    const [currentMajor, currentMinor] = current.split(".").map(Number);
    const [targetMajor, targetMinor] = targetVersion.split(".").map(Number);
    if (currentMajor === targetMajor && targetMinor >= currentMinor) {
      const versions = Array.from({ length: targetMinor - currentMinor + 1 }, (_, index) => `${currentMajor}.${currentMinor + index}`);
      const gap = targetMinor - currentMinor;
      return {
        current,
        target,
        versions,
        path: versions.join(" \u2192 "),
        label: gap === 0 ? "same-minor target" : gap === 1 ? "one-minor upgrade" : "multi-minor upgrade path",
        line: `This scan checks readiness for upgrading from ${current} to ${target}.`,
      };
    }
  }
  if (current === "Unknown") {
    return {
      current,
      target,
      path: `${current} \u2192 ${target}`,
      label: "current version unknown",
      line: `This scan checks readiness for target ${target}; current control-plane version is unknown.`,
      note: "Current control-plane version was not available from the Kubernetes server version API. Node/kubelet versions are evaluated separately.",
    };
  }
  return {
    current,
    target,
    path: `${current} \u2192 ${target}`,
    label: "upgrade path unavailable",
    line: `This scan checks readiness for target ${target}; upgrade path could not be derived from current version ${current}.`,
  };
}

export function upgradeDetails(report: Report): UpgradeDetailHop[] {
  const context = upgradeContext(report);
  const versions = context.versions;
  if (!versions || versions.length < 2) return [];
  const multiHop = versions.length > 2;
  return versions.slice(0, -1).map((from, index) => {
    const to = versions[index + 1];
    if (index === 0 && !multiHop) {
      const status = currentHopStatus(report.summary);
      return {
        from,
        to,
        ...status,
        checks: upgradeCheckLines(),
        findingLines: currentHopFindingLines(report.findings),
      };
    }
    if (index === 0) {
      return {
        from,
        to,
        statusLabel: "Planned, hop-specific scan recommended",
        statusClass: "rescan-required",
        assessment: `Findings were evaluated against final target ${report.targetVersion}, not this individual hop. Re-scan or run plan for a hop-specific assessment.`,
        checks: upgradeCheckLines(),
        findingLines: ["Overall target blockers remain listed in this report, but they are not proof that this intermediate hop is blocked."],
      };
    }
    return {
      from,
      to,
      statusLabel: "Planned, re-scan required",
      statusClass: "rescan-required",
      assessment: "Do not treat this future hop as safe yet. Complete the previous hop, then re-run KubePreflight against this target.",
      checks: upgradeCheckLines(),
      findingLines: [`Findings were evaluated against final target ${report.targetVersion}; current findings are not projected as proof for this future cluster state.`],
    };
  });
}

function currentHopStatus(summary: Summary): Pick<UpgradeDetailHop, "statusLabel" | "statusClass" | "assessment"> {
  if (summary.blockers > 0) {
    return {
      statusLabel: "Blocked",
      statusClass: "blocked",
      assessment: "Current findings must be resolved before this hop should proceed.",
    };
  }
  if (summary.warnings > 0) {
    return {
      statusLabel: "Needs review",
      statusClass: "warning",
      assessment: "No hard blockers were found, but warnings should be reviewed before this hop.",
    };
  }
  return {
    statusLabel: "Current assessment",
    statusClass: "current-live",
    assessment: "No blockers or warnings were found for the currently assessed hop.",
  };
}

function upgradeCheckLines(): string[] {
  return [
    "API removals and deprecated API usage",
    "Node/kubelet version skew",
    "Admission webhook availability and scope",
    "PDB and workload drain safety",
    "Add-on, CoreDNS, CNI, and storage/CSI compatibility",
    "Release notes review for the target minor",
  ];
}

function currentHopFindingLines(findings: Finding[]): string[] {
  if (!findings.length) return ["No current findings mapped to hop risk categories."];
  const counts = new Map<string, Map<Severity, number>>();
  const ruleIds = new Map<string, Set<string>>();
  findings.forEach((finding) => {
    const category = upgradeCategoryForRule(finding.ruleId);
    if (!counts.has(category)) {
      counts.set(category, new Map());
      ruleIds.set(category, new Set());
    }
    const severityCounts = counts.get(category)!;
    severityCounts.set(finding.severity, (severityCounts.get(finding.severity) ?? 0) + 1);
    ruleIds.get(category)!.add(finding.ruleId);
  });
  return [...counts.keys()].sort().map((category) => {
    const severityCounts = counts.get(category)!;
    const parts = [
      severityCounts.get("Blocker") ? `${severityCounts.get("Blocker")} blocker(s)` : "",
      severityCounts.get("Warning") ? `${severityCounts.get("Warning")} warning(s)` : "",
      severityCounts.get("Info") ? `${severityCounts.get("Info")} info` : "",
    ].filter(Boolean);
    return `${category}: ${parts.join(", ")} (${[...(ruleIds.get(category) ?? [])].sort().join(", ")})`;
  });
}

function upgradeCategoryForRule(ruleId: string): string {
  switch (ruleId) {
    case "API-001":
    case "API-002":
    case "CRD-001":
    case "EKS-INSIGHT-001":
    case "EKS-INSIGHT-002":
    case "EKS-INSIGHT-003":
      return "API removals and deprecations";
    case "NODE-001":
      return "Node/kubelet skew";
    case "NODE-003":
      return "Node scheduling compatibility";
    case "WH-001":
    case "WH-002":
      return "Admission webhooks";
    case "PDB-001":
    case "PDB-002":
      return "PDB and drain safety";
    case "WORKLOAD-001":
      return "Workload health";
    case "ADDON-001":
    case "ADDON-002":
    case "COREDNS-001":
    case "NODE-002":
      return "Add-on and platform compatibility";
    default:
      return "Other upgrade readiness checks";
  }
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
    priority: stringOr(raw.priority, ""),
    canUpgradeContinue: typeof raw.canUpgradeContinue === "boolean" ? raw.canUpgradeContinue : true,
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
const PRIORITY_RANK: Record<string, number> = { P1: 0, P2: 1, P3: 2, P4: 3 };
const CONFIDENCE_RANK: Record<string, number> = { STATIC_CERTAIN: 0, OBSERVED: 1, PROVIDER_REPORTED: 2, INFERRED: 3 };

export function priorityRank(priority?: string): number {
  if (priority === undefined) return 4;
  return PRIORITY_RANK[priority] ?? 4;
}

// priorityPillClass mirrors internal/report/html.go's priorityClass — the
// lowercase CSS class ("p1".."p4") for a Priority string, falling back to
// "p4" for an empty/unrecognized value (e.g. a pre-priority legacy
// findings.json) so a pill never renders with an empty class.
export function priorityPillClass(priority?: string): "p1" | "p2" | "p3" | "p4" {
  switch (priority) {
    case "P1":
    case "P2":
    case "P3":
    case "P4":
      return priority.toLowerCase() as "p1" | "p2" | "p3" | "p4";
    default:
      return "p4";
  }
}

function confidenceRank(confidence: string): number {
  return CONFIDENCE_RANK[confidence] ?? 4;
}

// compareFindings mirrors Go's findingLess (internal/report/view.go): the
// one sort order every surface uses — Priority first (P1 most urgent),
// Severity second, Confidence third, then rule ID/resource for a stable
// tie-break. Priority already reflects globalBlocker (see
// findings.AssignPriority, Go) — no separate globalBlocker-first check
// needed on top of this.
export function compareFindings(a: Finding, b: Finding): number {
  return (
    priorityRank(a.priority) - priorityRank(b.priority) ||
    SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] ||
    confidenceRank(a.confidence) - confidenceRank(b.confidence) ||
    a.ruleId.localeCompare(b.ruleId) ||
    findingResourceLabel(a).localeCompare(findingResourceLabel(b))
  );
}

// topRisks: the highest-priority findings first (see compareFindings),
// truncated to `limit` — used for the Console's Top Risks strip and
// report.html's executive summary. Not a scoring model, just "worst
// findings first," matching the same severity-then-rule-ID order every
// other renderer (terminal/Markdown/HTML) already sorts by.
export function topRisks(findings: Finding[], limit = 3): Finding[] {
  return [...findings].sort(compareFindings).slice(0, limit);
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
  return findings
    .filter((finding) => {
      const namespaces = finding.resources.map((resource) => resource.namespace || "cluster-scoped");
      const haystack = [finding.ruleId, finding.message, findingResourceLabel(finding), ...namespaces].join(" ").toLowerCase();
      return (
        (!query || haystack.includes(query)) &&
        (!filters.severities || filters.severities.includes(finding.severity)) &&
        (!filters.confidence || finding.confidence === filters.confidence) &&
        (!filters.namespace || namespaces.includes(filters.namespace))
      );
    })
    .sort(compareFindings);
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
