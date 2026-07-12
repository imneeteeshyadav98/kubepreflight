// Package aws collects a read-only snapshot of EKS/EC2 provider state used
// by the AWS-enrichment checks (EKS-INSIGHT, ADDON-001, NODE-002). Every AWS
// operation this collector needs is captured as a narrow interface so tests
// inject fakes instead of hitting real AWS — the same dependency-injection
// pattern the k8s collector uses.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
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

	// Insights holds EKS Upgrade Insights returned for the scan's target
	// Kubernetes version. PASSING insights are inventory only; WARNING,
	// ERROR, and UNKNOWN are evaluated by conservative EKS-INSIGHT rules.
	Insights []InsightRecord

	// Addons holds every EKS-managed add-on currently installed, along with
	// which versions are compatible with the scan's target Kubernetes
	// version (ADDON-001).
	Addons []AddonRecord

	// Nodegroups holds every EKS managed node group returned by
	// ListNodegroups. Self-managed node groups are not returned by that AWS
	// API and therefore are not represented here.
	Nodegroups []NodegroupRecord

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
	ClusterName        string
	ID                 string
	Name               string
	Category           string
	KubernetesVersion  string
	Status             string // "PASSING", "WARNING", "ERROR", or "UNKNOWN"
	Reason             string
	Description        string
	Recommendation     string
	LastRefreshTime    time.Time
	LastTransitionTime time.Time
	AdditionalInfo     map[string]string
	DeprecationDetails []string
	AddonCompatibility []string
}

// AddonRecord is one installed EKS add-on and the versions AWS reports as
// compatible with the scan's target Kubernetes version.
type AddonRecord struct {
	ClusterName        string
	Name               string
	CurrentVersion     string
	CompatibleVersions []string
}

// NodegroupRecord is one EKS managed node group and the read-only
// readiness fields AWS exposes via DescribeNodegroup.
type NodegroupRecord struct {
	ClusterName              string
	Name                     string
	Status                   string
	Version                  string
	ReleaseVersion           string
	AMIType                  string
	CapacityType             string
	DesiredSize              *int32
	MinSize                  *int32
	MaxSize                  *int32
	MaxUnavailable           *int32
	MaxUnavailablePercentage *int32
	LaunchTemplate           bool
	HealthIssues             []NodegroupHealthIssue
	AutoScalingGroups        []string
	ReadinessStatus          string
	Notes                    []string
}

// NodegroupHealthIssue is one AWS-reported managed node group health issue.
type NodegroupHealthIssue struct {
	Code        string
	Message     string
	ResourceIDs []string
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
// relevant to targetVersion (EKS-INSIGHT), add-on version compatibility against
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
	} else if out != nil && out.Cluster != nil {
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
	c.collectNodegroups(ctx, snap)
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

// collectInsights populates Insights via ListInsights filtered to
// UPGRADE_READINESS and, when AWS accepts it, the target version. If the
// version-filtered call fails, retrying category-only keeps EKS Upgrade
// Insights available for clusters/regions whose available-version set
// disagrees with the requested target. PASSING insights are kept for
// inventory but do not create findings.
func (c *Collector) collectInsights(ctx context.Context, targetVersion string, snap *Snapshot) {
	summaries, err := c.listUpgradeInsightSummaries(ctx, targetVersion)
	if err != nil {
		if targetVersion != "" {
			if fallbackSummaries, fallbackErr := c.listUpgradeInsightSummaries(ctx, ""); fallbackErr == nil {
				summaries = fallbackSummaries
			} else {
				snap.Errors["list-insights"] = err
				snap.Errors["list-insights-fallback"] = fallbackErr
				return
			}
		} else {
			snap.Errors["list-insights"] = err
			return
		}
	}

	for _, summary := range summaries {
		c.collectInsightDetail(ctx, summary, snap)
	}
}

func (c *Collector) listUpgradeInsightSummaries(ctx context.Context, targetVersion string) ([]ekstypes.InsightSummary, error) {
	filter := &ekstypes.InsightsFilter{Categories: []ekstypes.Category{ekstypes.CategoryUpgradeReadiness}}
	if targetVersion != "" {
		filter.KubernetesVersions = []string{targetVersion}
	}

	var summaries []ekstypes.InsightSummary
	var nextToken *string
	for {
		listOut, err := c.eksClient.ListInsights(ctx, &eks.ListInsightsInput{
			ClusterName: awssdk.String(c.clusterName),
			Filter:      filter,
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, err
		}
		if listOut == nil {
			return summaries, nil
		}
		summaries = append(summaries, listOut.Insights...)
		if listOut.NextToken == nil || *listOut.NextToken == "" {
			return summaries, nil
		}
		nextToken = listOut.NextToken
	}
}

func (c *Collector) collectInsightDetail(ctx context.Context, summary ekstypes.InsightSummary, snap *Snapshot) {
	if summary.Id == nil {
		return
	}

	rec := InsightRecord{
		ClusterName:        c.clusterName,
		ID:                 awssdk.ToString(summary.Id),
		Name:               awssdk.ToString(summary.Name),
		Category:           string(summary.Category),
		KubernetesVersion:  awssdk.ToString(summary.KubernetesVersion),
		Status:             insightStatusValue(summary.InsightStatus),
		Reason:             insightStatusReason(summary.InsightStatus),
		Description:        awssdk.ToString(summary.Description),
		LastRefreshTime:    awssdk.ToTime(summary.LastRefreshTime),
		LastTransitionTime: awssdk.ToTime(summary.LastTransitionTime),
	}

	descOut, err := c.eksClient.DescribeInsight(ctx, &eks.DescribeInsightInput{
		ClusterName: awssdk.String(c.clusterName),
		Id:          summary.Id,
	})
	if err != nil {
		snap.Errors["describe-insight:"+rec.ID] = err
	} else if descOut != nil && descOut.Insight != nil {
		rec = mergeInsightDetails(rec, descOut.Insight)
	}

	snap.Insights = append(snap.Insights, rec)
}

func mergeInsightDetails(rec InsightRecord, ins *ekstypes.Insight) InsightRecord {
	if ins.Id != nil {
		rec.ID = *ins.Id
	}
	if ins.Name != nil {
		rec.Name = *ins.Name
	}
	if ins.Category != "" {
		rec.Category = string(ins.Category)
	}
	if ins.KubernetesVersion != nil {
		rec.KubernetesVersion = *ins.KubernetesVersion
	}
	if ins.InsightStatus != nil {
		rec.Status = insightStatusValue(ins.InsightStatus)
		rec.Reason = insightStatusReason(ins.InsightStatus)
	}
	if ins.Description != nil {
		rec.Description = *ins.Description
	}
	rec.Recommendation = awssdk.ToString(ins.Recommendation)
	if ins.LastRefreshTime != nil {
		rec.LastRefreshTime = *ins.LastRefreshTime
	}
	if ins.LastTransitionTime != nil {
		rec.LastTransitionTime = *ins.LastTransitionTime
	}
	if len(ins.AdditionalInfo) > 0 {
		rec.AdditionalInfo = map[string]string{}
		for k, v := range ins.AdditionalInfo {
			rec.AdditionalInfo[k] = v
		}
	}
	if ins.CategorySpecificSummary != nil {
		rec.DeprecationDetails = deprecationDetailLabels(ins.CategorySpecificSummary.DeprecationDetails)
		rec.AddonCompatibility = addonCompatibilityLabels(ins.CategorySpecificSummary.AddonCompatibilityDetails)
	}
	return rec
}

func deprecationDetailLabels(details []ekstypes.DeprecationDetail) []string {
	out := make([]string, 0, len(details))
	for _, detail := range details {
		parts := []string{}
		if detail.Usage != nil {
			parts = append(parts, "usage: "+*detail.Usage)
		}
		if detail.ReplacedWith != nil {
			parts = append(parts, "replacedWith: "+*detail.ReplacedWith)
		}
		if detail.StopServingVersion != nil {
			parts = append(parts, "stopServingVersion: "+*detail.StopServingVersion)
		}
		if detail.StartServingReplacementVersion != nil {
			parts = append(parts, "startServingReplacementVersion: "+*detail.StartServingReplacementVersion)
		}
		for _, stat := range detail.ClientStats {
			if stat.UserAgent == nil {
				continue
			}
			parts = append(parts, fmt.Sprintf("client %s: %d request(s) in last 30 days", *stat.UserAgent, stat.NumberOfRequestsLast30Days))
		}
		if len(parts) > 0 {
			out = append(out, strings.Join(parts, "; "))
		}
	}
	return out
}

func addonCompatibilityLabels(details []ekstypes.AddonCompatibilityDetail) []string {
	out := make([]string, 0, len(details))
	for _, detail := range details {
		name := awssdk.ToString(detail.Name)
		if name == "" && len(detail.CompatibleVersions) == 0 {
			continue
		}
		if len(detail.CompatibleVersions) == 0 {
			out = append(out, name)
			continue
		}
		out = append(out, fmt.Sprintf("%s compatible versions: %s", name, strings.Join(detail.CompatibleVersions, ", ")))
	}
	return out
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
		} else if describeOut.Addon != nil {
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

// collectNodegroups populates Nodegroups via ListNodegroups and
// DescribeNodegroup. ListNodegroups returns only EKS managed node groups;
// self-managed nodes are intentionally outside this AWS API's scope.
func (c *Collector) collectNodegroups(ctx context.Context, snap *Snapshot) {
	listOut, err := c.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: awssdk.String(c.clusterName)})
	if err != nil {
		snap.Errors["list-nodegroups"] = err
		return
	}
	if listOut == nil {
		return
	}

	for _, name := range listOut.Nodegroups {
		rec := NodegroupRecord{Name: name, ClusterName: c.clusterName}
		describeOut, err := c.eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   awssdk.String(c.clusterName),
			NodegroupName: awssdk.String(name),
		})
		if err != nil {
			snap.Errors["describe-nodegroup:"+name] = err
			continue
		}
		if describeOut.Nodegroup != nil {
			rec = nodegroupRecordFromAWS(c.clusterName, name, describeOut.Nodegroup)
		}
		snap.Nodegroups = append(snap.Nodegroups, rec)
	}
}

func nodegroupRecordFromAWS(clusterName, fallbackName string, ng *ekstypes.Nodegroup) NodegroupRecord {
	rec := NodegroupRecord{
		ClusterName:    clusterName,
		Name:           fallbackName,
		Status:         string(ng.Status),
		Version:        awssdk.ToString(ng.Version),
		ReleaseVersion: awssdk.ToString(ng.ReleaseVersion),
		AMIType:        string(ng.AmiType),
		CapacityType:   string(ng.CapacityType),
		LaunchTemplate: ng.LaunchTemplate != nil,
	}
	if ng.NodegroupName != nil {
		rec.Name = *ng.NodegroupName
	}
	if ng.ScalingConfig != nil {
		rec.DesiredSize = ng.ScalingConfig.DesiredSize
		rec.MinSize = ng.ScalingConfig.MinSize
		rec.MaxSize = ng.ScalingConfig.MaxSize
	}
	if ng.UpdateConfig != nil {
		rec.MaxUnavailable = ng.UpdateConfig.MaxUnavailable
		rec.MaxUnavailablePercentage = ng.UpdateConfig.MaxUnavailablePercentage
	}
	if ng.Resources != nil {
		for _, asg := range ng.Resources.AutoScalingGroups {
			if asg.Name != nil {
				rec.AutoScalingGroups = append(rec.AutoScalingGroups, *asg.Name)
			}
		}
	}
	if ng.Health != nil {
		for _, issue := range ng.Health.Issues {
			rec.HealthIssues = append(rec.HealthIssues, NodegroupHealthIssue{
				Code:        string(issue.Code),
				Message:     awssdk.ToString(issue.Message),
				ResourceIDs: append([]string(nil), issue.ResourceIds...),
			})
		}
	}
	rec.ReadinessStatus, rec.Notes = nodegroupReadiness(rec)
	return rec
}

func nodegroupReadiness(rec NodegroupRecord) (string, []string) {
	var notes []string
	if len(rec.HealthIssues) > 0 {
		notes = append(notes, "EKS reports managed node group health issue(s).")
	}
	if rec.DesiredSize != nil && rec.MinSize != nil && *rec.DesiredSize <= *rec.MinSize {
		notes = append(notes, "Desired size is at or below minimum size, leaving limited rolling update headroom.")
	}
	if rec.LaunchTemplate || rec.AMIType == "CUSTOM" {
		notes = append(notes, "Launch template or custom AMI requires manual validation.")
	}
	if len(notes) == 0 {
		return "Ready with review", nil
	}
	return "Review required", notes
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
	if s == nil || s.Status == "" {
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
