package rules

import (
	"path/filepath"
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

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
	if f.Resources[0].Name != "catch-all-guard" {
		t.Errorf("resource name = %q, want catch-all-guard", f.Resources[0].Name)
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

// TestWH001_Positive_SubresourceWildcard is a regression test for a real
// gap found during live EKS testing: a webhook using resources: ["*/*"]
// (all resources and subresources) didn't fire, because hasCatchAllRule
// only recognized the literal "*" spelling. The fixture is the actual
// webhook manifest content from that test run.
func TestWH001_Positive_SubresourceWildcard(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh001", "positive-subresource-wildcard")
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
		t.Fatalf("got %d findings, want 1 (\"*/*\" must be recognized as catch-all): %+v", len(fs), fs)
	}

	foundPattern := false
	for _, e := range fs[0].Evidence {
		if strings.Contains(e, `"*/*"`) {
			foundPattern = true
		}
	}
	if !foundPattern {
		t.Errorf("evidence must cite the actual matched pattern (*/*), got: %v", fs[0].Evidence)
	}
}

func TestHasCatchAllRule_RecognizesBothWildcardSpellings(t *testing.T) {
	cases := []struct {
		name      string
		resources []string
		want      bool
	}{
		{"plain wildcard", []string{"*"}, true},
		{"subresource wildcard", []string{"*/*"}, true},
		{"narrow list", []string{"pods", "deployments"}, false},
		{"specific subresource only", []string{"pods/status"}, false},
	}
	for _, c := range cases {
		rules := []admissionregistrationv1.RuleWithOperations{
			{Rule: admissionregistrationv1.Rule{APIGroups: []string{"*"}, Resources: c.resources}},
		}
		matched, _ := hasCatchAllRule(rules)
		if matched != c.want {
			t.Errorf("hasCatchAllRule(%v) = %v, want %v", c.resources, matched, c.want)
		}
	}
}
