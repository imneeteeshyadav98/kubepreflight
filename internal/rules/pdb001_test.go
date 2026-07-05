package rules

import (
	"path/filepath"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestPDB001_Positive_ZeroDisruptionsAllowed(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "PDB-001" {
		t.Errorf("RuleID = %q, want PDB-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Resources[0].Name != "singleton-pdb" || f.Resources[0].Namespace != "payments" {
		t.Errorf("Resources = %+v, want payments/singleton-pdb", f.Resources)
	}
}

func TestPDB001_Negative_DisruptionsAllowedNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb001", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (disruptionsAllowed > 0 must not fire): %+v", len(fs), fs)
	}
}

func TestPDB001_RemediationDetail_AbsoluteMinAvailableIncludesReplicasChange(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	rd := fs[0].RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated")
	}
	if len(rd.Changes) != 2 {
		t.Fatalf("Changes = %+v, want 2 rows (disruptionsAllowed + replicas) for an absolute minAvailable", rd.Changes)
	}
	var replicasChange *findings.RemediationChange
	for i := range rd.Changes {
		if rd.Changes[i].Field == "replicas" {
			replicasChange = &rd.Changes[i]
		}
	}
	if replicasChange == nil {
		t.Fatalf("no replicas change row found in %+v", rd.Changes)
	}
	if replicasChange.Current != "1" || replicasChange.Required != "2" {
		t.Errorf("replicas change = %+v, want current=1 required=2 (expectedPods=1, minAvailable=1)", replicasChange)
	}
	if rd.SafeFix == nil || rd.Emergency == nil {
		t.Errorf("SafeFix/Emergency = %+v/%+v, want both populated", rd.SafeFix, rd.Emergency)
	}
	if rd.VerifyCommand == "" || rd.ExpectedResult == "" {
		t.Error("VerifyCommand/ExpectedResult must be populated")
	}
}

// TestPDB001_RemediationDetail_PercentageMinAvailableOmitsReplicasChange
// guards that a percentage-based minAvailable doesn't get a fabricated
// "required replicas" number — only the honestly-derivable
// disruptionsAllowed row is shown.
func TestPDB001_RemediationDetail_PercentageMinAvailableOmitsReplicasChange(t *testing.T) {
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "pct-pdb", Namespace: "payments"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.String, StrVal: "50%"},
		},
		Status: policyv1.PodDisruptionBudgetStatus{
			DisruptionsAllowed: 0, CurrentHealthy: 2, DesiredHealthy: 2, ExpectedPods: 2,
		},
	}
	f := pdb001Finding(pdb, "1.34")
	rd := f.RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated")
	}
	if len(rd.Changes) != 1 || rd.Changes[0].Field != "disruptionsAllowed" {
		t.Errorf("Changes = %+v, want exactly the disruptionsAllowed row", rd.Changes)
	}
}
