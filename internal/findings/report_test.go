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
