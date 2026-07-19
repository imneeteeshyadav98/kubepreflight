package aws_test

import (
	"context"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
)

// fakeEKSClient is a hand-rolled fake — no real AWS calls in this test
// suite. It implements exactly the awscol.EKSClient interface.
type fakeEKSClient struct {
	describeClusterOut *eks.DescribeClusterOutput
	describeClusterErr error

	listInsightsOut              *eks.ListInsightsOutput
	listInsightsErr              error
	listInsightsOutByToken       map[string]*eks.ListInsightsOutput
	listInsightsFallbackOut      *eks.ListInsightsOutput
	listInsightsVersionFilterErr error
	listInsightsInputs           []*eks.ListInsightsInput

	describeInsightOut map[string]*eks.DescribeInsightOutput // keyed by insight ID
	describeInsightErr map[string]error

	listAddonsOut *eks.ListAddonsOutput
	listAddonsErr error

	describeAddonOut map[string]*eks.DescribeAddonOutput // keyed by addon name
	describeAddonErr map[string]error

	describeAddonVersionsOut map[string]*eks.DescribeAddonVersionsOutput // keyed by addon name
	describeAddonVersionsErr map[string]error

	listNodegroupsOut *eks.ListNodegroupsOutput
	listNodegroupsErr error

	describeNodegroupOut map[string]*eks.DescribeNodegroupOutput // keyed by node group name
	describeNodegroupErr map[string]error
}

func (f *fakeEKSClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return f.describeClusterOut, f.describeClusterErr
}

func (f *fakeEKSClient) ListInsights(ctx context.Context, params *eks.ListInsightsInput, optFns ...func(*eks.Options)) (*eks.ListInsightsOutput, error) {
	f.listInsightsInputs = append(f.listInsightsInputs, params)
	if params.Filter != nil && len(params.Filter.KubernetesVersions) > 0 && f.listInsightsVersionFilterErr != nil {
		return nil, f.listInsightsVersionFilterErr
	}
	if params.Filter != nil && len(params.Filter.KubernetesVersions) == 0 && f.listInsightsFallbackOut != nil {
		return f.listInsightsFallbackOut, nil
	}
	if f.listInsightsOutByToken != nil {
		return f.listInsightsOutByToken[awssdk.ToString(params.NextToken)], nil
	}
	return f.listInsightsOut, f.listInsightsErr
}

func (f *fakeEKSClient) DescribeInsight(ctx context.Context, params *eks.DescribeInsightInput, optFns ...func(*eks.Options)) (*eks.DescribeInsightOutput, error) {
	id := awssdk.ToString(params.Id)
	return f.describeInsightOut[id], f.describeInsightErr[id]
}

func (f *fakeEKSClient) ListAddons(ctx context.Context, params *eks.ListAddonsInput, optFns ...func(*eks.Options)) (*eks.ListAddonsOutput, error) {
	return f.listAddonsOut, f.listAddonsErr
}

func (f *fakeEKSClient) DescribeAddon(ctx context.Context, params *eks.DescribeAddonInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonOutput, error) {
	name := awssdk.ToString(params.AddonName)
	return f.describeAddonOut[name], f.describeAddonErr[name]
}

func (f *fakeEKSClient) DescribeAddonVersions(ctx context.Context, params *eks.DescribeAddonVersionsInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonVersionsOutput, error) {
	name := awssdk.ToString(params.AddonName)
	return f.describeAddonVersionsOut[name], f.describeAddonVersionsErr[name]
}

func (f *fakeEKSClient) ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	return f.listNodegroupsOut, f.listNodegroupsErr
}

func (f *fakeEKSClient) DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	name := awssdk.ToString(params.NodegroupName)
	return f.describeNodegroupOut[name], f.describeNodegroupErr[name]
}

// fakeEC2Client implements exactly the awscol.EC2Client interface.
type fakeEC2Client struct {
	describeSubnetsOut *ec2.DescribeSubnetsOutput
	describeSubnetsErr error

	describeSecurityGroupsErr map[string]error // keyed by the single requested GroupId
	describeVpcsErr           map[string]error // keyed by the single requested VpcId
}

func (f *fakeEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return f.describeSubnetsOut, f.describeSubnetsErr
}

func (f *fakeEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	id := params.GroupIds[0]
	if err, ok := f.describeSecurityGroupsErr[id]; ok {
		return nil, err
	}
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (f *fakeEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	id := params.VpcIds[0]
	if err, ok := f.describeVpcsErr[id]; ok {
		return nil, err
	}
	return &ec2.DescribeVpcsOutput{}, nil
}

// awsNotFoundError builds a fake AWS API error with the given error code,
// matching how the real SDK surfaces NotFound-style failures.
func awsNotFoundError(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: "not found"}
}

func TestCollector_Collect_FullHappyPath(t *testing.T) {
	refreshTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	transitionTime := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				Version: awssdk.String("1.29"),
				ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
					VpcId:                awssdk.String("vpc-123"),
					SubnetIds:            []string{"subnet-a", "subnet-b"},
					EndpointPublicAccess: true,
				},
			},
		},
		listInsightsOut: &eks.ListInsightsOutput{
			Insights: []ekstypes.InsightSummary{
				{
					Id: awssdk.String("insight-1"), Name: awssdk.String("Deprecated API usage"),
					Category: ekstypes.CategoryUpgradeReadiness, KubernetesVersion: awssdk.String("1.34"),
					InsightStatus:   &ekstypes.InsightStatus{Status: ekstypes.InsightStatusValueError, Reason: awssdk.String("PSP in use")},
					LastRefreshTime: &refreshTime,
				},
				{
					// PASSING must be filtered out entirely.
					Id: awssdk.String("insight-2"), Name: awssdk.String("Clean check"),
					Category: ekstypes.CategoryUpgradeReadiness, KubernetesVersion: awssdk.String("1.34"),
					InsightStatus: &ekstypes.InsightStatus{Status: ekstypes.InsightStatusValuePassing},
				},
			},
		},
		describeInsightOut: map[string]*eks.DescribeInsightOutput{
			"insight-1": {Insight: &ekstypes.Insight{
				Recommendation:     awssdk.String("Migrate off PodSecurityPolicy"),
				LastTransitionTime: &transitionTime,
				AdditionalInfo:     map[string]string{"docs": "https://docs.aws.amazon.com/eks/"},
				CategorySpecificSummary: &ekstypes.InsightCategorySpecificSummary{
					DeprecationDetails: []ekstypes.DeprecationDetail{{
						Usage:              awssdk.String("policy/v1beta1/podsecuritypolicies"),
						StopServingVersion: awssdk.String("1.25"),
						ReplacedWith:       awssdk.String("policy/v1/podsecuritystandards"),
						ClientStats: []ekstypes.ClientStat{{
							UserAgent:                  awssdk.String("kubectl/v1.24"),
							NumberOfRequestsLast30Days: 7,
						}},
					}},
					AddonCompatibilityDetails: []ekstypes.AddonCompatibilityDetail{{
						Name: awssdk.String("vpc-cni"), CompatibleVersions: []string{"v1.18.1-eksbuild.1"},
					}},
				},
			}},
		},
		listAddonsOut: &eks.ListAddonsOutput{Addons: []string{"vpc-cni"}},
		describeAddonOut: map[string]*eks.DescribeAddonOutput{
			"vpc-cni": {Addon: &ekstypes.Addon{AddonVersion: awssdk.String("v1.15.0-eksbuild.1")}},
		},
		describeAddonVersionsOut: map[string]*eks.DescribeAddonVersionsOutput{
			"vpc-cni": {Addons: []ekstypes.AddonInfo{
				{AddonVersions: []ekstypes.AddonVersionInfo{
					{AddonVersion: awssdk.String("v1.18.0-eksbuild.1")},
					{AddonVersion: awssdk.String("v1.18.1-eksbuild.1")},
				}},
			}},
		},
		listNodegroupsOut: &eks.ListNodegroupsOutput{Nodegroups: []string{"ng-app"}},
		describeNodegroupOut: map[string]*eks.DescribeNodegroupOutput{
			"ng-app": {Nodegroup: &ekstypes.Nodegroup{
				NodegroupName:  awssdk.String("ng-app"),
				Status:         ekstypes.NodegroupStatusActive,
				Version:        awssdk.String("1.32"),
				ReleaseVersion: awssdk.String("1.32.7-20260601"),
				AmiType:        ekstypes.AMITypesAl2023X8664Standard,
				CapacityType:   ekstypes.CapacityTypesOnDemand,
				ScalingConfig:  &ekstypes.NodegroupScalingConfig{DesiredSize: awssdk.Int32(3), MinSize: awssdk.Int32(3), MaxSize: awssdk.Int32(8)},
				UpdateConfig:   &ekstypes.NodegroupUpdateConfig{MaxUnavailable: awssdk.Int32(1)},
				LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Name: awssdk.String("custom-ng")},
				Health: &ekstypes.NodegroupHealth{Issues: []ekstypes.Issue{{
					Code:        ekstypes.NodegroupIssueCodeAccessDenied,
					Message:     awssdk.String("node role cannot call API"),
					ResourceIds: []string{"i-123"},
				}}},
				Resources: &ekstypes.NodegroupResources{AutoScalingGroups: []ekstypes.AutoScalingGroup{{Name: awssdk.String("eks-ng-app-asg")}}},
			}},
		},
	}

	ec2Client := &fakeEC2Client{
		describeSubnetsOut: &ec2.DescribeSubnetsOutput{
			Subnets: []ec2types.Subnet{
				{SubnetId: awssdk.String("subnet-a"), AvailableIpAddressCount: awssdk.Int32(3)},
				{SubnetId: awssdk.String("subnet-b"), AvailableIpAddressCount: awssdk.Int32(200)},
			},
		},
	}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}
	if len(eksClient.listInsightsInputs) != 1 {
		t.Fatalf("ListInsights calls = %d, want 1", len(eksClient.listInsightsInputs))
	}
	insightFilter := eksClient.listInsightsInputs[0].Filter
	if insightFilter == nil || len(insightFilter.Categories) != 1 || insightFilter.Categories[0] != ekstypes.CategoryUpgradeReadiness {
		t.Fatalf("ListInsights category filter = %+v, want UPGRADE_READINESS", insightFilter)
	}
	if len(insightFilter.KubernetesVersions) != 1 || insightFilter.KubernetesVersions[0] != "1.34" {
		t.Fatalf("ListInsights KubernetesVersions = %v, want [1.34]", insightFilter.KubernetesVersions)
	}

	if snap.ClusterVersion != "1.29" {
		t.Errorf("ClusterVersion = %q, want 1.29", snap.ClusterVersion)
	}
	if snap.VpcID != "vpc-123" {
		t.Errorf("VpcID = %q, want vpc-123", snap.VpcID)
	}
	if snap.EndpointAccess != "public" {
		t.Errorf("EndpointAccess = %q, want public", snap.EndpointAccess)
	}

	if len(snap.Insights) != 2 {
		t.Fatalf("Insights = %d, want 2 (PASSING must remain as inventory)", len(snap.Insights))
	}
	ins := snap.Insights[0]
	if ins.ID != "insight-1" || ins.Status != "ERROR" || ins.Recommendation != "Migrate off PodSecurityPolicy" {
		t.Errorf("Insights[0] = %+v, unexpected values", ins)
	}
	if !ins.LastRefreshTime.Equal(refreshTime) {
		t.Errorf("LastRefreshTime = %v, want %v", ins.LastRefreshTime, refreshTime)
	}
	if !ins.LastTransitionTime.Equal(transitionTime) {
		t.Errorf("LastTransitionTime = %v, want %v", ins.LastTransitionTime, transitionTime)
	}
	if ins.AdditionalInfo["docs"] == "" || len(ins.DeprecationDetails) != 1 || len(ins.AddonCompatibility) != 1 {
		t.Errorf("Insights[0] detail fields = %+v, want additional/deprecation/add-on details", ins)
	}
	if snap.Insights[1].Status != "PASSING" {
		t.Errorf("Insights[1].Status = %q, want PASSING inventory", snap.Insights[1].Status)
	}

	if len(snap.Addons) != 1 {
		t.Fatalf("Addons = %d, want 1", len(snap.Addons))
	}
	addon := snap.Addons[0]
	if addon.Name != "vpc-cni" || addon.CurrentVersion != "v1.15.0-eksbuild.1" {
		t.Errorf("Addons[0] = %+v, unexpected values", addon)
	}
	if len(addon.CompatibleVersions) != 2 {
		t.Errorf("CompatibleVersions = %v, want 2 entries", addon.CompatibleVersions)
	}

	if len(snap.Nodegroups) != 1 {
		t.Fatalf("Nodegroups = %d, want 1", len(snap.Nodegroups))
	}
	ng := snap.Nodegroups[0]
	if ng.Name != "ng-app" || ng.Status != "ACTIVE" || ng.Version != "1.32" || ng.ReleaseVersion != "1.32.7-20260601" {
		t.Errorf("Nodegroups[0] = %+v, unexpected identity/version fields", ng)
	}
	if ng.AMIType != "AL2023_x86_64_STANDARD" || ng.CapacityType != "ON_DEMAND" || !ng.LaunchTemplate {
		t.Errorf("Nodegroups[0] = %+v, unexpected AMI/capacity/launch-template fields", ng)
	}
	if ng.DesiredSize == nil || *ng.DesiredSize != 3 || ng.MinSize == nil || *ng.MinSize != 3 || ng.MaxSize == nil || *ng.MaxSize != 8 {
		t.Errorf("Nodegroups[0] scaling = desired:%v min:%v max:%v, want 3/3/8", ng.DesiredSize, ng.MinSize, ng.MaxSize)
	}
	if ng.MaxUnavailable == nil || *ng.MaxUnavailable != 1 {
		t.Errorf("Nodegroups[0].MaxUnavailable = %v, want 1", ng.MaxUnavailable)
	}
	if len(ng.HealthIssues) != 1 || ng.HealthIssues[0].Code != "AccessDenied" || len(ng.HealthIssues[0].ResourceIDs) != 1 {
		t.Errorf("Nodegroups[0].HealthIssues = %+v, unexpected health issues", ng.HealthIssues)
	}
	if len(ng.AutoScalingGroups) != 1 || ng.AutoScalingGroups[0] != "eks-ng-app-asg" {
		t.Errorf("Nodegroups[0].AutoScalingGroups = %+v, want eks-ng-app-asg", ng.AutoScalingGroups)
	}
	if ng.ReadinessStatus != "Review required" {
		t.Errorf("Nodegroups[0].ReadinessStatus = %q, want Review required", ng.ReadinessStatus)
	}

	if len(snap.Subnets) != 2 {
		t.Fatalf("Subnets = %d, want 2", len(snap.Subnets))
	}
}

func TestCollector_Collect_KeepsAddonInventoryWhenDescribeAddonFails(t *testing.T) {
	eksClient := &fakeEKSClient{
		listInsightsOut: &eks.ListInsightsOutput{},
		listAddonsOut:   &eks.ListAddonsOutput{Addons: []string{"coredns"}},
		describeAddonErr: map[string]error{
			"coredns": errors.New("access denied"),
		},
		describeAddonVersionsErr: map[string]error{
			"coredns": errors.New("not attempted"),
		},
		listNodegroupsOut: &eks.ListNodegroupsOutput{},
	}
	c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "prod")

	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.Addons) != 1 {
		t.Fatalf("Addons = %d, want one installed add-on retained for ADDON-002: %+v", len(snap.Addons), snap.Addons)
	}
	if snap.Addons[0].Name != "coredns" || snap.Addons[0].CurrentVersion != "" {
		t.Fatalf("Addons[0] = %+v, want coredns with unknown current version", snap.Addons[0])
	}
	if snap.Errors["describe-addon:coredns"] == nil {
		t.Fatalf("Errors = %+v, want describe-addon:coredns error", snap.Errors)
	}
}

// TestCollector_Collect_ClusterMetadata guards the EKS provider-depth
// enrichment fields (PlatformVersion, Status, SupportType, ARN, Region) —
// all sourced from the same DescribeCluster call the collector already
// makes for ClusterVersion/VpcID/EndpointAccess, so no new AWS permission
// is required for any of them.
func TestCollector_Collect_ClusterMetadata(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				Version:         awssdk.String("1.29"),
				PlatformVersion: awssdk.String("eks.5"),
				Status:          ekstypes.ClusterStatusActive,
				Arn:             awssdk.String("arn:aws:eks:ap-south-1:123456789012:cluster/my-cluster"),
				UpgradePolicy:   &ekstypes.UpgradePolicyResponse{SupportType: ekstypes.SupportTypeExtended},
			},
		},
		listInsightsOut: &eks.ListInsightsOutput{},
		listAddonsOut:   &eks.ListAddonsOutput{},
	}
	ec2Client := &fakeEC2Client{}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	c.Region = "ap-south-1"
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if snap.PlatformVersion != "eks.5" {
		t.Errorf("PlatformVersion = %q, want eks.5", snap.PlatformVersion)
	}
	if snap.Status != "ACTIVE" {
		t.Errorf("Status = %q, want ACTIVE", snap.Status)
	}
	if snap.SupportType != "EXTENDED" {
		t.Errorf("SupportType = %q, want EXTENDED", snap.SupportType)
	}
	if snap.ARN != "arn:aws:eks:ap-south-1:123456789012:cluster/my-cluster" {
		t.Errorf("ARN = %q, unexpected value", snap.ARN)
	}
	if snap.Region != "ap-south-1" {
		t.Errorf("Region = %q, want ap-south-1 (from Collector.Region, not an AWS call)", snap.Region)
	}
}

// TestCollector_Collect_ClusterMetadata_MissingFieldsStayEmpty guards
// against a nil UpgradePolicy (a cluster created before EKS extended
// support existed) causing a nil-pointer panic instead of just leaving
// SupportType empty.
func TestCollector_Collect_ClusterMetadata_MissingFieldsStayEmpty(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{Version: awssdk.String("1.29")},
		},
		listInsightsOut: &eks.ListInsightsOutput{},
		listAddonsOut:   &eks.ListAddonsOutput{},
	}
	c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if snap.SupportType != "" {
		t.Errorf("SupportType = %q, want empty when UpgradePolicy is nil", snap.SupportType)
	}
	if snap.PlatformVersion != "" || snap.ARN != "" {
		t.Errorf("PlatformVersion/ARN = %q/%q, want empty when not set", snap.PlatformVersion, snap.ARN)
	}
}

func TestCollector_Collect_PartialFailureRecordedNotFatal(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{Version: awssdk.String("1.29")},
		},
		listInsightsErr: errors.New("access denied"),
		listAddonsOut:   &eks.ListAddonsOutput{},
	}
	ec2Client := &fakeEC2Client{}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect must not return a hard error on partial failure: %v", err)
	}
	if snap.Errors["list-insights"] == nil {
		t.Errorf("expected list-insights error to be recorded, got: %v", snap.Errors)
	}
	if snap.ClusterVersion != "1.29" {
		t.Errorf("ClusterVersion should still be populated despite the Insights failure, got %q", snap.ClusterVersion)
	}
}

func TestCollector_Collect_InsightsFallbacksToCategoryOnlyWhenTargetFilterFails(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{Version: awssdk.String("1.29")},
		},
		listInsightsVersionFilterErr: errors.New("unsupported kubernetes version"),
		listInsightsFallbackOut: &eks.ListInsightsOutput{
			Insights: []ekstypes.InsightSummary{{
				Id: awssdk.String("insight-1"), Name: awssdk.String("Deprecated API usage"),
				Category: ekstypes.CategoryUpgradeReadiness, KubernetesVersion: awssdk.String("1.33"),
				InsightStatus: &ekstypes.InsightStatus{Status: ekstypes.InsightStatusValueWarning},
			}},
		},
		listAddonsOut: &eks.ListAddonsOutput{},
	}

	c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("fallback success should not mark insights unavailable, got errors: %v", snap.Errors)
	}
	if len(snap.Insights) != 1 || snap.Insights[0].KubernetesVersion != "1.33" {
		t.Fatalf("Insights = %+v, want fallback category-only insight with response version preserved", snap.Insights)
	}
	if len(eksClient.listInsightsInputs) != 2 {
		t.Fatalf("ListInsights calls = %d, want version-filtered call plus category-only fallback", len(eksClient.listInsightsInputs))
	}
	if len(eksClient.listInsightsInputs[0].Filter.KubernetesVersions) != 1 {
		t.Fatalf("first ListInsights call should include target filter: %+v", eksClient.listInsightsInputs[0].Filter)
	}
	if len(eksClient.listInsightsInputs[1].Filter.KubernetesVersions) != 0 {
		t.Fatalf("fallback ListInsights call should omit target filter: %+v", eksClient.listInsightsInputs[1].Filter)
	}
}

func TestCollector_Collect_InsightsPagination(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{Version: awssdk.String("1.29")},
		},
		listInsightsOutByToken: map[string]*eks.ListInsightsOutput{
			"": {
				Insights: []ekstypes.InsightSummary{{
					Id: awssdk.String("insight-1"), Name: awssdk.String("First"),
					Category:      ekstypes.CategoryUpgradeReadiness,
					InsightStatus: &ekstypes.InsightStatus{Status: ekstypes.InsightStatusValuePassing},
				}},
				NextToken: awssdk.String("page-2"),
			},
			"page-2": {
				Insights: []ekstypes.InsightSummary{{
					Id: awssdk.String("insight-2"), Name: awssdk.String("Second"),
					Category:      ekstypes.CategoryUpgradeReadiness,
					InsightStatus: &ekstypes.InsightStatus{Status: ekstypes.InsightStatusValueUnknown},
				}},
			},
		},
		listAddonsOut: &eks.ListAddonsOutput{},
	}

	c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Insights) != 2 {
		t.Fatalf("Insights = %+v, want both paginated insights", snap.Insights)
	}
	if len(eksClient.listInsightsInputs) != 2 || awssdk.ToString(eksClient.listInsightsInputs[1].NextToken) != "page-2" {
		t.Fatalf("ListInsights pagination calls = %+v, want second call with page-2 token", eksClient.listInsightsInputs)
	}
}

func clusterWithNetworkConfig(sgIDs []string, clusterSG, vpcID string) *eks.DescribeClusterOutput {
	return &eks.DescribeClusterOutput{
		Cluster: &ekstypes.Cluster{
			Version: awssdk.String("1.29"),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				VpcId:                  awssdk.String(vpcID),
				SecurityGroupIds:       sgIDs,
				ClusterSecurityGroupId: awssdk.String(clusterSG),
			},
		},
	}
}

func TestCollector_Collect_NetworkPreflight_AllResourcesExist(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: clusterWithNetworkConfig([]string{"sg-extra"}, "sg-cluster", "vpc-123"),
		listAddonsOut:      &eks.ListAddonsOutput{},
		listInsightsOut:    &eks.ListInsightsOutput{},
	}
	ec2Client := &fakeEC2Client{}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.NetworkPreflightIssues) != 0 {
		t.Errorf("NetworkPreflightIssues = %+v, want none (both SGs and VPC exist)", snap.NetworkPreflightIssues)
	}
	if len(snap.Errors) != 0 {
		t.Errorf("unexpected collector errors: %v", snap.Errors)
	}
}

func TestCollector_Collect_NetworkPreflight_MissingSecurityGroup(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: clusterWithNetworkConfig(nil, "sg-deleted", "vpc-123"),
		listAddonsOut:      &eks.ListAddonsOutput{},
		listInsightsOut:    &eks.ListInsightsOutput{},
	}
	ec2Client := &fakeEC2Client{
		describeSecurityGroupsErr: map[string]error{"sg-deleted": awsNotFoundError("InvalidGroup.NotFound")},
	}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.NetworkPreflightIssues) != 1 {
		t.Fatalf("NetworkPreflightIssues = %+v, want exactly 1", snap.NetworkPreflightIssues)
	}
	issue := snap.NetworkPreflightIssues[0]
	if issue.Kind != "SecurityGroup" || issue.ID != "sg-deleted" {
		t.Errorf("issue = %+v, want SecurityGroup/sg-deleted", issue)
	}
	if len(snap.Errors) != 0 {
		t.Errorf("a NotFound must be recorded as an issue, not a collector error: %v", snap.Errors)
	}
}

func TestCollector_Collect_NetworkPreflight_MissingVpc(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: clusterWithNetworkConfig(nil, "sg-cluster", "vpc-deleted"),
		listAddonsOut:      &eks.ListAddonsOutput{},
		listInsightsOut:    &eks.ListInsightsOutput{},
	}
	ec2Client := &fakeEC2Client{
		describeVpcsErr: map[string]error{"vpc-deleted": awsNotFoundError("InvalidVpcID.NotFound")},
	}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.NetworkPreflightIssues) != 1 {
		t.Fatalf("NetworkPreflightIssues = %+v, want exactly 1", snap.NetworkPreflightIssues)
	}
	issue := snap.NetworkPreflightIssues[0]
	if issue.Kind != "Vpc" || issue.ID != "vpc-deleted" {
		t.Errorf("issue = %+v, want Vpc/vpc-deleted", issue)
	}
}

// TestCollector_Collect_NetworkPreflight_NonNotFoundErrorRecordedSeparately
// guards an important distinction: a permissions/throttling error on
// DescribeSecurityGroups must be recorded as a collector error, not
// misread as "the security group doesn't exist" — those are very
// different facts and conflating them would produce a false positive.
func TestCollector_Collect_NetworkPreflight_NonNotFoundErrorRecordedSeparately(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: clusterWithNetworkConfig(nil, "sg-cluster", "vpc-123"),
		listAddonsOut:      &eks.ListAddonsOutput{},
		listInsightsOut:    &eks.ListInsightsOutput{},
	}
	ec2Client := &fakeEC2Client{
		describeSecurityGroupsErr: map[string]error{"sg-cluster": awsNotFoundError("UnauthorizedOperation")},
	}

	c := awscol.NewCollector(eksClient, ec2Client, "my-cluster")
	snap, err := c.Collect(context.Background(), time.Second, "1.34")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.NetworkPreflightIssues) != 0 {
		t.Errorf("a non-NotFound error must not be recorded as a NetworkPreflightIssue: %+v", snap.NetworkPreflightIssues)
	}
	if snap.Errors["describe-security-group:sg-cluster"] == nil {
		t.Errorf("expected the permissions error to be recorded under describe-security-group:sg-cluster, got: %v", snap.Errors)
	}
}

// TestCollector_DescribeClusterVersion guards the plan command's
// --from-version=auto discovery path for --provider=eks runs.
func TestCollector_DescribeClusterVersion(t *testing.T) {
	eksClient := &fakeEKSClient{
		describeClusterOut: &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{Version: awssdk.String("1.29")},
		},
	}
	c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "my-cluster")

	got, err := c.DescribeClusterVersion(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("DescribeClusterVersion: %v", err)
	}
	if got != "1.29" {
		t.Errorf("DescribeClusterVersion() = %q, want %q", got, "1.29")
	}
}

func TestCollector_DescribeClusterVersion_APIError(t *testing.T) {
	wantErr := errors.New("access denied")
	eksClient := &fakeEKSClient{describeClusterErr: wantErr}
	c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "my-cluster")

	if _, err := c.DescribeClusterVersion(context.Background(), time.Second); err == nil {
		t.Fatal("DescribeClusterVersion succeeded, want the API error to propagate")
	}
}

func TestCollector_DescribeClusterVersion_NilClusterOrVersion(t *testing.T) {
	tests := []struct {
		name string
		out  *eks.DescribeClusterOutput
	}{
		{name: "nil Cluster", out: &eks.DescribeClusterOutput{}},
		{name: "nil Cluster.Version", out: &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eksClient := &fakeEKSClient{describeClusterOut: tc.out}
			c := awscol.NewCollector(eksClient, &fakeEC2Client{}, "my-cluster")
			if _, err := c.DescribeClusterVersion(context.Background(), time.Second); err == nil {
				t.Fatal("DescribeClusterVersion succeeded, want error for a cluster with no reported version")
			}
		})
	}
}
