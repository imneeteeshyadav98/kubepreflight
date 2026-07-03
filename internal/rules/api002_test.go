package rules

import (
	"strings"
	"testing"
	"time"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// AWS SDK responses don't have an established YAML-fixture convention the
// way Kubernetes objects do (via testutil.LoadFixtures); hand-built
// awscol.Snapshot literals are the idiomatic equivalent here since the
// collector itself is already tested against mocked SDK calls
// (internal/collectors/aws/collector_test.go).

func TestAPI002_Positive_ErrorInsightIsBlockerWithStalenessCaveat(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Insights: []awscol.InsightRecord{
			{
				ID: "insight-1", Name: "Deprecated API usage", Category: "UPGRADE_READINESS",
				KubernetesVersion: "1.34", Status: "ERROR", Reason: "PodSecurityPolicy in use",
				Recommendation:  "Migrate off PodSecurityPolicy before upgrading",
				LastRefreshTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}}

	fs, err := (API002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "API-002" {
		t.Errorf("RuleID = %q, want API-002", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker (ERROR status)", f.Severity)
	}
	if f.Confidence != findings.TierProviderReported {
		t.Errorf("Confidence = %q, want PROVIDER_REPORTED", f.Confidence)
	}

	foundCaveat := false
	for _, e := range f.Evidence {
		if strings.Contains(e, "30-day audit-log lookback") {
			foundCaveat = true
		}
	}
	if !foundCaveat {
		t.Errorf("evidence must explicitly state the 30-day audit-window staleness caveat, got: %v", f.Evidence)
	}
	if !strings.Contains(f.Remediation, "30-day audit-log lookback") {
		t.Errorf("remediation must also carry the staleness caveat, got: %q", f.Remediation)
	}
}

func TestAPI002_Positive_WarningInsightIsWarningSeverity(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Insights: []awscol.InsightRecord{
			{ID: "insight-2", Name: "Minor concern", Category: "UPGRADE_READINESS", Status: "WARNING", KubernetesVersion: "1.34"},
		},
	}}

	fs, err := (API002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityWarning {
		t.Fatalf("got %+v, want a single Warning finding", fs)
	}
}

func TestAPI002_Negative_NilAWSSnapshotNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{AWS: nil}
	fs, err := (API002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when AWS enrichment was skipped: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when sc.AWS is nil: %+v", len(fs), fs)
	}
}

func TestAPI002_Negative_NoInsightsNoFindings(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{}}
	fs, err := (API002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 with an empty Insights list: %+v", len(fs), fs)
	}
}
