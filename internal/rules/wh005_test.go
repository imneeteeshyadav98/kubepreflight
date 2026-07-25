package rules

import (
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func wh005RequireN(t *testing.T, fs []findings.Finding, n int) []findings.Finding {
	t.Helper()
	if len(fs) != n {
		t.Fatalf("got %d findings, want %d: %+v", len(fs), n, fs)
	}
	for _, f := range fs {
		if f.RuleID != "WH-005" {
			t.Errorf("RuleID = %q, want WH-005", f.RuleID)
		}
	}
	return fs
}

func TestWH005_NarrowScopeIsClean(t *testing.T) {
	fail := admissionregistrationv1.Fail
	snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh002Config(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
		})),
	}}
	fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a narrow-scope, default-timeout webhook", fs, err)
	}
}

func TestWH005_ExcessiveTimeoutSeconds(t *testing.T) {
	fail := admissionregistrationv1.Fail

	cases := []struct {
		name    string
		timeout *int32
		want    int
	}{
		{"unset", nil, 0},
		{"default 10s", int32Ptr(10), 0},
		{"24s just under threshold", int32Ptr(24), 0},
		{"25s at threshold", int32Ptr(25), 1},
		{"30s at API ceiling", int32Ptr(30), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wh := wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			})
			wh.TimeoutSeconds = c.timeout
			snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(wh)}}
			fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			wh005RequireN(t, fs, c.want)
			if c.want == 1 && fs[0].Severity != findings.SeverityWarning {
				t.Errorf("Severity = %q, want Warning", fs[0].Severity)
			}
		})
	}
}

func TestWH005_WildcardOperations(t *testing.T) {
	fail := admissionregistrationv1.Fail
	wh := admissionregistrationv1.ValidatingWebhook{
		Name:          "guard.example.com",
		FailurePolicy: &fail,
		Rules: []admissionregistrationv1.RuleWithOperations{
			{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}, Rule: admissionregistrationv1.Rule{APIGroups: []string{"apps"}, Resources: []string{"deployments"}}},
		},
		ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
	}
	snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(wh)}}
	fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh005RequireN(t, fs, 1)[0]
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
	if f.GlobalBlocker || f.CriticalInfra {
		t.Errorf("GlobalBlocker=%t CriticalInfra=%t, want both false for a narrow-resource wildcard-operations finding", f.GlobalBlocker, f.CriticalInfra)
	}
	if !strings.Contains(f.Message, "CONNECT") {
		t.Errorf("Message = %q, want it to mention CONNECT", f.Message)
	}
}

func TestWH005_SelfInterception(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	selfWebhook := func(fp *admissionregistrationv1.FailurePolicyType) admissionregistrationv1.ValidatingWebhook {
		return admissionregistrationv1.ValidatingWebhook{
			Name:          "guard.example.com",
			FailurePolicy: fp,
			Rules: []admissionregistrationv1.RuleWithOperations{
				{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update}, Rule: admissionregistrationv1.Rule{
					APIGroups: []string{"admissionregistration.k8s.io"}, Resources: []string{"validatingwebhookconfigurations"},
				}},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
		}
	}

	t.Run("Fail is a Warning with operator decision", func(t *testing.T) {
		snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(selfWebhook(&fail))}}
		fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh005RequireN(t, fs, 1)[0]
		if f.Severity != findings.SeverityWarning || f.UpgradeGate != findings.UpgradeGateOperatorDecision {
			t.Errorf("Severity/Gate = %q/%q, want Warning/operator_decision", f.Severity, f.UpgradeGate)
		}
		if f.GlobalBlocker {
			t.Error("GlobalBlocker = true, want false -- scope-only risk is not a confirmed admission failure")
		}
		if !strings.Contains(f.Message, "validatingwebhookconfigurations") {
			t.Errorf("Message = %q, want it to name the matched resource", f.Message)
		}
	})

	t.Run("Ignore is a Warning without GlobalBlocker", func(t *testing.T) {
		snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(selfWebhook(&ignore))}}
		fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh005RequireN(t, fs, 1)[0]
		if f.Severity != findings.SeverityWarning {
			t.Errorf("Severity = %q, want Warning", f.Severity)
		}
		if f.GlobalBlocker {
			t.Error("GlobalBlocker = true, want false -- failurePolicy Ignore doesn't reject any writes")
		}
	})
}

func TestWH005_HighRiskResourceScope(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	for _, resource := range []string{"nodes", "namespaces", "persistentvolumes"} {
		t.Run(resource, func(t *testing.T) {
			wh := admissionregistrationv1.ValidatingWebhook{
				Name:          "guard.example.com",
				FailurePolicy: &fail,
				Rules: []admissionregistrationv1.RuleWithOperations{
					{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update}, Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, Resources: []string{resource}}},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
			}
			snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(wh)}}
			fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			f := wh005RequireN(t, fs, 1)[0]
			if f.Severity != findings.SeverityWarning || f.UpgradeGate != findings.UpgradeGateOperatorDecision {
				t.Errorf("Severity/Gate = %q/%q, want Warning/operator_decision", f.Severity, f.UpgradeGate)
			}
			if f.CriticalInfra {
				t.Error("CriticalInfra = true, want false -- WH-005 scope-only risk must not escalate priority by itself")
			}
			if f.GlobalBlocker {
				t.Error("GlobalBlocker = true, want false -- this is a targeted resource, not a full catch-all")
			}
		})
	}

	t.Run("Ignore is a Warning without CriticalInfra", func(t *testing.T) {
		wh := admissionregistrationv1.ValidatingWebhook{
			Name:          "guard.example.com",
			FailurePolicy: &ignore,
			Rules: []admissionregistrationv1.RuleWithOperations{
				{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update}, Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, Resources: []string{"nodes"}}},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
		}
		snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(wh)}}
		fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh005RequireN(t, fs, 1)[0]
		if f.Severity != findings.SeverityWarning {
			t.Errorf("Severity = %q, want Warning", f.Severity)
		}
		if f.CriticalInfra {
			t.Error("CriticalInfra = true, want false")
		}
	})
}

// TestWH005_HighRiskResourceScope_MessageWordingMatchesFailurePolicy is a
// regression test for a real-cluster defect: the high-risk-resource-scope
// finding's message unconditionally claimed "fail-closed webhook becomes
// unavailable" even when the webhook's own Evidence recorded
// failurePolicy: Ignore -- a self-contradictory finding. The message must
// now branch on the real failurePolicy: fail-closed webhooks describe a
// direct dependency ("if it becomes unavailable, it can block..."),
// fail-open webhooks describe a configuration risk to review, and neither
// wording may appear for the wrong policy.
func TestWH005_HighRiskResourceScope_MessageWordingMatchesFailurePolicy(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	newWebhook := func(fp *admissionregistrationv1.FailurePolicyType) admissionregistrationv1.ValidatingWebhook {
		return admissionregistrationv1.ValidatingWebhook{
			Name:          "guard.example.com",
			FailurePolicy: fp,
			Rules: []admissionregistrationv1.RuleWithOperations{
				{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update}, Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, Resources: []string{"nodes"}}},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
		}
	}

	evaluate := func(t *testing.T, fp *admissionregistrationv1.FailurePolicyType) findings.Finding {
		t.Helper()
		snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(newWebhook(fp))}}
		fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		return wh005RequireN(t, fs, 1)[0]
	}

	t.Run("fail-closed message accurately describes a direct dependency", func(t *testing.T) {
		f := evaluate(t, &fail)
		if f.Severity != findings.SeverityWarning {
			t.Errorf("Severity = %q, want Warning (unchanged)", f.Severity)
		}
		if f.UpgradeGate != findings.UpgradeGateOperatorDecision {
			t.Errorf("UpgradeGate = %q, want operator_decision (unchanged)", f.UpgradeGate)
		}
		if !strings.Contains(f.Message, "fail-closed") {
			t.Errorf("Message = %q, want it to describe the webhook as fail-closed", f.Message)
		}
		if !strings.Contains(f.Message, "block node status updates") {
			t.Errorf("Message = %q, want it to describe the direct operational risk", f.Message)
		}
	})

	t.Run("fail-open message does not claim fail-closed or a blocker", func(t *testing.T) {
		f := evaluate(t, &ignore)
		if f.Severity != findings.SeverityWarning {
			t.Errorf("Severity = %q, want Warning (unchanged)", f.Severity)
		}
		if f.UpgradeGate != findings.UpgradeGateOperatorDecision {
			t.Errorf("UpgradeGate = %q, want operator_decision (unchanged)", f.UpgradeGate)
		}
		if strings.Contains(f.Message, "fail-closed") {
			t.Errorf("Message = %q, must not claim fail-closed for a failurePolicy: Ignore webhook", f.Message)
		}
		if strings.Contains(f.Message, "block node status updates") {
			t.Errorf("Message = %q, must not claim a blocking failure for a fail-open webhook", f.Message)
		}
		if !strings.Contains(f.Message, "fail-open") {
			t.Errorf("Message = %q, want it to describe the webhook as fail-open", f.Message)
		}
		// The finding's own Evidence records failurePolicy: Ignore -- the
		// message must not contradict it by claiming fail-closed behavior.
		var sawIgnoreEvidence bool
		for _, e := range f.Evidence {
			if e == "failurePolicy: Ignore" {
				sawIgnoreEvidence = true
			}
		}
		if !sawIgnoreEvidence {
			t.Fatalf("Evidence = %+v, want a \"failurePolicy: Ignore\" entry", f.Evidence)
		}
	})

	t.Run("only Message differs between policies, not structural fields", func(t *testing.T) {
		fClosed := evaluate(t, &fail)
		fOpen := evaluate(t, &ignore)
		if fClosed.RuleID != fOpen.RuleID {
			t.Errorf("RuleID differs: %q vs %q", fClosed.RuleID, fOpen.RuleID)
		}
		if fClosed.UpgradeGate != fOpen.UpgradeGate {
			t.Errorf("UpgradeGate differs: %q vs %q", fClosed.UpgradeGate, fOpen.UpgradeGate)
		}
		if fClosed.Severity != fOpen.Severity {
			t.Errorf("Severity differs: %q vs %q", fClosed.Severity, fOpen.Severity)
		}
		if fClosed.Fingerprint != fOpen.Fingerprint {
			t.Errorf("Fingerprint differs between failurePolicy values (%q vs %q); message text and failurePolicy must not affect the fingerprint shape used for occurrence tracking", fClosed.Fingerprint, fOpen.Fingerprint)
		}
		if fClosed.Message == fOpen.Message {
			t.Errorf("Message identical between fail-closed and fail-open cases, want distinct wording: %q", fClosed.Message)
		}
	})
}

// TestWH005_SelfInterception_MessageWordingMatchesFailurePolicy applies the
// same fail-open/fail-closed wording distinction to the self-interception
// finding, for consistency with the high-risk-resource-scope fix above.
func TestWH005_SelfInterception_MessageWordingMatchesFailurePolicy(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	selfWebhook := func(fp *admissionregistrationv1.FailurePolicyType) admissionregistrationv1.ValidatingWebhook {
		return admissionregistrationv1.ValidatingWebhook{
			Name:          "guard.example.com",
			FailurePolicy: fp,
			Rules: []admissionregistrationv1.RuleWithOperations{
				{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update}, Rule: admissionregistrationv1.Rule{
					APIGroups: []string{"admissionregistration.k8s.io"}, Resources: []string{"validatingwebhookconfigurations"},
				}},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
		}
	}

	t.Run("fail-closed message describes fail-closed behavior", func(t *testing.T) {
		snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(selfWebhook(&fail))}}
		fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh005RequireN(t, fs, 1)[0]
		if !strings.Contains(f.Message, "fail-closed") {
			t.Errorf("Message = %q, want it to describe the webhook as fail-closed", f.Message)
		}
	})

	t.Run("fail-open message does not claim fail-closed", func(t *testing.T) {
		snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(selfWebhook(&ignore))}}
		fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh005RequireN(t, fs, 1)[0]
		if strings.Contains(f.Message, "fail-closed") {
			t.Errorf("Message = %q, must not claim fail-closed for a failurePolicy: Ignore webhook", f.Message)
		}
		if !strings.Contains(f.Message, "fail-open") {
			t.Errorf("Message = %q, want it to describe the webhook as fail-open", f.Message)
		}
		var sawIgnoreEvidence bool
		for _, e := range f.Evidence {
			if e == "failurePolicy: Ignore" {
				sawIgnoreEvidence = true
			}
		}
		if !sawIgnoreEvidence {
			t.Fatalf("Evidence = %+v, want a \"failurePolicy: Ignore\" entry", f.Evidence)
		}
	})
}

func TestWH005_MutatingWebhookConfigsAlsoEvaluated(t *testing.T) {
	fail := admissionregistrationv1.Fail
	mwh := admissionregistrationv1.MutatingWebhook{
		Name:          "guard.example.com",
		FailurePolicy: &fail,
		Rules: []admissionregistrationv1.RuleWithOperations{
			{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update}, Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, Resources: []string{"nodes"}}},
		},
		ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
	}
	snap := &k8s.Snapshot{MutatingWebhookConfigs: []admissionregistrationv1.MutatingWebhookConfiguration{{
		ObjectMeta: metav1.ObjectMeta{Name: "guard-mutating", UID: "uid-guard-mutating"},
		Webhooks:   []admissionregistrationv1.MutatingWebhook{mwh},
	}}}
	fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh005RequireN(t, fs, 1)[0]
	if !strings.Contains(f.Message, "MutatingWebhookConfiguration") {
		t.Errorf("Message = %q, want it to reference MutatingWebhookConfiguration", f.Message)
	}
}

func TestWH005_CatchAllScope_FiresAllThreeScopeChecks(t *testing.T) {
	fail := admissionregistrationv1.Fail
	wh := admissionregistrationv1.ValidatingWebhook{
		Name:          "catchall.example.com",
		FailurePolicy: &fail,
		Rules: []admissionregistrationv1.RuleWithOperations{
			{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}, Rule: admissionregistrationv1.Rule{APIGroups: []string{"*"}, Resources: []string{"*"}}},
		},
		ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"}},
	}
	snap := &k8s.Snapshot{ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{wh002Config(wh)}}
	fs, err := (WH005{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	fs = wh005RequireN(t, fs, 3)

	seen := map[string]bool{}
	fingerprints := map[string]bool{}
	for _, f := range fs {
		seen[string(f.Severity)+":"+boolLabel(f.GlobalBlocker)+":"+boolLabel(f.CriticalInfra)] = true
		if fingerprints[f.Fingerprint] {
			t.Errorf("duplicate fingerprint %q across findings on the same webhook", f.Fingerprint)
		}
		fingerprints[f.Fingerprint] = true
	}
	want := []string{
		string(findings.SeverityWarning) + ":false:false", // wildcard operations
		string(findings.SeverityWarning) + ":false:false", // self-interception scope-only risk
		string(findings.SeverityWarning) + ":false:false", // high-risk-resource-scope
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing expected finding shape %q among: %+v", w, fs)
		}
	}
}

func int32Ptr(v int32) *int32 { return &v }

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
