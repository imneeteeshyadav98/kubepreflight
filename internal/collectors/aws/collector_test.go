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

	awscol "kubepreflight/internal/collectors/aws"
)

// fakeEKSClient is a hand-rolled fake — no real AWS calls in this test
// suite. It implements exactly the awscol.EKSClient interface.
type fakeEKSClient struct {
	describeClusterOut *eks.DescribeClusterOutput
	describeClusterErr error

	listInsightsOut *eks.ListInsightsOutput
	listInsightsErr error

	describeInsightOut map[string]*eks.DescribeInsightOutput // keyed by insight ID
	describeInsightErr map[string]error

	listAddonsOut *eks.ListAddonsOutput
	listAddonsErr error

	describeAddonOut map[string]*eks.DescribeAddonOutput // keyed by addon name
	describeAddonErr map[string]error

	describeAddonVersionsOut map[string]*eks.DescribeAddonVersionsOutput // keyed by addon name
	describeAddonVersionsErr map[string]error
}

func (f *fakeEKSClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return f.describeClusterOut, f.describeClusterErr
}

func (f *fakeEKSClient) ListInsights(ctx context.Context, params *eks.ListInsightsInput, optFns ...func(*eks.Options)) (*eks.ListInsightsOutput, error) {
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
			"insight-1": {Insight: &ekstypes.Insight{Recommendation: awssdk.String("Migrate off PodSecurityPolicy")}},
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
	snap, err := c.Collect(context.Background(), "1.34")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
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

	if len(snap.Insights) != 1 {
		t.Fatalf("Insights = %d, want 1 (PASSING must be filtered)", len(snap.Insights))
	}
	ins := snap.Insights[0]
	if ins.ID != "insight-1" || ins.Status != "ERROR" || ins.Recommendation != "Migrate off PodSecurityPolicy" {
		t.Errorf("Insights[0] = %+v, unexpected values", ins)
	}
	if !ins.LastRefreshTime.Equal(refreshTime) {
		t.Errorf("LastRefreshTime = %v, want %v", ins.LastRefreshTime, refreshTime)
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

	if len(snap.Subnets) != 2 {
		t.Fatalf("Subnets = %d, want 2", len(snap.Subnets))
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
	snap, err := c.Collect(context.Background(), "1.34")
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
	snap, err := c.Collect(context.Background(), "1.34")
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
	snap, err := c.Collect(context.Background(), "1.34")
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
	snap, err := c.Collect(context.Background(), "1.34")
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
	snap, err := c.Collect(context.Background(), "1.34")
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
