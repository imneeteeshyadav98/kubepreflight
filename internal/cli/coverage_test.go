package cli

import (
	"fmt"
	"testing"

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
