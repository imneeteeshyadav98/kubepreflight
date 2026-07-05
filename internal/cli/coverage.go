package cli

import (
	"fmt"
	"sort"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	manifestcol "kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

func buildScanCoverage(k8sSnap *k8s.Snapshot, awsSnap *awscol.Snapshot, manifestSnap *manifestcol.Snapshot, awsRequested, manifestsRequested bool, awsUnavailable error) findings.ScanCoverage {
	coverage := findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	}
	if k8sSnap == nil {
		coverage.Kubernetes = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"cluster snapshot unavailable"}}
	} else if len(k8sSnap.Errors) > 0 {
		coverage.Kubernetes = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: stableErrors(k8sSnap.Errors)}
	}
	if awsRequested {
		coverage.AWS.Status = findings.CoverageComplete
		if awsSnap == nil {
			message := "AWS enrichment unavailable"
			if awsUnavailable != nil {
				message = awsUnavailable.Error()
			}
			coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{message}}
		} else if len(awsSnap.Errors) > 0 {
			coverage.AWS = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: stableErrors(awsSnap.Errors)}
		}
	}
	if manifestsRequested {
		coverage.Manifests.Status = findings.CoverageComplete
		if manifestSnap == nil {
			coverage.Manifests = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"manifest snapshot unavailable"}}
		} else if len(manifestSnap.Errors) > 0 {
			coverage.Manifests = findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: stableErrors(manifestSnap.Errors)}
		}
	}
	return coverage
}

func stableErrors(in map[string]error) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s: %v", key, in[key]))
	}
	return out
}
