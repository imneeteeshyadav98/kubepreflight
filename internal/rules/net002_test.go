package rules

import (
	"testing"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func TestNET002_Positive_MissingSecurityGroup(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		NetworkPreflightIssues: []awscol.NetworkPreflightIssue{
			{Kind: "SecurityGroup", ID: "sg-deleted"},
		},
	}}

	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "NET-002" {
		t.Errorf("RuleID = %q, want NET-002", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resources[0].Name != "sg-deleted" {
		t.Errorf("resource name = %q, want sg-deleted", f.Resources[0].Name)
	}
}

func TestNET002_Positive_MissingVpc(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		NetworkPreflightIssues: []awscol.NetworkPreflightIssue{
			{Kind: "Vpc", ID: "vpc-deleted"},
		},
	}}

	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].Resources[0].Name != "vpc-deleted" {
		t.Errorf("resource name = %q, want vpc-deleted", fs[0].Resources[0].Name)
	}
}

func TestNET002_Negative_NoIssuesNoFindings(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{}}

	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (no network preflight issues): %+v", len(fs), fs)
	}
}

func TestNET002_Negative_NilAWSSnapshotNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{AWS: nil}
	fs, err := (NET002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when AWS enrichment was skipped: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when sc.AWS is nil: %+v", len(fs), fs)
	}
}
