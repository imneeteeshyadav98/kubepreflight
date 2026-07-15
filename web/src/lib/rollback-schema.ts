export type RollbackReasonCode = string;

export type RollbackCheck = {
  id: string;
  title: string;
  status: "pass" | "warning" | "fail" | "unknown";
  reasonCodes: RollbackReasonCode[];
  evidence: string[];
};

export type RollbackAssessment = {
  schemaVersion: "kubepreflight.io/rollback-assessment/v1alpha1";
  mode: "pre_upgrade_posture" | "post_upgrade_readiness";
  cluster: {
    name: string;
    region?: string;
    currentVersion: string;
    rollbackTargetVersion?: string;
    provider?: string;
  };
  eligibility: {
    status: "eligible" | "unavailable" | "unknown";
    source?: string;
    reasonCodes: RollbackReasonCode[];
    remainingMinutes?: number;
  };
  readiness: {
    status: "ready" | "blocked" | "high_risk" | "insufficient_evidence";
    blockers: number;
    warnings: number;
    unknowns: number;
  };
  recommendation: {
    decision: "rollback_preferred" | "fix_forward_preferred" | "operator_decision_required" | "do_not_proceed";
    confidence: "high" | "medium" | "low";
    reasonCodes: RollbackReasonCode[];
  };
  evidence: {
    complete: boolean;
    windowCalculation?: string;
    timestampSource?: string;
    eksInsightsRefreshedAt?: string;
    clusterObservedAt?: string;
  };
  checks: RollbackCheck[];
  generatedAt?: string;
};

export function parseRollbackAssessment(input: string): RollbackAssessment {
  const raw = JSON.parse(input) as Partial<RollbackAssessment>;
  if (raw.schemaVersion !== "kubepreflight.io/rollback-assessment/v1alpha1") {
    throw new Error("Unsupported rollback assessment schema");
  }
  if (!raw.cluster || !raw.eligibility || !raw.readiness || !raw.recommendation || !raw.evidence) {
    throw new Error("Rollback assessment is missing required sections");
  }
  return {
    schemaVersion: raw.schemaVersion,
    mode: raw.mode ?? "post_upgrade_readiness",
    cluster: {
      name: raw.cluster.name ?? "",
      region: raw.cluster.region,
      currentVersion: raw.cluster.currentVersion ?? "",
      rollbackTargetVersion: raw.cluster.rollbackTargetVersion,
      provider: raw.cluster.provider,
    },
    eligibility: {
      status: raw.eligibility.status ?? "unknown",
      source: raw.eligibility.source,
      reasonCodes: raw.eligibility.reasonCodes ?? [],
      remainingMinutes: raw.eligibility.remainingMinutes,
    },
    readiness: {
      status: raw.readiness.status ?? "insufficient_evidence",
      blockers: raw.readiness.blockers ?? 0,
      warnings: raw.readiness.warnings ?? 0,
      unknowns: raw.readiness.unknowns ?? 0,
    },
    recommendation: {
      decision: raw.recommendation.decision ?? "operator_decision_required",
      confidence: raw.recommendation.confidence ?? "low",
      reasonCodes: raw.recommendation.reasonCodes ?? [],
    },
    evidence: {
      complete: raw.evidence.complete === true,
      windowCalculation: raw.evidence.windowCalculation,
      timestampSource: raw.evidence.timestampSource,
      eksInsightsRefreshedAt: raw.evidence.eksInsightsRefreshedAt,
      clusterObservedAt: raw.evidence.clusterObservedAt,
    },
    checks: (raw.checks ?? []).map((check) => ({
      id: check.id ?? "",
      title: check.title ?? "",
      status: check.status ?? "unknown",
      reasonCodes: check.reasonCodes ?? [],
      evidence: check.evidence ?? [],
    })),
    generatedAt: raw.generatedAt,
  };
}

export function rollbackDecisionLabel(decision: RollbackAssessment["recommendation"]["decision"]): string {
  return decision.replace(/_/g, " ");
}

export function rollbackStatusClass(status: string): "clean" | "warning" | "blocked" {
  if (status === "ready" || status === "eligible" || status === "rollback_preferred") return "clean";
  if (status === "blocked" || status === "unavailable" || status === "do_not_proceed") return "blocked";
  return "warning";
}
