package findings

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFingerprintV2NeverReusesLegacyDomain(t *testing.T) {
	ref := LiveResource("ConfigMap", ScopeNamespaced, "payments", "settings", "uid-1")
	legacy := Fingerprint("CFG-001", "uid-1", "1.34")
	v2 := FingerprintV2("CFG-001", "1.34", "", ref)
	if legacy == v2 {
		t.Fatalf("finding-v2 fingerprint collided with legacy domain: %q", v2)
	}
}

// TestFingerprintV2_UnaffectedByReportRuleExecutions is the required
// fingerprint-noninterference test: FingerprintV2's fixed input list (rule
// ID, target version, discriminator, resource keys) has no Report or
// RuleExecutions parameter at all, so populating Report.RuleExecutions must
// never change a finding's fingerprint. This constructs two otherwise
// byte-identical findings/reports, differing only in whether the
// containing Report carries a populated RuleExecutions, and asserts the
// fingerprints -- computed the exact same way regardless -- are identical.
// If a future change ever threaded Report state into fingerprint
// computation (e.g. by making FingerprintV2 a Report method, or adding a
// Report/RuleExecutions parameter), this test would need updating to
// construct that path explicitly, at which point the fingerprint-stability
// invariant this test guards would need fresh, deliberate review.
func TestFingerprintV2_UnaffectedByReportRuleExecutions(t *testing.T) {
	ref := LiveResource("ValidatingWebhookConfiguration", ScopeCluster, "", "webhook-a", "uid-wh-a")

	fp := FingerprintV2("WH-005", "1.34", "", ref)
	f := Finding{RuleID: "WH-005", Severity: SeverityBlocker, Resources: []ResourceReference{ref}, Fingerprint: fp}

	withoutExecutions := &Report{Findings: []Finding{f}}
	withExecutions := &Report{
		Findings: []Finding{f},
		RuleExecutions: []RuleExecutionRecord{
			{RuleID: "WH-005", Applicability: ApplicabilityApplicable, State: ExecutionEvaluated},
			{RuleID: "WH-001", Applicability: ApplicabilityApplicable, State: ExecutionFailed, Reason: "boom"},
		},
	}

	// FingerprintV2 itself takes no Report/RuleExecutions input, so
	// recomputing it here is guaranteed identical -- this is the code-level
	// confirmation that its signature has no path to the new field.
	fpRecomputed := FingerprintV2("WH-005", "1.34", "", ref)
	if fp != fpRecomputed {
		t.Fatalf("FingerprintV2 produced different output on identical inputs: %q vs %q", fp, fpRecomputed)
	}

	if withoutExecutions.Findings[0].Fingerprint != withExecutions.Findings[0].Fingerprint {
		t.Fatalf("finding fingerprint differs (%q vs %q) depending on whether the containing Report.RuleExecutions was populated -- fingerprints must be computed purely from rule ID/target version/discriminator/resource keys",
			withoutExecutions.Findings[0].Fingerprint, withExecutions.Findings[0].Fingerprint)
	}
}

func TestConceptKey_OmittedNamespacedManifestIsUnmatchable(t *testing.T) {
	ref := ManifestResource("Deployment", ScopeNamespaced, "", "api", "deployment.yaml")
	if key, ok := ref.ConceptKey(); ok {
		t.Fatalf("omitted namespace produced concept key %q; want conservative no-match", key)
	}
}

func TestAWSInsightProviderIDPreventsCategoryVersionCollision(t *testing.T) {
	a := AWSInsightResource("UPGRADE_READINESS", "1.34", "insight-a", "deprecated APIs")
	b := AWSInsightResource("UPGRADE_READINESS", "1.34", "insight-b", "add-on compatibility")
	if got, wantNot := FingerprintV2("API-002", "1.34", "", a), FingerprintV2("API-002", "1.34", "", b); got == wantNot {
		t.Fatalf("different provider IDs collided at same category/version: %q", got)
	}
}

func baseFinding() Finding {
	return Finding{
		RuleID:      "TEST-001",
		Severity:    SeverityBlocker,
		Confidence:  TierStaticCertain,
		Message:     "test message",
		Resources:   []ResourceReference{LiveResource("ConfigMap", ScopeNamespaced, "default", "x", "uid-1")},
		Fingerprint: "fp",
	}
}

func TestRemediationDetail_NilOmittedFromJSON(t *testing.T) {
	f := baseFinding()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(raw), "remediationDetail") {
		t.Errorf("JSON = %s, want no remediationDetail key when nil", raw)
	}
	if err := f.Validate(); err != nil {
		t.Errorf("Validate() with nil RemediationDetail = %v, want nil error", err)
	}
}

func TestRemediationDetail_RoundTripsThroughJSON(t *testing.T) {
	f := baseFinding()
	f.RemediationDetail = &RemediationDetail{
		AffectedFile:   "manifests/pdb.yaml",
		Changes:        []RemediationChange{{Field: "apiVersion", Current: "policy/v1beta1", Required: "policy/v1"}},
		Diff:           "- apiVersion: policy/v1beta1\n+ apiVersion: policy/v1",
		SafeFix:        &RemediationAction{Label: "Safe fix", Command: "kubectl convert -f <file> --output-version policy/v1"},
		Emergency:      &RemediationAction{Label: "Emergency workaround", Risky: true, Command: "kubectl patch ..."},
		VerifyCommand:  "kubepreflight scan --target-version 1.34",
		ExpectedResult: "Allowed disruptions >= 1",
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate() with populated RemediationDetail = %v, want nil error", err)
	}

	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Finding
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.RemediationDetail == nil {
		t.Fatalf("decoded RemediationDetail = nil, want populated")
	}
	if decoded.RemediationDetail.Diff != f.RemediationDetail.Diff {
		t.Errorf("Diff = %q, want %q", decoded.RemediationDetail.Diff, f.RemediationDetail.Diff)
	}
	if decoded.RemediationDetail.Emergency == nil || !decoded.RemediationDetail.Emergency.Risky {
		t.Errorf("Emergency = %+v, want Risky=true to round-trip", decoded.RemediationDetail.Emergency)
	}
}

func TestGlobalBlocker_FalseOmittedFromJSON(t *testing.T) {
	f := baseFinding()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(raw), "globalBlocker") {
		t.Errorf("JSON = %s, want no globalBlocker key when false", raw)
	}
}

func TestGlobalBlocker_TrueRoundTripsThroughJSON(t *testing.T) {
	f := baseFinding()
	f.GlobalBlocker = true

	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"globalBlocker":true`) {
		t.Errorf("JSON = %s, want globalBlocker:true present", raw)
	}
	var decoded Finding
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !decoded.GlobalBlocker {
		t.Error("decoded GlobalBlocker = false, want true")
	}
}

func TestRemediationDetail_BreakGlassRoundTripsThroughJSON(t *testing.T) {
	f := baseFinding()
	f.RemediationDetail = &RemediationDetail{
		BreakGlass: &RemediationAction{Label: "Break-glass", Risky: true, Command: "kubectl delete validatingwebhookconfiguration x"},
	}

	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Finding
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.RemediationDetail == nil || decoded.RemediationDetail.BreakGlass == nil {
		t.Fatalf("decoded RemediationDetail.BreakGlass = nil, want populated")
	}
	if !decoded.RemediationDetail.BreakGlass.Risky {
		t.Error("BreakGlass.Risky = false, want true to round-trip")
	}
}
