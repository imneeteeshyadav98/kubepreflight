package rules

import (
	"testing"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

func TestNODE002_Positive_LowIPHeadroom(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Subnets: []awscol.SubnetRecord{
			{ID: "subnet-a", AvailableIPAddressCount: 2},
		},
	}}

	fs, err := (NODE002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "NODE-002" {
		t.Errorf("RuleID = %q, want NODE-002", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resources[0].Name != "subnet-a" {
		t.Errorf("resource name = %q, want subnet-a", f.Resources[0].Name)
	}
}

func TestNODE002_Negative_SufficientHeadroomNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Subnets: []awscol.SubnetRecord{
			{ID: "subnet-b", AvailableIPAddressCount: 200},
		},
	}}

	fs, err := (NODE002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (plenty of headroom): %+v", len(fs), fs)
	}
}

func TestNODE002_Negative_NilAWSSnapshotNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{AWS: nil}
	fs, err := (NODE002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when AWS enrichment was skipped: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when sc.AWS is nil: %+v", len(fs), fs)
	}
}
