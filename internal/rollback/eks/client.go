package eks

import (
	"context"

	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
)

// Client captures only the read-only EKS operations rollback eligibility needs.
// The real *eks.Client satisfies this interface structurally.
type Client interface {
	DescribeCluster(ctx context.Context, params *awseks.DescribeClusterInput, optFns ...func(*awseks.Options)) (*awseks.DescribeClusterOutput, error)
	ListUpdates(ctx context.Context, params *awseks.ListUpdatesInput, optFns ...func(*awseks.Options)) (*awseks.ListUpdatesOutput, error)
	DescribeUpdate(ctx context.Context, params *awseks.DescribeUpdateInput, optFns ...func(*awseks.Options)) (*awseks.DescribeUpdateOutput, error)
	DescribeClusterVersions(ctx context.Context, params *awseks.DescribeClusterVersionsInput, optFns ...func(*awseks.Options)) (*awseks.DescribeClusterVersionsOutput, error)
}
