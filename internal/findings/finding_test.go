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
