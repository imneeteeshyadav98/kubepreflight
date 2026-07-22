package rules

import "github.com/imneeteeshyadav98/kubepreflight/internal/findings"

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
