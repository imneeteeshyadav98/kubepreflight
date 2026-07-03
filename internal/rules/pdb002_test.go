package rules

import (
	"path/filepath"
	"testing"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestPDB002_Positive_OverlappingSelectors(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb002", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "PDB-002" {
		t.Errorf("RuleID = %q, want PDB-002", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Resource.Namespace != "kube-system" {
		t.Errorf("Resource.Namespace = %q, want kube-system", f.Resource.Namespace)
	}
}

func TestPDB002_Negative_DisjointSelectorsNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb002", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (disjoint selectors must not fire): %+v", len(fs), fs)
	}
}

func TestPDB002_FingerprintOrderIndependent(t *testing.T) {
	if got, want := pdbPairKey("uid-b", "uid-a"), pdbPairKey("uid-a", "uid-b"); got != want {
		t.Errorf("pdbPairKey not order-independent: %q vs %q", got, want)
	}
}
