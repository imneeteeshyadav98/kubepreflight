package findings

import (
	"testing"
	"time"
)

func TestReportExitCodeContract(t *testing.T) {
	ref := LiveResource("Node", ScopeCluster, "", "node-a", "uid-node-a")
	for _, tc := range []struct {
		name string
		fs   []Finding
		want int
	}{
		{name: "clean", want: 0},
		{name: "warnings only", fs: []Finding{{Severity: SeverityWarning, Resources: []ResourceReference{ref}}}, want: 1},
		{name: "blocker", fs: []Finding{{Severity: SeverityBlocker, Resources: []ResourceReference{ref}}}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rpt := NewReport("1.36", "test", "", time.Time{}, tc.fs)
			if got := rpt.ExitCode(); got != tc.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReportIncompleteCoverageHasDistinctResultAndExitCode(t *testing.T) {
	r := NewReport("1.34", "cluster", "", time.Now(), nil)
	r.Coverage.Kubernetes = PlaneCoverage{Status: CoveragePartial, Errors: []string{"pods: forbidden"}}
	if got := r.Result(); got != "INCOMPLETE" {
		t.Fatalf("Result() = %q", got)
	}
	if got := r.ExitCode(); got != 3 {
		t.Fatalf("ExitCode() = %d, want 3", got)
	}
}

func TestBuildAPICompatibilitySummary_CleanPassed(t *testing.T) {
	got := BuildAPICompatibilitySummary(nil)
	if got == nil {
		t.Fatal("BuildAPICompatibilitySummary(nil) returned nil")
	}
	if got.Status != "Passed" || !got.UpgradeContinue || got.ScoreImpact != 0 {
		t.Fatalf("summary = %+v, want Passed/continue/zero impact", got)
	}
}

func TestBuildAPICompatibilitySummary_RemovedAPIsDedupFamiliesAndCapsScore(t *testing.T) {
	fs := []Finding{
		apiCompatibilityFinding("API-001", SeverityBlocker, "policy/v1beta1", "PodSecurityPolicy", ScopeCluster, "", "restricted"),
		apiCompatibilityFinding("API-001", SeverityBlocker, "policy/v1beta1", "PodSecurityPolicy", ScopeCluster, "", "baseline"),
		apiCompatibilityFinding("API-001", SeverityBlocker, "extensions/v1beta1", "Ingress", ScopeNamespaced, "ingress-nginx", "edge"),
		apiCompatibilityFinding("API-001", SeverityBlocker, "apps/v1beta1", "Deployment", ScopeNamespaced, "apps", "api"),
		apiCompatibilityFinding("API-001", SeverityBlocker, "batch/v1beta1", "CronJob", ScopeNamespaced, "jobs", "cleanup"),
		apiCompatibilityFinding("API-002", SeverityWarning, "policy/v1beta1", "PodDisruptionBudget", ScopeNamespaced, "apps", "api-pdb"),
	}

	got := BuildAPICompatibilitySummary(fs)

	if got.Status != "Failed" || got.UpgradeContinue {
		t.Fatalf("summary = %+v, want failed and upgradeContinue=false", got)
	}
	if got.RemovedObjects != 5 || got.DeprecatedObjects != 1 {
		t.Fatalf("counts = removed %d deprecated %d, want 5/1", got.RemovedObjects, got.DeprecatedObjects)
	}
	if len(got.RemovedFamilies) != 4 {
		t.Fatalf("removed families = %+v, want 4 unique API version/kind families", got.RemovedFamilies)
	}
	if !got.CriticalImpact {
		t.Fatalf("CriticalImpact = false, want true for cluster-scoped removed API")
	}
	if got.ScoreImpact != -60 {
		t.Fatalf("ScoreImpact = %d, want capped -60", got.ScoreImpact)
	}
	psp := got.RemovedFamilies[3]
	if psp.APIVersion != "policy/v1beta1" || psp.Kind != "PodSecurityPolicy" || psp.Count != 2 {
		t.Fatalf("deduped PSP family = %+v, want policy/v1beta1 PodSecurityPolicy count 2", psp)
	}
}

func TestNewReportIncludesAPICompatibilitySummary(t *testing.T) {
	r := NewReport("1.36", "prod", "", time.Now(), []Finding{
		apiCompatibilityFinding("API-001", SeverityBlocker, "policy/v1beta1", "PodSecurityPolicy", ScopeCluster, "", "restricted"),
	})
	if r.APICompatibility == nil {
		t.Fatal("NewReport did not populate APICompatibility")
	}
	if r.APICompatibility.Status != "Failed" || r.APICompatibility.RemovedObjects != 1 {
		t.Fatalf("APICompatibility = %+v, want failed with one removed object", r.APICompatibility)
	}
}

func apiCompatibilityFinding(ruleID string, severity Severity, apiVersion, kind string, scope ResourceScope, namespace, name string) Finding {
	return Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Confidence:  TierStaticCertain,
		Message:     "api compatibility finding",
		Resources:   []ResourceReference{ManifestResource(kind, scope, namespace, name, "manifest.yaml")},
		Evidence:    []string{"apiVersion: " + apiVersion},
		Fingerprint: FingerprintV2(ruleID, "1.36", "", ManifestResource(kind, scope, namespace, name, "manifest.yaml")),
	}
}

func TestNormalizeKubernetesVersion(t *testing.T) {
	got, ok := NormalizeKubernetesVersion("v1.29.6-eks-1234567")
	if !ok || got != "1.29" {
		t.Fatalf("NormalizeKubernetesVersion() = %q, %v; want 1.29, true", got, ok)
	}
	if _, ok := NormalizeKubernetesVersion(""); ok {
		t.Fatal("NormalizeKubernetesVersion(empty) succeeded, want false")
	}
}

func TestUpgradePath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		from  string
		to    string
		path  []string
		label string
	}{
		{name: "one minor", from: "1.29", to: "1.30", path: []string{"1.29", "1.30"}, label: "one-minor upgrade"},
		{name: "multi minor", from: "1.32", to: "1.36", path: []string{"1.32", "1.33", "1.34", "1.35", "1.36"}, label: "multi-minor upgrade path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, label, ok := UpgradePath(tc.from, tc.to)
			if !ok {
				t.Fatal("UpgradePath ok = false, want true")
			}
			if label != tc.label {
				t.Fatalf("label = %q, want %q", label, tc.label)
			}
			if len(path) != len(tc.path) {
				t.Fatalf("path length = %d, want %d: %v", len(path), len(tc.path), path)
			}
			for i := range path {
				if path[i] != tc.path[i] {
					t.Fatalf("path[%d] = %q, want %q (full path %v)", i, path[i], tc.path[i], path)
				}
			}
		})
	}
}

// TestResultAndExitCodeShareOnePriorityOrder guards the exact regression
// found in review: Result() and ExitCode() must always agree, in
// particular when a scan has BOTH real blockers AND incomplete coverage —
// incomplete coverage must outrank the blocker count for both, not just
// one of the two functions.
func TestResultAndExitCodeShareOnePriorityOrder(t *testing.T) {
	blockerFinding := Finding{
		RuleID: "TEST-001", Severity: SeverityBlocker, Confidence: TierStaticCertain,
		Resources:   []ResourceReference{LiveResource("Node", ScopeCluster, "", "node-a", "uid-node-a")},
		Fingerprint: "fp-blocker",
	}

	tests := []struct {
		name         string
		findings     []Finding
		partialK8s   bool
		wantResult   string
		wantExitCode int
	}{
		{
			name:         "complete scan with blocker",
			findings:     []Finding{blockerFinding},
			wantResult:   "BLOCKED",
			wantExitCode: 2,
		},
		{
			name:         "incomplete scan with no blocker",
			partialK8s:   true,
			wantResult:   "INCOMPLETE",
			wantExitCode: 3,
		},
		{
			name:         "incomplete scan with blocker — incomplete must still win",
			findings:     []Finding{blockerFinding},
			partialK8s:   true,
			wantResult:   "INCOMPLETE",
			wantExitCode: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReport("1.36", "cluster", "", time.Now(), tc.findings)
			if tc.partialK8s {
				r.Coverage.Kubernetes = PlaneCoverage{Status: CoveragePartial, Errors: []string{"pods: forbidden"}}
			}
			if got := r.Result(); got != tc.wantResult {
				t.Errorf("Result() = %q, want %q", got, tc.wantResult)
			}
			if got := r.ExitCode(); got != tc.wantExitCode {
				t.Errorf("ExitCode() = %d, want %d", got, tc.wantExitCode)
			}
			// The two must never diverge: assert the shape of the bug
			// directly, not just the expected values above.
			gotResult, gotCode := r.resultAndExitCode()
			if gotResult != r.Result() || gotCode != r.ExitCode() {
				t.Errorf("resultAndExitCode() = (%q, %d) diverges from Result()/ExitCode() = (%q, %d)", gotResult, gotCode, r.Result(), r.ExitCode())
			}
		})
	}
}
