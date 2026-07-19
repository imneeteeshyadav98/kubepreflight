package aws_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
)

// hangingEKSClient blocks every operation until its context is done,
// simulating a black-holed AWS endpoint -- the same class of gap PR #95
// found and fixed in the k8s collector (ServerVersion's unbounded
// context.TODO() call). Unlike client-go's DiscoveryInterface, which
// hardcodes context.TODO() internally and can't be bounded at all,
// aws-sdk-go-v2's smithy-based clients properly thread context through to
// the underlying http.Client -- these fakes model that correctly (they
// respect ctx, a real network call would too), so what's under test here is
// purely whether Collect's per-call context.WithTimeout wrapping (see
// callWithTimeout) actually bounds each call, not whether the SDK itself
// respects context.
type hangingEKSClient struct{}

func (hangingEKSClient) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) ListInsights(ctx context.Context, params *eks.ListInsightsInput, optFns ...func(*eks.Options)) (*eks.ListInsightsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) DescribeInsight(ctx context.Context, params *eks.DescribeInsightInput, optFns ...func(*eks.Options)) (*eks.DescribeInsightOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) ListAddons(ctx context.Context, params *eks.ListAddonsInput, optFns ...func(*eks.Options)) (*eks.ListAddonsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) DescribeAddon(ctx context.Context, params *eks.DescribeAddonInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) DescribeAddonVersions(ctx context.Context, params *eks.DescribeAddonVersionsInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonVersionsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEKSClient) DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// hangingEC2Client is hangingEKSClient's EC2 counterpart.
type hangingEC2Client struct{}

func (hangingEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestCollector_Collect_FullyUnreachableAWSFinishesBoundedByTimeout(t *testing.T) {
	c := awscol.NewCollector(hangingEKSClient{}, hangingEC2Client{}, "demo-cluster")

	start := time.Now()
	snap, err := c.Collect(context.Background(), 20*time.Millisecond, "1.34")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	// Against a fully unreachable AWS endpoint, Collect makes exactly 4
	// top-level calls before giving up on each operation (DescribeCluster,
	// ListInsights, ListAddons, ListNodegroups) -- there's nothing to fan
	// out into per-item Describe calls since every List call itself times
	// out, and DescribeSubnets/DescribeSecurityGroups/DescribeVpcs never
	// run at all since DescribeCluster never returns VPC config. Confirmed
	// bounded well under (4 calls) * 20ms plus scheduling slack, not
	// hanging indefinitely.
	if elapsed > 2*time.Second {
		t.Fatalf("Collect took %s, want it bounded near 4 * 20ms per-call timeouts", elapsed)
	}

	wantKeys := []string{"describe-cluster", "list-insights", "list-addons", "list-nodegroups"}
	for _, key := range wantKeys {
		got, ok := snap.Errors[key]
		if !ok {
			t.Errorf("Errors[%q] not set, want the timeout recorded", key)
			continue
		}
		if !errors.Is(got, context.DeadlineExceeded) {
			t.Errorf("Errors[%q] = %v, want context.DeadlineExceeded", key, got)
		}
	}
	if len(snap.Subnets) != 0 || len(snap.NetworkPreflightIssues) != 0 {
		t.Errorf("Subnets/NetworkPreflightIssues should be empty when DescribeCluster never returns VPC config, got %+v / %+v", snap.Subnets, snap.NetworkPreflightIssues)
	}
}

func TestCollector_Collect_OneHungCallDoesNotStarveTheRest(t *testing.T) {
	// Only DescribeCluster hangs; every other operation is a normal fast
	// fake. This is the per-call-budget claim itself: one slow/hung call
	// must not block the calls that come after it in the same Collect.
	fastEKS := &fakeEKSClient{
		listInsightsOut:   &eks.ListInsightsOutput{},
		listAddonsOut:     &eks.ListAddonsOutput{},
		listNodegroupsOut: &eks.ListNodegroupsOutput{},
	}
	c := awscol.NewCollector(hangingDescribeClusterOnly{fastEKS}, &fakeEC2Client{}, "demo-cluster")

	start := time.Now()
	snap, err := c.Collect(context.Background(), 20*time.Millisecond, "1.34")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Collect took %s, want it bounded near a single 20ms per-call timeout", elapsed)
	}
	if _, ok := snap.Errors["describe-cluster"]; !ok {
		t.Fatal(`Errors["describe-cluster"] not set, want the timeout recorded`)
	}
	if _, ok := snap.Errors["list-insights"]; ok {
		t.Error(`Errors["list-insights"] set, want the fast fake to have succeeded despite DescribeCluster hanging`)
	}
}

// hangingDescribeClusterOnly wraps a real fake EKSClient but blocks on
// DescribeCluster specifically, so every other operation runs its normal,
// fast fake behavior.
type hangingDescribeClusterOnly struct {
	*fakeEKSClient
}

func (hangingDescribeClusterOnly) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
