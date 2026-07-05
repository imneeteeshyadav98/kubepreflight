package rules

import (
	"testing"

	"kubepreflight/internal/collectors/k8s"
)

func TestAPIService001_Unavailable(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{}, UnavailableAPIServices: []k8s.APIServiceAvailability{{Name: "v1beta1.metrics.k8s.io", UID: "uid-api", Reason: "MissingEndpoints", Message: "no endpoints"}}}
	fs, err := (APIService001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 1 || fs[0].RuleID != "APISERVICE-001" {
		t.Fatalf("Evaluate() = %+v, %v; want one APISERVICE-001 finding", fs, err)
	}
}
