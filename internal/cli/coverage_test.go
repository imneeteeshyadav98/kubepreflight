package cli

import (
	"fmt"
	"testing"
	"time"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

func TestBuildScanCoverage_RecordsPartialPlane(t *testing.T) {
	coverage := buildScanCoverage(&k8s.Snapshot{Errors: map[string]error{"endpointslices": fmt.Errorf("forbidden")}}, nil, nil, false, false, nil)
	if coverage.Kubernetes.Status != findings.CoveragePartial || len(coverage.Kubernetes.Errors) != 1 {
		t.Fatalf("coverage = %+v", coverage)
	}
	if coverage.AWS.Status != findings.CoverageSkipped || coverage.Manifests.Status != findings.CoverageSkipped {
		t.Fatalf("optional planes = %+v", coverage)
	}
}

func TestBuildScanCoverage_AWSRequestedButUnavailable(t *testing.T) {
	coverage := buildScanCoverage(&k8s.Snapshot{Errors: map[string]error{}}, nil, nil, true, false, fmt.Errorf("credentials unavailable"))
	if coverage.AWS.Status != findings.CoveragePartial || len(coverage.AWS.Errors) != 1 {
		t.Fatalf("AWS coverage = %+v", coverage.AWS)
	}
}

func TestEKSClusterInfo_NilWhenAWSSnapshotNil(t *testing.T) {
	if got := eksClusterInfo("my-cluster", nil); got != nil {
		t.Fatalf("eksClusterInfo(nil snapshot) = %+v, want nil (cluster-only scan or AWS enrichment unavailable)", got)
	}
}

func TestEKSClusterInfo_NilWhenDescribeClusterReturnedNothingUseful(t *testing.T) {
	// e.g. ListInsights/ListAddons succeeded independently but
	// DescribeCluster itself failed — ClusterVersion/PlatformVersion/Status
	// all stay empty, and we must not render an all-empty EKS card.
	got := eksClusterInfo("my-cluster", &awscol.Snapshot{})
	if got != nil {
		t.Fatalf("eksClusterInfo(empty snapshot) = %+v, want nil", got)
	}
}

func TestEKSClusterInfo_PopulatedFromSnapshot(t *testing.T) {
	snap := &awscol.Snapshot{
		ClusterVersion:  "1.29",
		Region:          "ap-south-1",
		PlatformVersion: "eks.5",
		Status:          "ACTIVE",
		SupportType:     "EXTENDED",
		EndpointAccess:  "public",
		ARN:             "arn:aws:eks:ap-south-1:123456789012:cluster/my-cluster",
	}
	got := eksClusterInfo("my-cluster", snap)
	if got == nil {
		t.Fatal("eksClusterInfo returned nil, want populated info")
	}
	want := findings.EKSClusterInfo{
		ClusterName:     "my-cluster",
		Region:          "ap-south-1",
		Version:         "1.29",
		PlatformVersion: "eks.5",
		Status:          "ACTIVE",
		SupportType:     "EXTENDED",
		EndpointAccess:  "public",
		ARN:             "arn:aws:eks:ap-south-1:123456789012:cluster/my-cluster",
	}
	if *got != want {
		t.Fatalf("eksClusterInfo = %+v, want %+v", *got, want)
	}
}

func TestEKSAddonInfos_NilWhenAWSSnapshotNilOrEmpty(t *testing.T) {
	if got := eksAddonInfos(nil); got != nil {
		t.Fatalf("eksAddonInfos(nil) = %+v, want nil", got)
	}
	if got := eksAddonInfos(&awscol.Snapshot{}); got != nil {
		t.Fatalf("eksAddonInfos(no addons) = %+v, want nil", got)
	}
}

// TestEKSAddonInfos_ThreeStates guards the same three-state classification
// ADDON-001 (internal/rules/addon001.go) uses to decide whether to raise a
// finding, so this inventory's "Compatible" column can never silently
// disagree with what actually appears as a finding.
func TestEKSAddonInfos_ThreeStates(t *testing.T) {
	snap := &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.18.1-eksbuild.1", CompatibleVersions: []string{"v1.18.1-eksbuild.1", "v1.18.2-eksbuild.1"}},
			{Name: "coredns", CurrentVersion: "v1.10.1-eksbuild.1", CompatibleVersions: []string{"v1.11.0-eksbuild.1"}},
			{Name: "kube-proxy", CurrentVersion: "v1.29.0-eksbuild.1", CompatibleVersions: nil},
		},
		Errors: map[string]error{"describe-addon-versions:kube-proxy": fmt.Errorf("access denied")},
	}
	got := eksAddonInfos(snap)
	if len(got) != 3 {
		t.Fatalf("eksAddonInfos returned %d entries, want 3", len(got))
	}

	if !got[0].Compatible || got[0].VerificationUnavailable {
		t.Errorf("vpc-cni = %+v, want Compatible=true VerificationUnavailable=false", got[0])
	}
	if got[1].Compatible || got[1].VerificationUnavailable {
		t.Errorf("coredns = %+v, want Compatible=false VerificationUnavailable=false (a real incompatibility)", got[1])
	}
	if got[2].Compatible || !got[2].VerificationUnavailable {
		t.Errorf("kube-proxy = %+v, want Compatible=false VerificationUnavailable=true (describe-addon-versions failed)", got[2])
	}
}

func TestEKSNodegroupInfos_MapsCollectorInventory(t *testing.T) {
	desired, min, max := int32(3), int32(3), int32(8)
	maxUnavailable := int32(1)
	snap := &awscol.Snapshot{
		Nodegroups: []awscol.NodegroupRecord{{
			Name:              "ng-app",
			Status:            "ACTIVE",
			Version:           "1.32",
			ReleaseVersion:    "1.32.7-20260601",
			AMIType:           "AL2023_x86_64_STANDARD",
			CapacityType:      "ON_DEMAND",
			DesiredSize:       &desired,
			MinSize:           &min,
			MaxSize:           &max,
			MaxUnavailable:    &maxUnavailable,
			LaunchTemplate:    true,
			ReadinessStatus:   "Review required",
			Notes:             []string{"Launch template or custom AMI requires manual validation."},
			AutoScalingGroups: []string{"eks-ng-app-asg"},
			HealthIssues:      []awscol.NodegroupHealthIssue{{Code: "AccessDenied", Message: "node role cannot call API", ResourceIDs: []string{"i-123"}}},
		}},
	}

	got := eksNodegroupInfos(snap)
	if len(got) != 1 {
		t.Fatalf("eksNodegroupInfos returned %d entries, want 1", len(got))
	}
	ng := got[0]
	if ng.Name != "ng-app" || ng.Status != "ACTIVE" || ng.Version != "1.32" || ng.ReleaseVersion != "1.32.7-20260601" {
		t.Errorf("nodegroup info = %+v, unexpected identity/version fields", ng)
	}
	if ng.DesiredSize == nil || *ng.DesiredSize != 3 || ng.MaxUnavailable == nil || *ng.MaxUnavailable != 1 {
		t.Errorf("nodegroup info = %+v, expected scaling/update pointers", ng)
	}
	if len(ng.HealthIssues) != 1 || ng.HealthIssues[0].Code != "AccessDenied" {
		t.Errorf("HealthIssues = %+v, want AccessDenied", ng.HealthIssues)
	}
	if !ng.LaunchTemplate || len(ng.AutoScalingGroups) != 1 || ng.ReadinessStatus != "Review required" {
		t.Errorf("nodegroup info = %+v, unexpected readiness/context fields", ng)
	}
}

func TestEKSUpgradeInsightInfos_MapsCollectorInventoryIncludingPassing(t *testing.T) {
	refreshTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	transitionTime := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	snap := &awscol.Snapshot{
		Insights: []awscol.InsightRecord{{
			ID:                 "insight-1",
			Name:               "Deprecated API usage",
			Category:           "UPGRADE_READINESS",
			Status:             "PASSING",
			KubernetesVersion:  "1.34",
			LastRefreshTime:    refreshTime,
			LastTransitionTime: transitionTime,
			Description:        "No deprecated API usage detected.",
			Recommendation:     "No action required.",
			AdditionalInfo:     map[string]string{"docs": "https://docs.aws.amazon.com/eks/"},
			DeprecationDetails: []string{"usage: policy/v1beta1/podsecuritypolicies"},
			AddonCompatibility: []string{"vpc-cni compatible versions: v1.18.1-eksbuild.1"},
		}},
	}

	got := eksUpgradeInsightInfos(snap)
	if len(got) != 1 {
		t.Fatalf("eksUpgradeInsightInfos returned %d entries, want 1", len(got))
	}
	ins := got[0]
	if ins.ID != "insight-1" || ins.Status != "PASSING" || ins.KubernetesVersion != "1.34" {
		t.Errorf("insight info = %+v, unexpected identity/status fields", ins)
	}
	if ins.LastRefreshTime != "2026-06-01T00:00:00Z" || ins.LastTransitionTime != "2026-06-02T00:00:00Z" {
		t.Errorf("insight times = %q/%q, want RFC3339 UTC strings", ins.LastRefreshTime, ins.LastTransitionTime)
	}
	if ins.AdditionalInfo["docs"] == "" || len(ins.DeprecationDetails) != 1 || len(ins.AddonCompatibility) != 1 {
		t.Errorf("insight detail fields = %+v, want additional/deprecation/add-on detail", ins)
	}
}
