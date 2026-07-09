package rules

import (
	"strings"
	"testing"
	"time"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

func TestEKSINSIGHT001_ErrorInsightCreatesWarning(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Insights: []awscol.InsightRecord{{
		ClusterName:       "prod",
		ID:                "insight-error",
		Name:              "Deprecated API usage",
		Category:          "UPGRADE_READINESS",
		KubernetesVersion: "1.34",
		Status:            "ERROR",
		Recommendation:    "Migrate off deprecated APIs",
		LastRefreshTime:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		DeprecationDetails: []string{
			"usage: policy/v1beta1/podsecuritypolicies; stopServingVersion: 1.25",
		},
	}}}}

	fs, err := EKSINSIGHT001{}.Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	f := fs[0]
	if f.RuleID != "EKS-INSIGHT-001" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierProviderReported {
		t.Fatalf("finding = %+v, unexpected classification", f)
	}
	if strings.Contains(string(f.Severity), "Blocker") {
		t.Fatalf("ERROR insights must not become blockers in this PR: %+v", f)
	}
	if !strings.Contains(strings.Join(f.Evidence, "\n"), "deprecation detail") {
		t.Errorf("evidence missing deprecation details: %+v", f.Evidence)
	}
}

func TestEKSINSIGHT002_WarningInsightCreatesWarning(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Insights: []awscol.InsightRecord{{
		ID: "insight-warning", Name: "Add-on compatibility", Category: "UPGRADE_READINESS", Status: "WARNING", KubernetesVersion: "1.34",
	}}}}

	fs, err := EKSINSIGHT002{}.Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityWarning {
		t.Fatalf("findings = %+v, want one warning", fs)
	}
}

func TestEKSINSIGHT003_UnknownInsightCreatesInfo(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Insights: []awscol.InsightRecord{{
		ID: "insight-unknown", Name: "Unknown provider state", Category: "UPGRADE_READINESS", Status: "UNKNOWN", KubernetesVersion: "1.34",
	}}}}

	fs, err := EKSINSIGHT003{}.Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityInfo {
		t.Fatalf("findings = %+v, want one info", fs)
	}
}

func TestEKSINSIGHT_PassingInsightCreatesNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Insights: []awscol.InsightRecord{{
		ID: "insight-passing", Name: "Clean check", Category: "UPGRADE_READINESS", Status: "PASSING", KubernetesVersion: "1.34",
	}}}}
	for _, rule := range []rulesRule{EKSINSIGHT001{}, EKSINSIGHT002{}, EKSINSIGHT003{}} {
		fs, err := rule.Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("%s Evaluate: %v", rule.ID(), err)
		}
		if len(fs) != 0 {
			t.Fatalf("%s findings = %+v, want none for PASSING", rule.ID(), fs)
		}
	}
}

type rulesRule interface {
	ID() string
	Evaluate(*ScanContext, string) ([]findings.Finding, error)
}
