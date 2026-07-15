package eks

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
)

// LoadCollector loads AWS credentials through the standard AWS SDK chain and
// returns a read-only rollback evidence collector.
func LoadCollector(ctx context.Context, clusterName string) (*Collector, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
		return nil, fmt.Errorf(
			"no AWS credentials found — configure them via `aws configure`, the AWS_PROFILE environment variable, "+
				"or an IAM role before using rollback assessment for EKS (SDK detail: %v)", err)
	}
	return NewCollector(awseks.NewFromConfig(cfg), clusterName, cfg.Region), nil
}
