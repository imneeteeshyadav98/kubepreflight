package findings

import (
	"fmt"
	"strings"
)

type UpgradeContext string

const (
	UpgradeContextUnspecified         UpgradeContext = "unspecified"
	UpgradeContextAuditOnly           UpgradeContext = "audit_only"
	UpgradeContextControlPlaneOnly    UpgradeContext = "control_plane_only"
	UpgradeContextWorkerRollout       UpgradeContext = "worker_rollout"
	UpgradeContextFullPlatformUpgrade UpgradeContext = "full_platform_upgrade"
	UpgradeContextWorkloadRestart     UpgradeContext = "workload_restart"
)

var upgradeContextCLIValues = map[string]UpgradeContext{
	"unspecified":           UpgradeContextUnspecified,
	"audit-only":            UpgradeContextAuditOnly,
	"control-plane-only":    UpgradeContextControlPlaneOnly,
	"worker-rollout":        UpgradeContextWorkerRollout,
	"full-platform-upgrade": UpgradeContextFullPlatformUpgrade,
	"workload-restart":      UpgradeContextWorkloadRestart,
}

func ParseUpgradeContextFlag(value string) (UpgradeContext, error) {
	if value == "" {
		return UpgradeContextUnspecified, nil
	}
	if ctx, ok := upgradeContextCLIValues[value]; ok {
		return ctx, nil
	}
	return "", fmt.Errorf("unsupported upgrade context %q (use %s)", value, strings.Join(UpgradeContextFlagValues(), ", "))
}

func UpgradeContextFlagValues() []string {
	return []string{"unspecified", "audit-only", "control-plane-only", "worker-rollout", "full-platform-upgrade", "workload-restart"}
}

func (c UpgradeContext) Validate() error {
	switch c {
	case "", UpgradeContextUnspecified, UpgradeContextAuditOnly, UpgradeContextControlPlaneOnly, UpgradeContextWorkerRollout, UpgradeContextFullPlatformUpgrade, UpgradeContextWorkloadRestart:
		return nil
	default:
		return fmt.Errorf("invalid upgrade context %q", c)
	}
}

type ImpactScope string

const (
	ImpactScopeControlPlaneUpgrade ImpactScope = "control_plane_upgrade"
	ImpactScopeWorkerRollout       ImpactScope = "worker_rollout"
	ImpactScopeNodeDrain           ImpactScope = "node_drain"
	ImpactScopeAPIWrite            ImpactScope = "api_write"
	ImpactScopeWorkloadRestart     ImpactScope = "workload_restart"
	ImpactScopeCurrentHealth       ImpactScope = "current_health"
	ImpactScopeFutureMaintenance   ImpactScope = "future_maintenance"
	ImpactScopeCRDConversion       ImpactScope = "crd_conversion"
	ImpactScopeAggregatedAPI       ImpactScope = "aggregated_api"
)

func (s ImpactScope) Validate() error {
	switch s {
	case ImpactScopeControlPlaneUpgrade, ImpactScopeWorkerRollout, ImpactScopeNodeDrain, ImpactScopeAPIWrite, ImpactScopeWorkloadRestart, ImpactScopeCurrentHealth, ImpactScopeFutureMaintenance, ImpactScopeCRDConversion, ImpactScopeAggregatedAPI:
		return nil
	default:
		return fmt.Errorf("invalid impact scope %q", s)
	}
}

type UpgradeGate string

const (
	UpgradeGateBlock            UpgradeGate = "block"
	UpgradeGateAllow            UpgradeGate = "allow"
	UpgradeGateOperatorDecision UpgradeGate = "operator_decision"
)

func (g UpgradeGate) Validate() error {
	switch g {
	case "", UpgradeGateBlock, UpgradeGateAllow, UpgradeGateOperatorDecision:
		return nil
	default:
		return fmt.Errorf("invalid upgrade gate %q", g)
	}
}

func (f Finding) EffectiveUpgradeGate() UpgradeGate {
	if f.UpgradeGate != "" {
		return f.UpgradeGate
	}
	if f.GlobalBlocker || f.Severity == SeverityBlocker {
		return UpgradeGateBlock
	}
	return UpgradeGateAllow
}
