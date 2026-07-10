package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/collectors/k8s"
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
	if f.Confidence != findings.TierObserved {
		t.Errorf("Confidence = %q, want OBSERVED", f.Confidence)
	}
	if f.Resources[0].Kind != "ValidatingWebhookConfiguration" {
		t.Errorf("resource kind = %q, want ValidatingWebhookConfiguration", f.Resources[0].Kind)
	}
	if f.Resources[0].Name != "broken-guard" {
		t.Errorf("resource name = %q, want broken-guard", f.Resources[0].Name)
	}
	if len(f.Evidence) != 5 {
		t.Errorf("Evidence has %d entries, want 5: %v", len(f.Evidence), f.Evidence)
	}

	wantFingerprint := findings.FingerprintV2("WH-002", "1.34", "guard.example.com", f.Resources[0])
	if f.Fingerprint != wantFingerprint {
		t.Errorf("Fingerprint = %q, want %q", f.Fingerprint, wantFingerprint)
	}

	// This fixture's rule is scoped to apps/deployments, not catch-all —
	// it's a real availability blocker but not a GLOBAL one.
	if f.GlobalBlocker {
		t.Error("GlobalBlocker = true, want false (fixture's webhook scope is not catch-all)")
	}

	rd := f.RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated")
	}
	if len(rd.Changes) != 1 || rd.Changes[0].Field != "endpoint count" || rd.Changes[0].Current != "0" {
		t.Errorf("Changes = %+v, want endpoint count 0 -> >= 1", rd.Changes)
	}
	if rd.SafeFix == nil || !strings.Contains(rd.SafeFix.Command, "kubectl get svc 'broken-guard-svc' -n 'guard-ns'") {
		t.Errorf("SafeFix = %+v, want a command inventorying the backend service", rd.SafeFix)
	}
	if rd.Emergency == nil || !rd.Emergency.Risky || !strings.Contains(rd.Emergency.Command, `"op":"replace"`) {
		t.Errorf("Emergency = %+v, want a risky replace-op failurePolicy patch (fixture sets failurePolicy explicitly)", rd.Emergency)
	}
	if rd.BreakGlass == nil || !rd.BreakGlass.Risky || !strings.Contains(rd.BreakGlass.Command, "kubectl delete validatingwebhookconfiguration "+shellQuote("broken-guard")) {
		t.Errorf("BreakGlass = %+v, want a risky delete command", rd.BreakGlass)
	}
	if rd.VerifyCommand == "" || rd.ExpectedResult == "" {
		t.Error("VerifyCommand/ExpectedResult must be populated")
	}
}

// TestWH002_GlobalBlocker_TrueWhenCatchAllScopeAndZeroEndpoints guards the
// composite "global API write blocker" detection: a fail-closed webhook
// with catch-all scope (WH-001's condition) AND zero ready endpoints
// (WH-002's own condition) together mean this outage can block other
// remediation commands too, not just this webhook's own writes.
func TestWH002_GlobalBlocker_TrueWhenCatchAllScopeAndZeroEndpoints(t *testing.T) {
	fail := admissionregistrationv1.Fail
	snap := &k8s.Snapshot{
		ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "catch-all-guard", UID: "uid-catch-all"},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name:          "catchall.example.com",
						FailurePolicy: &fail,
						Rules: []admissionregistrationv1.RuleWithOperations{
							{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}, Rule: admissionregistrationv1.Rule{APIGroups: []string{"*"}, Resources: []string{"*"}}},
						},
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "catch-all-svc"},
						},
					},
				},
			},
		},
	}

	fs, err := (WH002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if !fs[0].GlobalBlocker {
		t.Error("GlobalBlocker = false, want true (catch-all scope + fail-closed + zero endpoints)")
	}
}

func TestWH002_GlobalBlocker_FalseWhenNamespaceScoped(t *testing.T) {
	fail := admissionregistrationv1.Fail
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{{
		ObjectMeta: metav1.ObjectMeta{Name: "scoped-guard", UID: "uid-scoped"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name: "scoped.example.com", FailurePolicy: &fail,
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"guarded": "true"}},
			Rules:             []admissionregistrationv1.RuleWithOperations{{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}, Rule: admissionregistrationv1.Rule{APIGroups: []string{"*"}, Resources: []string{"*"}}}},
			ClientConfig:      admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
		}},
	}}}
	fs, err := (WH002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v", fs, err)
	}
	if fs[0].GlobalBlocker {
		t.Fatal("namespace-scoped webhook must not be classified as a global blocker")
	}
}

func TestWH002_SkipsWhenEndpointEvidenceUnavailable(t *testing.T) {
	snap := &k8s.Snapshot{Errors: map[string]error{"endpointslices": fmt.Errorf("forbidden")}}
	fs, err := (WH002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no false blocker", fs, err)
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
