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

// eksClusterInfo builds the report's EKS cluster metadata card from the AWS
// collector's snapshot — nil when AWS enrichment wasn't attempted/available
// at all, or when the underlying DescribeCluster call itself failed
// (per-operation AWS failures are already surfaced separately via
// ScanCoverage; this just avoids rendering an all-empty EKS metadata card).
// Absence must never be read as an upgrade blocker — see findings.EKSClusterInfo.
func eksClusterInfo(clusterName string, awsSnap *awscol.Snapshot) *findings.EKSClusterInfo {
	if awsSnap == nil {
		return nil
	}
	if awsSnap.ClusterVersion == "" && awsSnap.PlatformVersion == "" && awsSnap.Status == "" {
		return nil
	}
	return &findings.EKSClusterInfo{
		ClusterName:     clusterName,
		Region:          awsSnap.Region,
		Version:         awsSnap.ClusterVersion,
		PlatformVersion: awsSnap.PlatformVersion,
		Status:          awsSnap.Status,
		SupportType:     awsSnap.SupportType,
		EndpointAccess:  awsSnap.EndpointAccess,
		ARN:             awsSnap.ARN,
	}
}

// eksAddonInfos builds the report's full EKS add-on inventory from the AWS
// collector's snapshot — every add-on ListAddons returned, not just the
// ones ADDON-001 (internal/rules/addon001.go) flagged as incompatible.
// addonVersionCompatible mirrors that rule's own compatibility check
// exactly (kept as a small duplicated helper rather than an import, since
// internal/cli doesn't otherwise depend on internal/rules) so this
// inventory's "Compatible" column can never silently disagree with
// whether ADDON-001 actually raised a finding for the same add-on.
func eksAddonInfos(awsSnap *awscol.Snapshot) []findings.EKSAddonInfo {
	if awsSnap == nil || len(awsSnap.Addons) == 0 {
		return nil
	}
	out := make([]findings.EKSAddonInfo, 0, len(awsSnap.Addons))
	for _, addon := range awsSnap.Addons {
		_, unavailable := awsSnap.Errors["describe-addon-versions:"+addon.Name]
		out = append(out, findings.EKSAddonInfo{
			Name:                    addon.Name,
			CurrentVersion:          addon.CurrentVersion,
			CompatibleVersions:      addon.CompatibleVersions,
			Compatible:              !unavailable && addonVersionCompatible(addon.CurrentVersion, addon.CompatibleVersions),
			VerificationUnavailable: unavailable,
		})
	}
	return out
}

func addonVersionCompatible(current string, compatible []string) bool {
	for _, v := range compatible {
		if v == current {
			return true
		}
	}
	return false
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
