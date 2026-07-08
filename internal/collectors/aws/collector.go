// Package aws collects a read-only snapshot of EKS/EC2 provider state used
// by the AWS-enrichment checks (API-002, ADDON-001, NODE-002). Every AWS
// operation this collector needs is captured as a narrow interface so tests
// inject fakes instead of hitting real AWS — the same dependency-injection
// pattern the k8s collector uses.
package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"
)

// EKSClient captures exactly the EKS operations this collector and its
// rules need. The real *eks.Client satisfies this structurally.
type EKSClient interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	ListInsights(ctx context.Context, params *eks.ListInsightsInput, optFns ...func(*eks.Options)) (*eks.ListInsightsOutput, error)
	DescribeInsight(ctx context.Context, params *eks.DescribeInsightInput, optFns ...func(*eks.Options)) (*eks.DescribeInsightOutput, error)
	ListAddons(ctx context.Context, params *eks.ListAddonsInput, optFns ...func(*eks.Options)) (*eks.ListAddonsOutput, error)
	DescribeAddon(ctx context.Context, params *eks.DescribeAddonInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonOutput, error)
	DescribeAddonVersions(ctx context.Context, params *eks.DescribeAddonVersionsInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonVersionsOutput, error)
}

// EC2Client captures exactly the EC2 operations this collector needs.
type EC2Client interface {
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

// Snapshot is the read-only AWS provider state a scan operates on.
type Snapshot struct {
	ClusterVersion string
	VpcID          string
	EndpointAccess string // "public", "private", "public_and_private", or "unknown"

	// Region is the AWS region the SDK resolved from the standard
	// credential/config chain (env var, shared config, EC2/IRSA
	// metadata) — not a new CLI flag, just surfacing what LoadCollector
	// already had to resolve to make any AWS call at all.
	Region string
	// PlatformVersion, Status, and SupportType are additional
	// DescribeCluster fields alongside ClusterVersion/VpcID/EndpointAccess
	// above — same API call, no extra AWS permission required.
	// SupportType is "STANDARD" or "EXTENDED" (EKS's extended support
	// program), empty if the cluster predates that field.
	PlatformVersion string
	Status          string
	SupportType     string
	// ARN is the cluster's Amazon Resource Name (includes the AWS account
	// ID), from the same DescribeCluster call.
	ARN string

	// Insights holds EKS Upgrade Insights whose status is WARNING or ERROR
	// for the scan's target Kubernetes version (API-002). PASSING/UNKNOWN
	// insights carry no actionable signal and aren't collected.
	Insights []InsightRecord

	// Addons holds every EKS-managed add-on currently installed, along with
	// which versions are compatible with the scan's target Kubernetes
	// version (ADDON-001).
	Addons []AddonRecord

	// Subnets holds the cluster's control-plane subnets and their free IP
	// headroom (NODE-002).
	Subnets []SubnetRecord

	// NetworkPreflightIssues holds security groups or the VPC the cluster
	// references that no longer exist — hard control-plane upgrade-failure
	// preconditions per AWS's own troubleshooting documentation
	// (SecurityGroupNotFound, VpcIdNotFound), not soft warnings (NET-002).
	NetworkPreflightIssues []NetworkPreflightIssue

	// Errors records collectors that failed, keyed by operation, so a scan
	// can report partial AWS results instead of dropping enrichment
	// entirely — same principle as the k8s collector's Snapshot.Errors.
	Errors map[string]error
}

// NetworkPreflightIssue is one cluster-referenced VPC/security-group
// resource that no longer resolves.
type NetworkPreflightIssue struct {
	Kind string // "SecurityGroup" or "Vpc"
	ID   string
}

// InsightRecord is one EKS Upgrade Insight relevant to the scan's target
// version.
type InsightRecord struct {
	ClusterName       string
	ID                string
	Name              string
	Category          string
	KubernetesVersion string
	Status            string // "WARNING" or "ERROR" (PASSING/UNKNOWN are filtered out at collection)
	Reason            string
	Description       string
	Recommendation    string
	LastRefreshTime   time.Time
}

// AddonRecord is one installed EKS add-on and the versions AWS reports as
// compatible with the scan's target Kubernetes version.
type AddonRecord struct {
	ClusterName        string
	Name               string
	CurrentVersion     string
	CompatibleVersions []string
}

// SubnetRecord is one of the cluster's control-plane subnets.
type SubnetRecord struct {
	ID                      string
	AvailableIPAddressCount int32
}

// Collector gathers a Snapshot via read-only EKS/EC2 API calls.
type Collector struct {
	eksClient   EKSClient
	ec2Client   EC2Client
	clusterName string

	// Region is the AWS region calls are made against, copied into every
	// Snapshot this Collector produces. Exported so LoadCollector can set
	// it after construction without changing NewCollector's signature
	// (test fakes construct via NewCollector directly and don't need a
	// region). Empty for hand-built test Collectors, which is fine — the
	// Region snapshot field and its report chip are then simply absent.
	Region string
}

// NewCollector builds a Collector from already-constructed clients. Real
// callers use LoadCollector; tests pass hand-rolled fakes.
func NewCollector(eksClient EKSClient, ec2Client EC2Client, clusterName string) *Collector {
	return &Collector{eksClient: eksClient, ec2Client: ec2Client, clusterName: clusterName}
}

// LoadCollector loads AWS credentials the standard SDK way (environment,
// shared config/credentials files, EC2/ECS/EKS instance role) and builds a
// Collector against real AWS.
//
// It returns an error if no usable credentials are found. Callers MUST
// treat that as a signal to gracefully skip AWS enrichment and continue
// with a cluster-only scan — never as a reason to fail the whole scan.
// KubePreflight's CLI-first adoption path depends on this: it has to stay
// useful with zero AWS setup.
func LoadCollector(ctx context.Context, clusterName string) (*Collector, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	// LoadDefaultConfig succeeds even with no credentials configured — the
	// failure only surfaces on the first real API call. Force that check
	// now so callers get one clean "AWS unavailable" signal up front
	// instead of a confusing per-check error later.
	if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
		return nil, fmt.Errorf(
			"no AWS credentials found — configure them via `aws configure`, the AWS_PROFILE environment variable, "+
				"or an IAM role (EC2/ECS/EKS instance role, IRSA) before using --provider=eks (SDK detail: %v)", err)
	}

	c := NewCollector(eks.NewFromConfig(cfg), ec2.NewFromConfig(cfg), clusterName)
	c.Region = cfg.Region
	return c, nil
}

// Collect gathers cluster metadata (DescribeCluster), EKS Upgrade Insights
// relevant to targetVersion (API-002), add-on version compatibility against
// targetVersion (ADDON-001), control-plane subnet IP headroom (NODE-002),
// and VPC/security-group existence (NET-002). A failure in one operation is
// recorded in Snapshot.Errors and does not abort the others — never
// all-or-nothing, same as the k8s collector.
func (c *Collector) Collect(ctx context.Context, targetVersion string) (*Snapshot, error) {
	snap := &Snapshot{Errors: map[string]error{}, Region: c.Region}

	var subnetIDs, securityGroupIDs []string
	var vpcID string
	out, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: awssdk.String(c.clusterName)})
	if err != nil {
		snap.Errors["describe-cluster"] = err
	} else if out.Cluster != nil {
		if out.Cluster.Version != nil {
			snap.ClusterVersion = *out.Cluster.Version
		}
		snap.PlatformVersion = awssdk.ToString(out.Cluster.PlatformVersion)
		snap.Status = string(out.Cluster.Status)
		snap.ARN = awssdk.ToString(out.Cluster.Arn)
		if out.Cluster.UpgradePolicy != nil {
			snap.SupportType = string(out.Cluster.UpgradePolicy.SupportType)
		}
		if out.Cluster.ResourcesVpcConfig != nil {
			if out.Cluster.ResourcesVpcConfig.VpcId != nil {
				snap.VpcID = *out.Cluster.ResourcesVpcConfig.VpcId
				vpcID = *out.Cluster.ResourcesVpcConfig.VpcId
			}
			snap.EndpointAccess = endpointAccessLabel(out.Cluster.ResourcesVpcConfig)
			subnetIDs = out.Cluster.ResourcesVpcConfig.SubnetIds
			securityGroupIDs = out.Cluster.ResourcesVpcConfig.SecurityGroupIds
			if out.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId != nil {
				securityGroupIDs = append(securityGroupIDs, *out.Cluster.ResourcesVpcConfig.ClusterSecurityGroupId)
			}
		}
	}

	c.collectInsights(ctx, targetVersion, snap)
	c.collectAddons(ctx, targetVersion, snap)
	c.collectSubnets(ctx, subnetIDs, snap)
	c.collectNetworkPreflight(ctx, vpcID, securityGroupIDs, snap)

	return snap, nil
}

// DescribeClusterVersion returns the cluster's current Kubernetes version
// via a single DescribeCluster call, without the version-filtered
// Insights/Addons/Subnets/NetworkPreflight collection Collect performs.
// Used by the `plan` command's --from-version=auto discovery, which needs
// the current version before it has decided what hop-1 target version to
// filter AWS-enrichment calls by.
func (c *Collector) DescribeClusterVersion(ctx context.Context) (string, error) {
	out, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: awssdk.String(c.clusterName)})
	if err != nil {
		return "", fmt.Errorf("describing cluster: %w", err)
	}
	if out.Cluster == nil || out.Cluster.Version == nil {
		return "", fmt.Errorf("cluster %q has no reported version", c.clusterName)
	}
	return *out.Cluster.Version, nil
}

// collectInsights populates Insights via ListInsights (filtered to
// UPGRADE_READINESS and the target version) then DescribeInsight for each
// non-passing result to pull its full recommendation text.
func (c *Collector) collectInsights(ctx context.Context, targetVersion string, snap *Snapshot) {
	listOut, err := c.eksClient.ListInsights(ctx, &eks.ListInsightsInput{
		ClusterName: awssdk.String(c.clusterName),
		Filter: &ekstypes.InsightsFilter{
			Categories:         []ekstypes.Category{ekstypes.CategoryUpgradeReadiness},
			KubernetesVersions: []string{targetVersion},
		},
	})
	if err != nil {
		snap.Errors["list-insights"] = err
		return
	}

	for _, summary := range listOut.Insights {
		status := insightStatusValue(summary.InsightStatus)
		if status != string(ekstypes.InsightStatusValueWarning) && status != string(ekstypes.InsightStatusValueError) {
			continue // PASSING/UNKNOWN carry no actionable signal for API-002
		}
		if summary.Id == nil {
			continue
		}

		rec := InsightRecord{
			ClusterName:       c.clusterName,
			ID:                awssdk.ToString(summary.Id),
			Name:              awssdk.ToString(summary.Name),
			Category:          string(summary.Category),
			KubernetesVersion: awssdk.ToString(summary.KubernetesVersion),
			Status:            status,
			Reason:            insightStatusReason(summary.InsightStatus),
			Description:       awssdk.ToString(summary.Description),
			LastRefreshTime:   awssdk.ToTime(summary.LastRefreshTime),
		}

		descOut, err := c.eksClient.DescribeInsight(ctx, &eks.DescribeInsightInput{
			ClusterName: awssdk.String(c.clusterName),
			Id:          summary.Id,
		})
		if err != nil {
			snap.Errors["describe-insight:"+rec.ID] = err
		} else if descOut.Insight != nil {
			rec.Recommendation = awssdk.ToString(descOut.Insight.Recommendation)
			if rec.Description == "" {
				rec.Description = awssdk.ToString(descOut.Insight.Description)
			}
		}

		snap.Insights = append(snap.Insights, rec)
	}
}

// collectAddons populates Addons via ListAddons, then for each installed
// add-on: DescribeAddon for the currently-installed version, and
// DescribeAddonVersions filtered to targetVersion for the compatible set.
func (c *Collector) collectAddons(ctx context.Context, targetVersion string, snap *Snapshot) {
	listOut, err := c.eksClient.ListAddons(ctx, &eks.ListAddonsInput{ClusterName: awssdk.String(c.clusterName)})
	if err != nil {
		snap.Errors["list-addons"] = err
		return
	}

	for _, name := range listOut.Addons {
		rec := AddonRecord{Name: name, ClusterName: c.clusterName}

		describeOut, err := c.eksClient.DescribeAddon(ctx, &eks.DescribeAddonInput{
			ClusterName: awssdk.String(c.clusterName),
			AddonName:   awssdk.String(name),
		})
		if err != nil {
			snap.Errors["describe-addon:"+name] = err
			continue
		}
		if describeOut.Addon != nil {
			rec.CurrentVersion = awssdk.ToString(describeOut.Addon.AddonVersion)
		}

		versionsOut, err := c.eksClient.DescribeAddonVersions(ctx, &eks.DescribeAddonVersionsInput{
			AddonName:         awssdk.String(name),
			KubernetesVersion: awssdk.String(targetVersion),
		})
		if err != nil {
			snap.Errors["describe-addon-versions:"+name] = err
		} else {
			for _, info := range versionsOut.Addons {
				for _, v := range info.AddonVersions {
					if v.AddonVersion != nil {
						rec.CompatibleVersions = append(rec.CompatibleVersions, *v.AddonVersion)
					}
				}
			}
		}

		snap.Addons = append(snap.Addons, rec)
	}
}

// collectSubnets populates Subnets via DescribeSubnets for the cluster's
// control-plane subnet IDs (from DescribeCluster's VpcConfig).
func (c *Collector) collectSubnets(ctx context.Context, subnetIDs []string, snap *Snapshot) {
	if len(subnetIDs) == 0 {
		return
	}

	out, err := c.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: subnetIDs})
	if err != nil {
		snap.Errors["describe-subnets"] = err
		return
	}

	for _, s := range out.Subnets {
		rec := SubnetRecord{ID: awssdk.ToString(s.SubnetId)}
		if s.AvailableIpAddressCount != nil {
			rec.AvailableIPAddressCount = *s.AvailableIpAddressCount
		}
		snap.Subnets = append(snap.Subnets, rec)
	}
}

// collectNetworkPreflight verifies the cluster's referenced security
// groups and VPC still exist. Each ID is checked with its own API call
// rather than batched into one DescribeSecurityGroups/DescribeVpcs call
// with multiple IDs: EC2's ID-filtered Describe calls fail the whole
// request if any one ID is invalid, and don't return partial results for
// the IDs that are still valid — checking one at a time is what makes a
// NotFound error unambiguously attributable to a single ID, without
// parsing AWS's free-text error message to figure out which one failed.
func (c *Collector) collectNetworkPreflight(ctx context.Context, vpcID string, securityGroupIDs []string, snap *Snapshot) {
	for _, sgID := range securityGroupIDs {
		if sgID == "" {
			continue
		}
		_, err := c.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{sgID}})
		if err == nil {
			continue
		}
		if isAWSErrorCode(err, "InvalidGroup.NotFound") {
			snap.NetworkPreflightIssues = append(snap.NetworkPreflightIssues, NetworkPreflightIssue{Kind: "SecurityGroup", ID: sgID})
		} else {
			snap.Errors["describe-security-group:"+sgID] = err
		}
	}

	if vpcID == "" {
		return
	}
	_, err := c.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{vpcID}})
	if err == nil {
		return
	}
	if isAWSErrorCode(err, "InvalidVpcID.NotFound") {
		snap.NetworkPreflightIssues = append(snap.NetworkPreflightIssues, NetworkPreflightIssue{Kind: "Vpc", ID: vpcID})
	} else {
		snap.Errors["describe-vpc:"+vpcID] = err
	}
}

// isAWSErrorCode reports whether err is an AWS API error with exactly the
// given error code (e.g. "InvalidGroup.NotFound"), as opposed to any other
// failure (permissions, throttling, network) that should be recorded as a
// collection error, not misread as "the resource doesn't exist."
func isAWSErrorCode(err error, code string) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == code
	}
	return false
}

func insightStatusValue(s *ekstypes.InsightStatus) string {
	if s == nil {
		return string(ekstypes.InsightStatusValueUnknown)
	}
	return string(s.Status)
}

func insightStatusReason(s *ekstypes.InsightStatus) string {
	if s == nil {
		return ""
	}
	return awssdk.ToString(s.Reason)
}

func endpointAccessLabel(vpcConfig *ekstypes.VpcConfigResponse) string {
	pub := vpcConfig.EndpointPublicAccess
	priv := vpcConfig.EndpointPrivateAccess
	switch {
	case pub && priv:
		return "public_and_private"
	case pub:
		return "public"
	case priv:
		return "private"
	default:
		return "unknown"
	}
}
