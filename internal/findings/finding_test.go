package findings

import "testing"

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
