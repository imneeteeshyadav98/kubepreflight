package rules

import (
	"github.com/imneeteeshyadav98/kubepreflight/internal/compatcatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func scanUpgradeContext(sc *ScanContext) findings.UpgradeContext {
	if sc == nil || sc.UpgradeContext == "" {
		return findings.UpgradeContextUnspecified
	}
	return sc.UpgradeContext
}

func drainDependentGate(ctx findings.UpgradeContext) (findings.Severity, findings.UpgradeGate) {
	switch ctx {
	case findings.UpgradeContextWorkerRollout, findings.UpgradeContextFullPlatformUpgrade:
		return findings.SeverityBlocker, findings.UpgradeGateBlock
	case findings.UpgradeContextAuditOnly, findings.UpgradeContextControlPlaneOnly:
		return findings.SeverityWarning, findings.UpgradeGateAllow
	default:
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision
	}
}

func currentHealthGate(ctx findings.UpgradeContext, criticalInfra, zeroReady bool) (findings.Severity, findings.UpgradeGate) {
	if zeroReady && criticalInfra && (ctx == findings.UpgradeContextWorkerRollout || ctx == findings.UpgradeContextFullPlatformUpgrade) {
		return findings.SeverityBlocker, findings.UpgradeGateBlock
	}
	if ctx == findings.UpgradeContextControlPlaneOnly && !criticalInfra {
		return findings.SeverityWarning, findings.UpgradeGateAllow
	}
	if ctx == findings.UpgradeContextAuditOnly {
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision
	}
	if ctx == findings.UpgradeContextWorkerRollout || ctx == findings.UpgradeContextFullPlatformUpgrade || ctx == findings.UpgradeContextWorkloadRestart || ctx == findings.UpgradeContextUnspecified || criticalInfra {
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision
	}
	return findings.SeverityWarning, findings.UpgradeGateAllow
}

func eksControlPlanePreconditionGate(ctx findings.UpgradeContext) (findings.Severity, findings.UpgradeGate) {
	switch ctx {
	case findings.UpgradeContextControlPlaneOnly, findings.UpgradeContextFullPlatformUpgrade:
		return findings.SeverityBlocker, findings.UpgradeGateBlock
	case findings.UpgradeContextAuditOnly:
		return findings.SeverityWarning, findings.UpgradeGateAllow
	default:
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision
	}
}

func addonCompatibilityGate(ctx findings.UpgradeContext, impacts []compatcatalog.OperationalImpact) (findings.Severity, findings.UpgradeGate, []findings.ImpactScope) {
	if len(impacts) == 0 {
		impacts = []compatcatalog.OperationalImpact{compatcatalog.OperationalImpactUnknown}
	}
	switch ctx {
	case findings.UpgradeContextAuditOnly:
		return findings.SeverityWarning, findings.UpgradeGateAllow, addonImpactScopes(impacts)
	case findings.UpgradeContextControlPlaneOnly:
		if hasAnyOperationalImpact(impacts,
			compatcatalog.OperationalImpactControlPlaneDependency,
			compatcatalog.OperationalImpactAdmissionAPI,
			compatcatalog.OperationalImpactClusterDNS,
		) {
			return findings.SeverityBlocker, findings.UpgradeGateBlock, addonImpactScopes(impacts)
		}
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision, addonImpactScopes(impacts)
	case findings.UpgradeContextWorkerRollout:
		if hasAnyOperationalImpact(impacts,
			compatcatalog.OperationalImpactWorkerRolloutDependency,
			compatcatalog.OperationalImpactNetworkingDataPlane,
			compatcatalog.OperationalImpactStorageDataPlane,
			compatcatalog.OperationalImpactClusterDNS,
		) {
			return findings.SeverityBlocker, findings.UpgradeGateBlock, addonImpactScopes(impacts)
		}
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision, addonImpactScopes(impacts)
	case findings.UpgradeContextFullPlatformUpgrade:
		if hasAnyOperationalImpact(impacts,
			compatcatalog.OperationalImpactControlPlaneDependency,
			compatcatalog.OperationalImpactWorkerRolloutDependency,
			compatcatalog.OperationalImpactNetworkingDataPlane,
			compatcatalog.OperationalImpactStorageDataPlane,
			compatcatalog.OperationalImpactClusterDNS,
			compatcatalog.OperationalImpactAdmissionAPI,
			compatcatalog.OperationalImpactWorkloadDependency,
		) {
			return findings.SeverityBlocker, findings.UpgradeGateBlock, addonImpactScopes(impacts)
		}
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision, addonImpactScopes(impacts)
	case findings.UpgradeContextWorkloadRestart, findings.UpgradeContextUnspecified:
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision, addonImpactScopes(impacts)
	default:
		return findings.SeverityWarning, findings.UpgradeGateOperatorDecision, addonImpactScopes(impacts)
	}
}

func hasAnyOperationalImpact(impacts []compatcatalog.OperationalImpact, matches ...compatcatalog.OperationalImpact) bool {
	wanted := make(map[compatcatalog.OperationalImpact]bool, len(matches))
	for _, match := range matches {
		wanted[match] = true
	}
	for _, impact := range impacts {
		if wanted[impact] {
			return true
		}
	}
	return false
}

func addonImpactScopes(impacts []compatcatalog.OperationalImpact) []findings.ImpactScope {
	var out []findings.ImpactScope
	for _, impact := range impacts {
		switch impact {
		case compatcatalog.OperationalImpactControlPlaneDependency:
			out = appendUniqueImpactScope(out, findings.ImpactScopeControlPlaneUpgrade)
		case compatcatalog.OperationalImpactWorkerRolloutDependency:
			out = appendUniqueImpactScope(out, findings.ImpactScopeWorkerRollout)
		case compatcatalog.OperationalImpactNetworkingDataPlane, compatcatalog.OperationalImpactStorageDataPlane, compatcatalog.OperationalImpactClusterDNS, compatcatalog.OperationalImpactWorkloadDependency:
			out = appendUniqueImpactScope(out, findings.ImpactScopeWorkloadRestart)
		case compatcatalog.OperationalImpactAdmissionAPI:
			out = appendUniqueImpactScope(out, findings.ImpactScopeAPIWrite)
		case compatcatalog.OperationalImpactOptionalEcosystem, compatcatalog.OperationalImpactOperatorReview, compatcatalog.OperationalImpactUnknown:
			out = appendUniqueImpactScope(out, findings.ImpactScopeFutureMaintenance)
		}
	}
	return out
}

func appendUniqueImpactScope(scopes []findings.ImpactScope, scope findings.ImpactScope) []findings.ImpactScope {
	for _, existing := range scopes {
		if existing == scope {
			return scopes
		}
	}
	return append(scopes, scope)
}
