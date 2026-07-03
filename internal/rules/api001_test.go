package rules

import (
	"path/filepath"
	"testing"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestAPI001_Positive_LiveObjectAtRemovedAPI(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "positive")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("BuildSnapshot matched %d deprecated-API objects, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	// policy/v1beta1 PodSecurityPolicy was removed in Kubernetes 1.25;
	// target 1.34 has long since passed that, so this must fire.
	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "API-001" {
		t.Errorf("RuleID = %q, want API-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resource.Kind != "PodSecurityPolicy" || f.Resource.Name != "restricted" {
		t.Errorf("Resource = %+v, want PodSecurityPolicy/restricted", f.Resource)
	}
}

func TestAPI001_Negative_TargetVersionBeforeRemoval(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "negative")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("BuildSnapshot matched %d deprecated-API objects, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	// autoscaling/v2beta2 HPA is removed in Kubernetes 1.26. Target 1.24
	// hasn't reached that yet, so this object is still perfectly valid at
	// the target version and must not fire.
	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (target version before removal must not fire): %+v", len(fs), fs)
	}
}

func TestAPI001_Negative_UnmatchedObjectNotCollected(t *testing.T) {
	// A live object at a *current* API version should never even reach
	// DeprecatedAPIUsage — BuildSnapshot only records objects whose GVK
	// matches an apicatalog.Deprecated entry.
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)
	if len(snap.DeprecatedAPIUsage) != 0 {
		t.Fatalf("got %d DeprecatedAPIUsage entries from unrelated fixtures, want 0: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}
}
