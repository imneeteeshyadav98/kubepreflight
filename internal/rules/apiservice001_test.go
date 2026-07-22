package rules

import (
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func TestAPIService001_Unavailable(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, UnavailableAPIServices: []k8s.APIServiceAvailability{{Name: "v1beta1.metrics.k8s.io", UID: "uid-api", Reason: "MissingEndpoints", Message: "no endpoints"}}}
	fs, err := (APIService001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 1 || fs[0].RuleID != "APISERVICE-001" {
		t.Fatalf("Evaluate() = %+v, %v; want one APISERVICE-001 finding", fs, err)
	}
	if fs[0].Severity != findings.SeverityWarning || fs[0].UpgradeGate != findings.UpgradeGateOperatorDecision {
		t.Fatalf("severity/gate = %s/%s, want Warning/operator_decision", fs[0].Severity, fs[0].UpgradeGate)
	}
}
