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
}

func (f *fakeEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return f.describeSubnetsOut, f.describeSubnetsErr
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
