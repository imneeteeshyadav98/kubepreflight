package rules

import (
	"path/filepath"
	"strings"
	"testing"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestWH002_Positive_FailClosedNoReadyEndpoints(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh002", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (WH002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "WH-002" {
		t.Errorf("RuleID = %q, want WH-002", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resource.Kind != "ValidatingWebhookConfiguration" {
		t.Errorf("Resource.Kind = %q, want ValidatingWebhookConfiguration", f.Resource.Kind)
	}
	if f.Resource.Name != "broken-guard" {
		t.Errorf("Resource.Name = %q, want broken-guard", f.Resource.Name)
	}
	if len(f.Evidence) != 4 {
		t.Errorf("Evidence has %d entries, want 4: %v", len(f.Evidence), f.Evidence)
	}

	wantFingerprint := findings.Fingerprint("WH-002", "wh002-pos-webhook-uid/guard.example.com", "1.34")
	if f.Fingerprint != wantFingerprint {
		t.Errorf("Fingerprint = %q, want %q", f.Fingerprint, wantFingerprint)
	}
}

// TestWH002_Fingerprint_StableAcrossReorderAndUniquePerBlock guards the
// exact failure mode a reviewer flagged: fingerprints must not depend on a
// webhook block's position in .webhooks[] (so reordering doesn't silently
// mint a new fingerprint for an already-known failure and break waivers),
// and two distinct failing blocks in the same config must not collide onto
// the same fingerprint.
func TestWH002_Fingerprint_StableAcrossReorderAndUniquePerBlock(t *testing.T) {
	loadAndEvaluate := func(scenario string) []findings.Finding {
		dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh002", "reorder", scenario)
		objs, err := testutil.LoadFixtures(dir)
		if err != nil {
			t.Fatalf("loading %s fixtures: %v", scenario, err)
		}
		snap := testutil.BuildSnapshot(objs)
		fs, err := (WH002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate(%s): %v", scenario, err)
		}
		return fs
	}

	byWebhookName := func(fs []findings.Finding) map[string]findings.Finding {
		out := map[string]findings.Finding{}
		for _, f := range fs {
			for _, e := range f.Evidence {
				if name, ok := strings.CutPrefix(e, "webhook name: "); ok {
					out[name] = f
				}
			}
		}
		return out
	}

	orderA := loadAndEvaluate("order-a")
	orderB := loadAndEvaluate("order-b")

	if len(orderA) != 2 {
		t.Fatalf("order-a: got %d findings, want 2: %+v", len(orderA), orderA)
	}
	if len(orderB) != 2 {
		t.Fatalf("order-b: got %d findings, want 2: %+v", len(orderB), orderB)
	}

	// Two distinct blocks in the same config must not collide onto one
	// fingerprint.
	if orderA[0].Fingerprint == orderA[1].Fingerprint {
		t.Errorf("guard-a and guard-b findings share a fingerprint: %q", orderA[0].Fingerprint)
	}

	// guard-a is at index 0 in order-a and index 1 in order-b; guard-b is
	// the reverse. The fingerprint for each must be identical across both,
	// despite the index flip.
	a := byWebhookName(orderA)
	b := byWebhookName(orderB)
	for _, name := range []string{"guard-a.example.com", "guard-b.example.com"} {
		if a[name].Fingerprint != b[name].Fingerprint {
			t.Errorf("%s: fingerprint changed across reorder: order-a=%q order-b=%q", name, a[name].Fingerprint, b[name].Fingerprint)
		}
	}
}

func TestWH002_Negative_HealthyEndpointsNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh002", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (WH002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (healthy endpoints must not fire): %+v", len(fs), fs)
	}
}
