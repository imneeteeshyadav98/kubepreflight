package cli

import (
	"fmt"
	"testing"

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
