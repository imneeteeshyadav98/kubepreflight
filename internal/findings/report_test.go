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

func TestReport_UpgradeApplicable(t *testing.T) {
	cases := []struct {
		name           string
		currentVersion string
		targetVersion  string
		want           bool
	}{
		{name: "different minor versions", currentVersion: "1.31", targetVersion: "1.32", want: true},
		{name: "same major.minor, exact strings", currentVersion: "1.32", targetVersion: "1.32", want: false},
		{name: "same major.minor, different string forms", currentVersion: "v1.32.6-eks-1234567", targetVersion: "1.32", want: false},
		{name: "current version unknown (empty)", currentVersion: "", targetVersion: "1.32", want: true},
		{name: "current version unparseable", currentVersion: "not-a-version", targetVersion: "1.32", want: true},
		{name: "different major versions", currentVersion: "1.32", targetVersion: "2.0", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Report{CurrentVersion: tc.currentVersion, TargetVersion: tc.targetVersion}
			if got := r.UpgradeApplicable(); got != tc.want {
				t.Errorf("UpgradeApplicable() = %v, want %v (current=%q target=%q)", got, tc.want, tc.currentVersion, tc.targetVersion)
			}
		})
	}
}

func TestCompareMinorVersions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		target  string
		want    VersionRelation
		wantErr bool
	}{
		{name: "target minor greater is upgrade", current: "1.36", target: "1.37", want: VersionUpgrade},
		{name: "exact same strings", current: "1.36", target: "1.36", want: VersionSame},
		{name: "same major.minor, different string forms", current: "v1.36.2-eks-1234567", target: "1.36", want: VersionSame},
		{name: "target minor lower is downgrade", current: "1.36", target: "1.30", want: VersionDowngrade},
		{name: "target minor lower by one is still downgrade", current: "1.36.2", target: "1.35", want: VersionDowngrade},
		{name: "unparseable current errors", current: "not-a-version", target: "1.36", wantErr: true},
		{name: "unparseable target errors", current: "1.36", target: "not-a-version", wantErr: true},
		{name: "different major errors", current: "1.36", target: "2.0", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CompareMinorVersions(tc.current, tc.target)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("CompareMinorVersions(%q, %q) returned nil error, want an error", tc.current, tc.target)
				}
				return
			}
			if err != nil {
				t.Fatalf("CompareMinorVersions(%q, %q) returned error: %v", tc.current, tc.target, err)
			}
			if got != tc.want {
				t.Errorf("CompareMinorVersions(%q, %q) = %v, want %v", tc.current, tc.target, got, tc.want)
			}
		})
	}
}

func TestVersionRelation_String(t *testing.T) {
	cases := map[VersionRelation]string{
		VersionUpgrade:   "upgrade",
		VersionSame:      "same",
		VersionDowngrade: "downgrade",
	}
	for relation, want := range cases {
		if got := relation.String(); got != want {
			t.Errorf("VersionRelation(%d).String() = %q, want %q", relation, got, want)
		}
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

func readinessFinding(ruleID string, severity Severity) Finding {
	ref := LiveResource("Resource", ScopeCluster, "", ruleID+"-obj", "uid-"+ruleID)
	return Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Confidence:  TierStaticCertain,
		Message:     "readiness scorecard test finding",
		Resources:   []ResourceReference{ref},
		Fingerprint: FingerprintV2(ruleID, "1.36", "", ref),
	}
}

// TestBuildUpgradeReadinessSummary_PerCategoryStatus proves every one of
// the 9 scorecard categories independently: a Blocker-severity finding for
// one of that category's rule IDs marks only that category Failed, leaves
// every other category Passed, and Verdict/UpgradeContinue reflect the
// passed-in verdict/blocker state — not a second, independently-derived
// decision.
func TestBuildUpgradeReadinessSummary_PerCategoryStatus(t *testing.T) {
	cases := []struct {
		category string
		ruleID   string
	}{
		{"API Compatibility", "API-001"},
		{"API Compatibility", "API-002"},
		{"Extension APIs", "CRD-001"},
		{"Extension APIs", "CRD-002"},
		{"Extension APIs", "APISERVICE-001"},
		{"Admission Webhooks", "WH-001"},
		{"Admission Webhooks", "WH-002"},
		{"Disruption Safety", "PDB-001"},
		{"Disruption Safety", "PDB-002"},
		{"Node Readiness", "NODE-001"},
		{"Node Readiness", "NODE-002"},
		{"Node Readiness", "NODE-003"},
		{"Node Readiness", "NET-002"},
		{"Node Readiness", "EKS-NG-001"},
		{"Node Readiness", "EKS-NG-002"},
		{"Node Readiness", "EKS-NG-003"},
		{"Node Readiness", "EKS-NG-004"},
		{"Add-ons", "ADDON-001"},
		{"Add-ons", "ADDON-002"},
		{"CoreDNS", "COREDNS-001"},
		{"Workload Health", "WORKLOAD-001"},
		{"EKS Upgrade Insights", "EKS-INSIGHT-001"},
		{"EKS Upgrade Insights", "EKS-INSIGHT-002"},
		{"EKS Upgrade Insights", "EKS-INSIGHT-003"},
	}
	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			summary := BuildUpgradeReadinessSummary([]Finding{readinessFinding(tc.ruleID, SeverityBlocker)}, "BLOCKED")
			if summary.Verdict != "BLOCKED" {
				t.Errorf("Verdict = %q, want the passed-in verdict unchanged", summary.Verdict)
			}
			if summary.UpgradeContinue {
				t.Errorf("UpgradeContinue = true, want false with a Blocker finding present")
			}
			for _, cat := range summary.Categories {
				if cat.Name == tc.category {
					if cat.Status != "Failed" || cat.BlockerCount != 1 {
						t.Errorf("category %s = %+v, want Failed with 1 blocker", cat.Name, cat)
					}
					continue
				}
				if cat.Status != "Passed" || cat.BlockerCount != 0 || cat.WarningCount != 0 {
					t.Errorf("unrelated category %s = %+v, want untouched Passed", cat.Name, cat)
				}
			}
		})
	}
}

// TestBuildUpgradeReadinessSummary_WarningOnlyDoesNotBlock proves a
// Warning-severity-only category reports Warning (not Failed) and doesn't
// flip UpgradeContinue to false on its own.
func TestBuildUpgradeReadinessSummary_WarningOnlyDoesNotBlock(t *testing.T) {
	summary := BuildUpgradeReadinessSummary([]Finding{readinessFinding("COREDNS-001", SeverityWarning)}, "PASSED_WITH_WARNINGS")
	if !summary.UpgradeContinue {
		t.Errorf("UpgradeContinue = false, want true — no Blocker finding anywhere")
	}
	for _, cat := range summary.Categories {
		if cat.Name == "CoreDNS" {
			if cat.Status != "Warning" || cat.WarningCount != 1 || cat.BlockerCount != 0 {
				t.Errorf("CoreDNS category = %+v, want Warning with 1 warning, 0 blockers", cat)
			}
		}
	}
}

// TestBuildUpgradeReadinessSummary_NoFindingsIsCleanPassed mirrors
// TestBuildAPICompatibilitySummary_CleanPassed for the general scorecard.
func TestBuildUpgradeReadinessSummary_NoFindingsIsCleanPassed(t *testing.T) {
	summary := BuildUpgradeReadinessSummary(nil, "CLEAN")
	if summary.ReadinessScore != 100 || !summary.UpgradeContinue || summary.Verdict != "CLEAN" {
		t.Fatalf("summary = %+v, want score 100, continue true, verdict CLEAN", summary)
	}
	for _, cat := range summary.Categories {
		if cat.Status != "Passed" {
			t.Errorf("category %s = %q, want Passed with no findings", cat.Name, cat.Status)
		}
	}
	if len(summary.Categories) != 10 {
		t.Fatalf("got %d categories, want all 10 present even with zero findings", len(summary.Categories))
	}
}

// TestBuildUpgradeReadinessSummary_ScoreFormula pins the exact penalty
// math documented on upgradeReadinessCategoryPenalty: a single-blocker
// Failed category costs 15, an additional blocker in the same category
// costs 3 more each up to a cap of 25; a single-warning Warning category
// costs 5, additional warnings cost 1 more each up to a cap of 10.
func TestBuildUpgradeReadinessSummary_ScoreFormula(t *testing.T) {
	cases := []struct {
		name      string
		findings  []Finding
		wantScore int
	}{
		{
			name:      "one blocker in one category",
			findings:  []Finding{readinessFinding("WH-001", SeverityBlocker)},
			wantScore: 85, // 100 - 15
		},
		{
			name: "four blockers in one category, still under the cap",
			findings: []Finding{
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
			},
			wantScore: 76, // 100 - min(25, 15+3*(4-1)=24) = 100-24
		},
		{
			name: "five blockers in one category hits the -25 cap",
			findings: []Finding{
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
			},
			wantScore: 75, // 100 - min(25, 15+3*(5-1)=27) = 100-25
		},
		{
			name:      "one warning in one category",
			findings:  []Finding{readinessFinding("COREDNS-001", SeverityWarning)},
			wantScore: 95, // 100 - 5
		},
		{
			name: "two categories failed",
			findings: []Finding{
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("PDB-001", SeverityBlocker),
			},
			wantScore: 70, // 100 - 15 - 15
		},
		{
			name: "many failed categories floor at zero",
			findings: []Finding{
				readinessFinding("API-001", SeverityBlocker),
				readinessFinding("CRD-001", SeverityBlocker),
				readinessFinding("WH-001", SeverityBlocker),
				readinessFinding("PDB-001", SeverityBlocker),
				readinessFinding("DRAIN-001", SeverityBlocker),
				readinessFinding("NODE-001", SeverityBlocker),
				readinessFinding("ADDON-001", SeverityBlocker),
				readinessFinding("COREDNS-001", SeverityBlocker),
				readinessFinding("WORKLOAD-001", SeverityBlocker),
				readinessFinding("EKS-INSIGHT-001", SeverityBlocker),
			},
			wantScore: 0, // 10 categories * -15 = -150, floored to 0
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := BuildUpgradeReadinessSummary(tc.findings, "BLOCKED")
			if summary.ReadinessScore != tc.wantScore {
				t.Errorf("ReadinessScore = %d, want %d", summary.ReadinessScore, tc.wantScore)
			}
		})
	}
}

// TestNewReportIncludesUpgradeReadinessSummary proves the scorecard is
// wired into NewReport (not just unit-testable in isolation), and that its
// Verdict always equals the same Report's own Result() — the core "never
// disagree" guarantee.
func TestNewReportIncludesUpgradeReadinessSummary(t *testing.T) {
	r := NewReport("1.36", "prod", "", time.Now(), []Finding{
		readinessFinding("WH-002", SeverityBlocker),
	})
	if r.UpgradeReadiness == nil {
		t.Fatal("NewReport did not populate UpgradeReadiness")
	}
	if r.UpgradeReadiness.Verdict != r.Result() {
		t.Errorf("UpgradeReadiness.Verdict = %q, want it to equal Report.Result() = %q", r.UpgradeReadiness.Verdict, r.Result())
	}
	if r.UpgradeReadiness.UpgradeContinue {
		t.Error("UpgradeContinue = true, want false with a Blocker finding present")
	}
}

// TestUpgradeReadinessCategories_APICompatibilityAgreesWithDedicatedSummary
// empirically proves the claim in the design doc: applying the same
// generic Blocker/Warning logic to {API-001, API-002} naturally reproduces
// the exact same status the dedicated BuildAPICompatibilitySummary
// computes — not just "should agree by inspection."
func TestUpgradeReadinessCategories_APICompatibilityAgreesWithDedicatedSummary(t *testing.T) {
	fs := []Finding{
		apiCompatibilityFinding("API-001", SeverityBlocker, "policy/v1beta1", "PodSecurityPolicy", ScopeCluster, "", "restricted"),
		apiCompatibilityFinding("API-002", SeverityWarning, "policy/v1beta1", "PodDisruptionBudget", ScopeNamespaced, "apps", "api-pdb"),
	}
	r := NewReport("1.36", "prod", "", time.Now(), fs)

	var apiCategory *UpgradeReadinessCategory
	for i := range r.UpgradeReadiness.Categories {
		if r.UpgradeReadiness.Categories[i].Name == "API Compatibility" {
			apiCategory = &r.UpgradeReadiness.Categories[i]
		}
	}
	if apiCategory == nil {
		t.Fatal("no API Compatibility category in UpgradeReadiness.Categories")
	}
	if apiCategory.Status != r.APICompatibility.Status {
		t.Errorf("scorecard category status = %q, dedicated APICompatibility.Status = %q — must agree", apiCategory.Status, r.APICompatibility.Status)
	}
}

// TestSetCoverage_RefreshesUpgradeReadinessVerdict guards a real bug found
// while building the GitHub Action integration: NewReport builds
// UpgradeReadiness against a placeholder, always-complete Coverage, since
// real Coverage is only known once every collector has finished (after
// NewReport already had to run). Every scan.go/plan.go caller sets the real
// Coverage afterward — before this fix, via direct field assignment
// (rpt.Coverage = ...), which left UpgradeReadiness.Verdict silently stuck
// at whatever it would be for a fully complete scan, disagreeing with
// Report.Result()/ExitCode() the moment coverage turned out partial or
// incomplete. This is not a rare edge case: --provider=eks with missing IAM
// permissions is a documented, expected way to hit it (README: "the report
// is marked INCOMPLETE and exits 3").
func TestSetCoverage_RefreshesUpgradeReadinessVerdict(t *testing.T) {
	r := NewReport("1.34", "test", "", time.Now(), nil)
	if r.UpgradeReadiness.Verdict != "CLEAN" {
		t.Fatalf("sanity check failed: fresh NewReport() with no findings = %q, want CLEAN", r.UpgradeReadiness.Verdict)
	}

	r.SetCoverage(ScanCoverage{
		Kubernetes: PlaneCoverage{Status: CoveragePartial, Errors: []string{"connection refused"}},
		AWS:        PlaneCoverage{Status: CoverageSkipped},
		Manifests:  PlaneCoverage{Status: CoverageSkipped},
	})

	wantVerdict := r.Result()
	if wantVerdict != "INCOMPLETE" {
		t.Fatalf("sanity check failed: r.Result() after partial Coverage = %q, want INCOMPLETE", wantVerdict)
	}
	if r.UpgradeReadiness.Verdict != wantVerdict {
		t.Errorf("UpgradeReadiness.Verdict = %q after SetCoverage, want it to equal Report.Result() = %q", r.UpgradeReadiness.Verdict, wantVerdict)
	}
	if r.UpgradeReadiness.UpgradeContinue {
		t.Error("UpgradeReadiness.UpgradeContinue = true for an INCOMPLETE scan, want false — partial evidence must never look safe to continue")
	}
}
