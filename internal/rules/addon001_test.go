package rules

import (
	"testing"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

func TestADDON001_Positive_IncompatibleVersion(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.15.0-eksbuild.1", CompatibleVersions: []string{"v1.18.0-eksbuild.1", "v1.18.1-eksbuild.1"}},
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "ADDON-001" {
		t.Errorf("RuleID = %q, want ADDON-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierProviderReported {
		t.Errorf("Confidence = %q, want PROVIDER_REPORTED", f.Confidence)
	}
	if f.Resources[0].Name != "vpc-cni" {
		t.Errorf("resource name = %q, want vpc-cni", f.Resources[0].Name)
	}
}

func TestADDON001_Positive_NoCompatibleVersionAtAll(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "legacy-addon", CurrentVersion: "v0.1.0", CompatibleVersions: nil},
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
}

func TestADDON001_Negative_CompatibleVersionNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.18.1-eksbuild.1", CompatibleVersions: []string{"v1.18.0-eksbuild.1", "v1.18.1-eksbuild.1"}},
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (current version is in the compatible list): %+v", len(fs), fs)
	}
}

func TestADDON001_Negative_NilAWSSnapshotNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{AWS: nil}
	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when AWS enrichment was skipped: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when sc.AWS is nil: %+v", len(fs), fs)
	}
}
