package rules

import (
	"path/filepath"
	"testing"

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
	if f.Resource.Name != "singleton-pdb" || f.Resource.Namespace != "payments" {
		t.Errorf("Resource = %+v, want payments/singleton-pdb", f.Resource)
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
