package plan

import (
	"errors"
	"testing"
	"time"

	"kubepreflight/internal/findings"
)

func sampleHop1Report(t *testing.T, blockers int) *findings.Report {
	t.Helper()
	var fs []findings.Finding
	for i := 0; i < blockers; i++ {
		fs = append(fs, findings.Finding{
			RuleID:      "TEST-001",
			Severity:    findings.SeverityBlocker,
			Confidence:  findings.TierStaticCertain,
			Message:     "synthetic blocker",
			Resources:   []findings.ResourceReference{findings.LiveResource("Node", findings.ScopeCluster, "", "node-x", "uid-x")},
			Fingerprint: "fp-test",
		})
	}
	return findings.NewReport("1.30", "test-cluster", "", time.Now().UTC(), fs)
}

func TestBuildPlan_Hop1IsExactWithGivenReport(t *testing.T) {
	hops := []Hop{{Index: 1, From: "1.29", To: "1.30"}}
	hop1 := sampleHop1Report(t, 1)

	pr, err := BuildPlan("test-cluster", "eks", "1.29", "explicit-flag", "1.30", hops, hop1, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(pr.Hops) != 1 {
		t.Fatalf("len(Hops) = %d, want 1", len(pr.Hops))
	}
	if pr.Hops[0].Status != HopStatusExact {
		t.Errorf("Hops[0].Status = %v, want HopStatusExact", pr.Hops[0].Status)
	}
	if pr.Hops[0].Report != hop1 {
		t.Errorf("Hops[0].Report is not the exact hop1Report passed in")
	}
}

func TestBuildPlan_FutureHopsUseAssessCallback(t *testing.T) {
	hops := []Hop{
		{Index: 1, From: "1.29", To: "1.30"},
		{Index: 2, From: "1.30", To: "1.31"},
		{Index: 3, From: "1.31", To: "1.32"},
	}
	hop1 := sampleHop1Report(t, 0)

	var assessedHops []Hop
	assess := func(hop Hop) (HopReport, error) {
		assessedHops = append(assessedHops, hop)
		return HopReport{
			Hop:    hop,
			Status: HopStatusPredicted,
			CarryForward: []CarryForwardNote{
				{RuleID: "NODE-001", Reason: "nodes may be replaced before this hop is reached", RecommendedCommand: "kubepreflight scan --target-version " + hop.To},
			},
		}, nil
	}

	pr, err := BuildPlan("test-cluster", "", "1.29", "k8s-server-version", "1.32", hops, hop1, assess, time.Now().UTC())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(pr.Hops) != 3 {
		t.Fatalf("len(Hops) = %d, want 3", len(pr.Hops))
	}
	if len(assessedHops) != 2 {
		t.Fatalf("assessFutureHop called %d times, want 2 (hops[1:])", len(assessedHops))
	}
	if assessedHops[0] != hops[1] || assessedHops[1] != hops[2] {
		t.Errorf("assessFutureHop called with %+v, want %+v", assessedHops, hops[1:])
	}
	for i, hr := range pr.Hops[1:] {
		if hr.Status != HopStatusPredicted {
			t.Errorf("Hops[%d].Status = %v, want HopStatusPredicted", i+1, hr.Status)
		}
		if len(hr.CarryForward) != 1 || hr.CarryForward[0].RuleID != "NODE-001" {
			t.Errorf("Hops[%d].CarryForward = %+v, want one NODE-001 note", i+1, hr.CarryForward)
		}
	}
}

func TestBuildPlan_AssessErrorPropagates(t *testing.T) {
	hops := []Hop{
		{Index: 1, From: "1.29", To: "1.30"},
		{Index: 2, From: "1.30", To: "1.31"},
	}
	hop1 := sampleHop1Report(t, 0)
	wantErr := errors.New("aws call failed")
	assess := func(hop Hop) (HopReport, error) { return HopReport{}, wantErr }

	_, err := BuildPlan("test-cluster", "eks", "1.29", "eks-describe-cluster", "1.31", hops, hop1, assess, time.Now().UTC())
	if err == nil {
		t.Fatal("BuildPlan succeeded, want error propagated from assessFutureHop")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("BuildPlan error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestBuildPlan_RequiresHopsAndHop1Report(t *testing.T) {
	if _, err := BuildPlan("c", "", "1.29", "explicit-flag", "1.30", nil, sampleHop1Report(t, 0), nil, time.Now()); err == nil {
		t.Error("BuildPlan with no hops succeeded, want error")
	}
	hops := []Hop{{Index: 1, From: "1.29", To: "1.30"}}
	if _, err := BuildPlan("c", "", "1.29", "explicit-flag", "1.30", hops, nil, nil, time.Now()); err == nil {
		t.Error("BuildPlan with nil hop1Report succeeded, want error")
	}
}

func TestPlanReport_OverallExitCodeMirrorsHop1Only(t *testing.T) {
	hops := []Hop{
		{Index: 1, From: "1.29", To: "1.30"},
		{Index: 2, From: "1.30", To: "1.31"},
	}
	hop1 := sampleHop1Report(t, 1) // 1 blocker -> exit code 2
	assess := func(hop Hop) (HopReport, error) {
		// Future hop predicts a blocker too — must NOT affect exit code.
		return HopReport{Hop: hop, Status: HopStatusPredicted}, nil
	}

	pr, err := BuildPlan("c", "eks", "1.29", "eks-describe-cluster", "1.31", hops, hop1, assess, time.Now().UTC())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if got := pr.OverallExitCode(); got != 2 {
		t.Errorf("OverallExitCode() = %d, want 2 (hop1's blocker count only)", got)
	}
}

func TestPlanReport_FieldsRoundTrip(t *testing.T) {
	hops := []Hop{{Index: 1, From: "1.29", To: "1.30"}}
	hop1 := sampleHop1Report(t, 0)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	pr, err := BuildPlan("my-cluster", "eks", "1.29", "eks-describe-cluster", "1.30", hops, hop1, nil, now)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if pr.ClusterContext != "my-cluster" || pr.Provider != "eks" || pr.FromVersion != "1.29" ||
		pr.FromVersionSource != "eks-describe-cluster" || pr.ToVersion != "1.30" || !pr.GeneratedAt.Equal(now) {
		t.Errorf("PlanReport fields did not round-trip: %+v", pr)
	}
}
