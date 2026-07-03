package rules

import (
	"path/filepath"
	"testing"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestWH001_Positive_CatchAllFailClosed(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (WH001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "WH-001" {
		t.Errorf("RuleID = %q, want WH-001", f.RuleID)
	}
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resource.Name != "catch-all-guard" {
		t.Errorf("Resource.Name = %q, want catch-all-guard", f.Resource.Name)
	}
}

func TestWH001_Negative_ScopedWebhookNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh001", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (WH001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (narrow scope must not fire): %+v", len(fs), fs)
	}
}
