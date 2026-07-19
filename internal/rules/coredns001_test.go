package rules

import (
	"path/filepath"
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

func TestCOREDNS001_Positive_MissingReadyPlugin(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "coredns001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)
	if snap.CoreDNSConfigMap == nil {
		t.Fatalf("BuildSnapshot did not pick up the CoreDNS ConfigMap fixture")
	}

	fs, err := (COREDNS001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "COREDNS-001" {
		t.Errorf("RuleID = %q, want COREDNS-001", f.RuleID)
	}
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resources[0].Namespace != "kube-system" || f.Resources[0].Name != "coredns" {
		t.Errorf("Resources = %+v, want kube-system/coredns", f.Resources)
	}
}

func TestCOREDNS001_Negative_HasReadyPluginNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "coredns001", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (COREDNS001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (ready plugin present): %+v", len(fs), fs)
	}
}

func TestCOREDNS001_Negative_NoConfigMapNoFinding(t *testing.T) {
	snap := testutil.BuildSnapshot(nil)
	fs, err := (COREDNS001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when no CoreDNS ConfigMap was found: %+v", len(fs), fs)
	}
}

func TestHasReadyPlugin_DoesNotFalsePositiveOnSubstring(t *testing.T) {
	// "readyz" and a comment mentioning "ready" must not count as the
	// plugin directive.
	corefile := ".:53 {\n    readyz\n    # ready check disabled\n}\n"
	if hasReadyPlugin(corefile) {
		t.Error("hasReadyPlugin matched a substring/comment, want false")
	}
}
