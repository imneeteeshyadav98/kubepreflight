package cli

import (
	"path/filepath"
	"testing"

	"kubepreflight/internal/rollback"
)

func TestRollbackCommandHasPlanAndAssess(t *testing.T) {
	exitCode := 0
	cmd := newRollbackCmd(&exitCode)

	if cmd.Name() != "rollback" {
		t.Fatalf("Name = %q, want rollback", cmd.Name())
	}
	for _, name := range []string{"plan", "assess"} {
		sub, _, err := cmd.Find([]string{name, "--help"})
		if err != nil {
			t.Fatalf("Find(%s): %v", name, err)
		}
		if sub == nil || sub.Name() != name {
			t.Fatalf("Find(%s) = %v", name, sub)
		}
		for _, flag := range []string{"provider", "cluster-name", "output", "assessment-out", "findings", "terminal-output", "collector-timeout"} {
			if sub.Flags().Lookup(flag) == nil {
				t.Fatalf("rollback %s missing --%s flag", name, flag)
			}
		}
	}
}

func TestRollbackReportTargetsAlwaysIncludeAssessmentJSON(t *testing.T) {
	targets := rollbackReportTargets("all", "out", "custom.json")
	got := targetPaths(targets)
	want := []string{
		filepath.Join("out", "custom.json"),
		filepath.Join("out", "rollback-report.md"),
		filepath.Join("out", "rollback-report.html"),
	}
	if len(got) != len(want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targets = %v, want %v", got, want)
		}
	}
}

func TestRollbackExitCodeMapping(t *testing.T) {
	tests := []struct {
		decision rollback.RecommendationDecision
		want     int
	}{
		{rollback.RecommendationRollbackPreferred, 0},
		{rollback.RecommendationFixForwardPreferred, 1},
		{rollback.RecommendationOperatorDecisionRequired, 1},
		{rollback.RecommendationDoNotProceed, 2},
	}
	for _, tc := range tests {
		got := rollbackExitCode(rollback.Assessment{
			Recommendation: rollback.Recommendation{Decision: tc.decision},
		})
		if got != tc.want {
			t.Fatalf("rollbackExitCode(%q) = %d, want %d", tc.decision, got, tc.want)
		}
	}
}

func targetPaths(targets []rollbackReportTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.path)
	}
	return out
}
