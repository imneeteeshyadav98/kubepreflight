package rules

import (
	"path/filepath"
	"strings"
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
	if len(f.Resources) != 2 || f.Resources[0].Namespace != "kube-system" || f.Resources[1].Namespace != "kube-system" {
		t.Errorf("Resources = %+v, want two kube-system PDB references", f.Resources)
	}

	rd := f.RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated")
	}
	if rd.SafeFix == nil || rd.SafeFix.Command == "" {
		t.Errorf("SafeFix = %+v, want a populated command offering both delete options", rd.SafeFix)
	}
	if !strings.Contains(rd.SafeFix.Command, "kubectl delete pdb "+f.Resources[0].Name) || !strings.Contains(rd.SafeFix.Command, "kubectl delete pdb "+f.Resources[1].Name) {
		t.Errorf("SafeFix.Command = %q, want delete commands for both %q and %q", rd.SafeFix.Command, f.Resources[0].Name, f.Resources[1].Name)
	}
	if rd.VerifyCommand == "" {
		t.Error("VerifyCommand is empty, want a kubectl get pdb command")
	}
	if rd.Emergency != nil {
		t.Errorf("Emergency = %+v, want nil (no safe temporary shortcut for an eviction-blocking overlap)", rd.Emergency)
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
	a := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "ns", "a", "uid-a")
	b := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "ns", "b", "uid-b")
	got := findings.FingerprintV2("PDB-002", "1.34", "", b, a)
	want := findings.FingerprintV2("PDB-002", "1.34", "", a, b)
	if got != want {
		t.Errorf("PDB-002 structured fingerprint not order-independent: %q vs %q", got, want)
	}
}
