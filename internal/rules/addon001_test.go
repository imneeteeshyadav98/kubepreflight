package rules

import (
	"fmt"
	"strings"
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
	if !contains(f.Evidence, "compatibility status: incompatible") {
		t.Errorf("evidence = %v, want structured compatibility status", f.Evidence)
	}
	if !containsPrefix(f.Evidence, "required upgrade order: 1. Amazon VPC CNI") {
		t.Errorf("evidence = %v, want VPC CNI upgrade order", f.Evidence)
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

func TestADDON002_Positive_CriticalAddonCompatibilityUnknown(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.15.0-eksbuild.1", ClusterName: "prod"},
			{Name: "kube-proxy", CurrentVersion: "v1.29.0-eksbuild.1", ClusterName: "prod"},
		},
		Errors: map[string]error{
			"describe-addon-versions:vpc-cni":    fmt.Errorf("access denied"),
			"describe-addon-versions:kube-proxy": fmt.Errorf("throttled"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(fs), fs)
	}
	for _, f := range fs {
		if f.RuleID != "ADDON-002" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierProviderReported {
			t.Fatalf("finding identity = %+v, want ADDON-002 warning PROVIDER_REPORTED", f)
		}
		if !contains(f.Evidence, "compatibility status: unknown") {
			t.Errorf("evidence = %v, want unknown compatibility status", f.Evidence)
		}
		if !containsPrefix(f.Evidence, "confidence/source: AWS EKS DescribeAddonVersions unavailable") {
			t.Errorf("evidence = %v, want unavailable source", f.Evidence)
		}
	}
}

func TestADDON002_Negative_NonCriticalAddonCompatibilityUnknown(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.30.0-eksbuild.1"}},
		Errors: map[string]error{"describe-addon-versions:aws-ebs-csi-driver": fmt.Errorf("access denied")},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 until CSI lands in PR 2: %+v", len(fs), fs)
	}
}

func TestADDON002_Negative_VerificationSucceededNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "kube-proxy", CurrentVersion: "v1.29.0-eksbuild.1", CompatibleVersions: []string{"v1.29.0-eksbuild.1"}}},
		Errors: map[string]error{},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when compatibility was verified: %+v", len(fs), fs)
	}
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
