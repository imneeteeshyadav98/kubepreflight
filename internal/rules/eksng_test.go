package rules

import (
	"strings"
	"testing"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

func TestEKSNG001_HealthIssuesCreateWarning(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Nodegroups: []awscol.NodegroupRecord{{
		ClusterName: "prod", Name: "ng-app",
		HealthIssues: []awscol.NodegroupHealthIssue{{
			Code: "AccessDenied", Message: "node role cannot call API", ResourceIDs: []string{"i-123"},
		}},
	}}}}

	fs, err := EKSNG001{}.Evaluate(sc, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	f := fs[0]
	if f.RuleID != "EKS-NG-001" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierProviderReported {
		t.Fatalf("finding = %+v, unexpected classification", f)
	}
	if f.Resources[0].Kind != "EKSNodegroup" || f.Resources[0].ProviderID != "prod/ng-app" {
		t.Errorf("resource = %+v, want EKSNodegroup prod/ng-app", f.Resources[0])
	}
	if len(f.Evidence) == 0 || f.Evidence[0] == "" {
		t.Errorf("evidence missing health issue details: %+v", f.Evidence)
	}
}

func TestEKSNG002_LimitedHeadroomCreatesWarning(t *testing.T) {
	desired, min, max := int32(3), int32(3), int32(8)
	sc := &ScanContext{AWS: &awscol.Snapshot{Nodegroups: []awscol.NodegroupRecord{{
		ClusterName: "prod", Name: "ng-app", DesiredSize: &desired, MinSize: &min, MaxSize: &max,
	}}}}

	fs, err := EKSNG002{}.Evaluate(sc, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityWarning {
		t.Fatalf("findings = %+v, want one warning", fs)
	}
	if got := fs[0].Evidence; len(got) != 3 {
		t.Errorf("evidence = %+v, want desired/min/max", got)
	}
}

func TestEKSNG003_LaunchTemplateOrCustomAMICreatesInfo(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Nodegroups: []awscol.NodegroupRecord{
		{ClusterName: "prod", Name: "lt-ng", LaunchTemplate: true, AMIType: "AL2023_x86_64_STANDARD"},
		{ClusterName: "prod", Name: "custom-ng", AMIType: "CUSTOM"},
		{ClusterName: "prod", Name: "plain-ng", AMIType: "AL2023_x86_64_STANDARD"},
	}}}

	fs, err := EKSNG003{}.Evaluate(sc, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("findings = %d, want 2", len(fs))
	}
	for _, f := range fs {
		if f.Severity != findings.SeverityInfo {
			t.Errorf("%s severity = %s, want Info", f.Resources[0].Name, f.Severity)
		}
	}
}

func TestEKSNG004_VersionContextCreatesInfoWithoutDuplicatingNodeBlocker(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{Nodegroups: []awscol.NodegroupRecord{{
		ClusterName: "prod", Name: "ng-app", Version: "1.32",
	}}}}

	fs, err := EKSNG004{}.Evaluate(sc, "1.36")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	f := fs[0]
	if f.Severity != findings.SeverityInfo {
		t.Fatalf("severity = %s, want Info", f.Severity)
	}
	if f.RuleID != "EKS-NG-004" || f.Message == "" {
		t.Fatalf("finding = %+v, unexpected finding", f)
	}
	if !strings.Contains(f.Message, "Node kubelet skew is evaluated separately by NODE-001") {
		t.Errorf("message = %q, want NODE-001 separation wording", f.Message)
	}
}
