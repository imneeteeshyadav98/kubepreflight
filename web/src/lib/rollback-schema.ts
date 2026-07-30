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

// Human-readable translations for RollbackReasonCode. Presentation only —
// mirrors internal/rollback/model.go's ReasonCode const block (and that
// file's doc comments for the ROLLBACK_EVIDENCE_* family) verbatim in
// meaning, so Console wording never drifts from what the Go engine actually
// checks. Keep this in sync when a new ReasonCode constant is added there.
export type RollbackReasonCodeLabel = { title: string; explanation: string };

const ROLLBACK_REASON_CODE_LABELS: Record<string, RollbackReasonCodeLabel> = {
  UPGRADE_HISTORY_UNAVAILABLE: {
    title: "Upgrade history unavailable",
    explanation: "The cluster's upgrade history could not be retrieved, so whether this was an in-place upgrade cannot be confirmed.",
  },
  EKS_UPGRADE_HISTORY_UNAVAILABLE: {
    title: "EKS upgrade history unavailable",
    explanation: "Amazon EKS's own upgrade update history could not be retrieved for this cluster.",
  },
  UPGRADE_WAS_NOT_IN_PLACE: {
    title: "Upgrade was not in-place",
    explanation: "The current version was not reached through a normal in-place upgrade, so rollback assumptions may not hold.",
  },
  ROLLBACK_WINDOW_EXPIRED: {
    title: "Rollback window has expired",
    explanation: "The safe rollback window for this upgrade has already closed.",
  },
  ROLLBACK_WINDOW_NEAR_EXPIRY: {
    title: "Rollback window closing soon",
    explanation: "The safe rollback window for this upgrade is about to close.",
  },
  PREVIOUS_VERSION_NOT_N_MINUS_ONE: {
    title: "Target is not the previous minor version",
    explanation: "The requested rollback target is not exactly one minor version behind the current version.",
  },
  CLUSTER_NOT_ACTIVE: {
    title: "Cluster is not active",
    explanation: "The cluster is not currently in an ACTIVE state, so a rollback cannot be safely assessed.",
  },
  ROLLBACK_TARGET_UNSUPPORTED: {
    title: "Rollback target version is unsupported",
    explanation: "The requested rollback target version is not a version Amazon EKS currently supports.",
  },
  ROLLBACK_TARGET_REQUIRES_EXTENDED_SUPPORT: {
    title: "Target requires EKS Extended Support",
    explanation: "The requested rollback target is only available under Amazon EKS Extended Support.",
  },
  UPGRADE_POLICY_DISALLOWS_ROLLBACK_TARGET: {
    title: "Cluster upgrade policy disallows this target",
    explanation: "The cluster's configured upgrade policy does not allow rolling back to this version.",
  },
  END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE: {
    title: "Extended support has ended — auto-upgrade applies",
    explanation: "Extended support for the current version has ended, so Amazon EKS will auto-upgrade the cluster regardless of a rollback request.",
  },
  END_OF_EXTENDED_SUPPORT_AUTO_UPGRADE_UNVERIFIED: {
    title: "End-of-extended-support status unverified",
    explanation: "Whether Amazon EKS's end-of-extended-support auto-upgrade applies to this cluster could not be confirmed.",
  },
  EKS_FEATURE_COMPATIBILITY_UNVERIFIED: {
    title: "EKS feature compatibility unverified",
    explanation: "Whether enabled EKS features remain compatible with the rollback target could not be confirmed.",
  },
  EKS_FEATURE_INCOMPATIBLE: {
    title: "EKS feature is incompatible with target",
    explanation: "An enabled EKS feature is confirmed incompatible with the rollback target version.",
  },
  INCOMPATIBLE_EKS_FEATURE_ENABLED: {
    title: "An incompatible EKS feature is enabled",
    explanation: "A cluster feature is enabled that is known not to work on the rollback target version.",
  },
  EKS_INSIGHTS_UNAVAILABLE: {
    title: "EKS Upgrade Insights unavailable",
    explanation: "Amazon EKS's own upgrade-readiness insights could not be retrieved for this cluster.",
  },
  EKS_INSIGHTS_STALE: {
    title: "EKS Upgrade Insights are stale",
    explanation: "Amazon EKS's upgrade-readiness insights are older than their normal refresh window.",
  },
  EKS_INSIGHTS_BLOCKING: {
    title: "EKS Upgrade Insights report a blocker",
    explanation: "Amazon EKS's own upgrade-readiness insights report a blocking issue for this cluster.",
  },
  MANAGED_NODEGROUP_ROLLBACK_REQUIRED: {
    title: "Managed node groups also need rollback",
    explanation: "This cluster has managed node groups that must be rolled back along with the control plane.",
  },
  SELF_MANAGED_NODE_EVIDENCE_UNAVAILABLE: {
    title: "Self-managed node evidence unavailable",
    explanation: "Self-managed nodes exist on this cluster but their rollback readiness could not be evaluated.",
  },
  SELF_MANAGED_NODE_ROLLBACK_REQUIRED: {
    title: "Self-managed nodes also need rollback",
    explanation: "This cluster has self-managed nodes that must be rolled back along with the control plane.",
  },
  WORKER_NODES_WOULD_REMAIN_NEWER_THAN_CONTROL_PLANE: {
    title: "Worker nodes would outpace the control plane",
    explanation: "Rolling back only the control plane would leave worker nodes on a newer Kubernetes version than is supported.",
  },
  AUTO_MODE_DISRUPTION_RISK: {
    title: "EKS Auto Mode disruption risk",
    explanation: "EKS Auto Mode is enabled, which can add disruption risk during a rollback.",
  },
  FARGATE_EVIDENCE_UNAVAILABLE: {
    title: "Fargate evidence unavailable",
    explanation: "Fargate profiles exist on this cluster but their rollback impact could not be evaluated.",
  },
  FARGATE_POD_RECREATION_RISK: {
    title: "Fargate pods will be recreated",
    explanation: "Fargate pods are recreated (not rolled back in place) during this operation, which carries its own disruption risk.",
  },
  MANAGED_ADDON_ROLLBACK_REQUIRED: {
    title: "Managed add-ons also need rollback",
    explanation: "This cluster has managed add-ons that must be rolled back along with the control plane.",
  },
  MANAGED_ADDON_COMPATIBILITY_UNKNOWN: {
    title: "Managed add-on compatibility unknown",
    explanation: "Whether installed managed add-ons remain compatible with the rollback target could not be confirmed.",
  },
  SELF_MANAGED_ADDON_COMPATIBILITY_UNKNOWN: {
    title: "Self-managed add-on compatibility unknown",
    explanation: "Whether installed self-managed add-ons remain compatible with the rollback target could not be confirmed.",
  },
  NEW_VERSION_API_ADOPTION_RISK: {
    title: "Newer API versions may already be in use",
    explanation: "Workloads may already be using API versions introduced after the rollback target, which would break on rollback.",
  },
  CRD_WEBHOOK_CONTROLLER_RISK: {
    title: "CRD/webhook controller compatibility risk",
    explanation: "Installed CustomResourceDefinitions or admission webhooks may not be compatible with the rollback target.",
  },
  PDB_DISRUPTION_CONSTRAINTS: {
    title: "PodDisruptionBudgets may block the rollback",
    explanation: "Existing PodDisruptionBudgets may prevent the node-level changes a rollback requires.",
  },
  UNHEALTHY_WORKLOADS_PRESENT: {
    title: "Unhealthy workloads present",
    explanation: "Some workloads are already unhealthy, which can make rollback impact hard to distinguish from a pre-existing issue.",
  },
  OBSERVABILITY_EVIDENCE_MISSING: {
    title: "Observability evidence missing",
    explanation: "Evidence needed to confirm workload health during the rollback window is missing.",
  },
  APPLICATION_HEALTH_UNKNOWN: {
    title: "Application health unknown",
    explanation: "Application-level health could not be confirmed from the available evidence.",
  },
  FORCE_BYPASS_NOT_RECOMMENDED: {
    title: "Bypassing this check is not recommended",
    explanation: "This check can be bypassed, but doing so is not recommended given the current evidence.",
  },
  ROLLBACK_EVIDENCE_TARGET_MISMATCH: {
    title: "Evidence was scanned for a different target version",
    explanation: "The supplied findings were scanned for a different target version than this rollback's target, so they cannot confirm this rollback's API compatibility.",
  },
  ROLLBACK_EVIDENCE_TARGET_UNKNOWN: {
    title: "Evidence target version could not be confirmed",
    explanation: "The target version recorded on the supplied findings could not be confirmed against this rollback's target.",
  },
  ROLLBACK_EVIDENCE_CLUSTER_MISMATCH: {
    title: "Evidence was scanned from a different cluster",
    explanation: "The supplied findings identify a different cluster than the one being assessed, so they cannot be trusted as current evidence for this cluster.",
  },
  ROLLBACK_EVIDENCE_CLUSTER_UNKNOWN: {
    title: "Evidence cluster identity could not be confirmed",
    explanation: "The cluster identity recorded on the supplied findings could not be confirmed against the cluster being assessed.",
  },
  ROLLBACK_EVIDENCE_STALE: {
    title: "Evidence is too old to trust",
    explanation: "The supplied findings are older than the maximum age this assessment trusts as current evidence.",
  },
  ROLLBACK_EVIDENCE_TIMESTAMP_UNKNOWN: {
    title: "Evidence age could not be confirmed",
    explanation: "The supplied findings' scan time could not be confirmed, so their freshness is treated as unknown rather than current.",
  },
};

// rollbackReasonCodeLabel never throws and never returns an empty title —
// a reason code introduced in internal/rollback after this map was last
// updated falls back to a generic, still-informative label rather than
// breaking the page or silently dropping the reason.
export function rollbackReasonCodeLabel(code: RollbackReasonCode): RollbackReasonCodeLabel {
  return ROLLBACK_REASON_CODE_LABELS[code] ?? { title: "Unknown rollback reason", explanation: "" };
}

const ELIGIBILITY_STATUS_LABELS: Record<string, string> = {
  eligible: "Eligible",
  unavailable: "Unavailable",
  unknown: "Unknown",
};

export function eligibilityStatusLabel(status: string): string {
  return ELIGIBILITY_STATUS_LABELS[status] ?? status;
}

const READINESS_STATUS_LABELS: Record<string, string> = {
  ready: "Ready",
  blocked: "Blocked",
  high_risk: "High risk",
  insufficient_evidence: "Insufficient evidence",
};

export function readinessStatusLabel(status: string): string {
  return READINESS_STATUS_LABELS[status] ?? status;
}
