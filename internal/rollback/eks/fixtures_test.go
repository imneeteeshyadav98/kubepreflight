package eks

import (
	"context"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"kubepreflight/internal/rollback"
)

type fakeClient struct {
	describeClusterOut *awseks.DescribeClusterOutput
	describeClusterErr error

	listUpdatesOutByToken map[string]*awseks.ListUpdatesOutput
	listUpdatesOut        *awseks.ListUpdatesOutput
	listUpdatesErr        error
	listUpdatesInputs     []*awseks.ListUpdatesInput

	describeUpdateOut map[string]*awseks.DescribeUpdateOutput
	describeUpdateErr map[string]error

	describeClusterVersionsOutByToken map[string]*awseks.DescribeClusterVersionsOutput
	describeClusterVersionsOut        *awseks.DescribeClusterVersionsOutput
	describeClusterVersionsErr        error
	describeClusterVersionsInputs     []*awseks.DescribeClusterVersionsInput
}

func (f *fakeClient) DescribeCluster(ctx context.Context, params *awseks.DescribeClusterInput, optFns ...func(*awseks.Options)) (*awseks.DescribeClusterOutput, error) {
	return f.describeClusterOut, f.describeClusterErr
}

func (f *fakeClient) ListUpdates(ctx context.Context, params *awseks.ListUpdatesInput, optFns ...func(*awseks.Options)) (*awseks.ListUpdatesOutput, error) {
	f.listUpdatesInputs = append(f.listUpdatesInputs, params)
	if f.listUpdatesOutByToken != nil {
		return f.listUpdatesOutByToken[awssdk.ToString(params.NextToken)], f.listUpdatesErr
	}
	return f.listUpdatesOut, f.listUpdatesErr
}

func (f *fakeClient) DescribeUpdate(ctx context.Context, params *awseks.DescribeUpdateInput, optFns ...func(*awseks.Options)) (*awseks.DescribeUpdateOutput, error) {
	id := awssdk.ToString(params.UpdateId)
	if f.describeUpdateErr != nil {
		if err, ok := f.describeUpdateErr[id]; ok {
			return nil, err
		}
	}
	return f.describeUpdateOut[id], nil
}

func (f *fakeClient) DescribeClusterVersions(ctx context.Context, params *awseks.DescribeClusterVersionsInput, optFns ...func(*awseks.Options)) (*awseks.DescribeClusterVersionsOutput, error) {
	f.describeClusterVersionsInputs = append(f.describeClusterVersionsInputs, params)
	if f.describeClusterVersionsOutByToken != nil {
		return f.describeClusterVersionsOutByToken[awssdk.ToString(params.NextToken)], f.describeClusterVersionsErr
	}
	return f.describeClusterVersionsOut, f.describeClusterVersionsErr
}

func TestCollectorCollectsRollbackEligibilityEvidence(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	createdAt := now.Add(-48 * time.Hour)
	client := healthyFakeClient(createdAt)
	client.listUpdatesOutByToken = map[string]*awseks.ListUpdatesOutput{
		"":       {UpdateIds: []string{"ignored-config"}, NextToken: awssdk.String("page-2")},
		"page-2": {UpdateIds: []string{"upgrade-1"}},
	}
	client.listUpdatesOut = nil
	client.describeUpdateOut["ignored-config"] = updateOutput("ignored-config", ekstypes.UpdateTypeConfigUpdate, ekstypes.UpdateStatusSuccessful, createdAt.Add(-time.Hour), "")

	c := NewCollector(client, "prod", "ap-south-1")
	snap, err := c.Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if snap.ClusterName != "prod" || snap.Region != "ap-south-1" || snap.CurrentVersion != "1.35" || snap.ClusterStatus != "ACTIVE" {
		t.Fatalf("snapshot cluster fields = %+v", snap)
	}
	if len(client.listUpdatesInputs) != 2 {
		t.Fatalf("ListUpdates calls = %d, want pagination", len(client.listUpdatesInputs))
	}
	if len(snap.Updates) != 2 {
		t.Fatalf("Updates = %+v, want both described updates", snap.Updates)
	}
	if len(snap.ClusterVersions) != 2 {
		t.Fatalf("ClusterVersions = %+v, want version support inventory", snap.ClusterVersions)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("Errors = %+v, want none", snap.Errors)
	}
}

func TestEvaluateEligibilityEligibleWithinRollbackWindow(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap, err := NewCollector(healthyFakeClient(now.Add(-24*time.Hour)), "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if err := assessment.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if assessment.Eligibility.Status != rollback.EligibilityEligible {
		t.Fatalf("Eligibility = %q, want eligible: %+v", assessment.Eligibility.Status, assessment)
	}
	if assessment.Readiness.Status != rollback.ReadinessReady {
		t.Fatalf("Readiness = %q, want ready", assessment.Readiness.Status)
	}
	if assessment.Cluster.RollbackTargetVersion != "1.34" {
		t.Fatalf("RollbackTargetVersion = %q, want 1.34", assessment.Cluster.RollbackTargetVersion)
	}
	if assessment.Eligibility.WindowExpiresAt == nil || assessment.Eligibility.RemainingMinutes == nil {
		t.Fatalf("rollback window not populated: %+v", assessment.Eligibility)
	}
	if assessment.Recommendation.Decision == rollback.RecommendationRollbackPreferred {
		t.Fatal("eligibility-only PR must not prefer rollback")
	}
}

func TestEvaluateEligibilityListUpdatesFailureIsUnknown(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.listUpdatesErr = errors.New("access denied")

	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	assessment := EvaluateEligibility(snap, now)

	if assessment.Eligibility.Status != rollback.EligibilityUnknown {
		t.Fatalf("Eligibility = %q, want unknown", assessment.Eligibility.Status)
	}
	if assessment.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient evidence", assessment.Readiness.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonEKSUpgradeHistoryUnavailable) {
		t.Fatalf("ReasonCodes = %v, want EKS upgrade history unavailable", assessment.Eligibility.ReasonCodes)
	}
}

func TestEvaluateEligibilityDescribeUpdateFailureIsUnknown(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.describeUpdateErr = map[string]error{"upgrade-1": errors.New("access denied")}

	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	assessment := EvaluateEligibility(snap, now)

	if assessment.Eligibility.Status != rollback.EligibilityUnknown {
		t.Fatalf("Eligibility = %q, want unknown", assessment.Eligibility.Status)
	}
	if assessment.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient evidence", assessment.Readiness.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonEKSUpgradeHistoryUnavailable) {
		t.Fatalf("ReasonCodes = %v, want EKS upgrade history unavailable", assessment.Eligibility.ReasonCodes)
	}
}

func TestEvaluateEligibilityExpiredWindowIsUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap, err := NewCollector(healthyFakeClient(now.Add(-(8*24)*time.Hour)), "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	assessment := EvaluateEligibility(snap, now)

	if assessment.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("Eligibility = %q, want unavailable", assessment.Eligibility.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonRollbackWindowExpired) {
		t.Fatalf("ReasonCodes = %v, want rollback window expired", assessment.Eligibility.ReasonCodes)
	}
	if assessment.Readiness.Status != rollback.ReadinessBlocked || assessment.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("unexpected decision layers: readiness=%q recommendation=%q", assessment.Readiness.Status, assessment.Recommendation.Decision)
	}
}

func TestEvaluateEligibilityInactiveClusterIsUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.describeClusterOut.Cluster.Status = ekstypes.ClusterStatusUpdating
	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if assessment.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("Eligibility = %q, want unavailable", assessment.Eligibility.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonClusterNotActive) {
		t.Fatalf("ReasonCodes = %v, want cluster not active", assessment.Eligibility.ReasonCodes)
	}
}

func TestEvaluateEligibilityUnsupportedTargetIsUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.describeClusterVersionsOut.ClusterVersions = []ekstypes.ClusterVersionInformation{
		{ClusterVersion: awssdk.String("1.34"), VersionStatus: ekstypes.VersionStatusUnsupported},
	}
	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if assessment.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("Eligibility = %q, want unavailable", assessment.Eligibility.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonRollbackTargetUnsupported) {
		t.Fatalf("ReasonCodes = %v, want target unsupported", assessment.Eligibility.ReasonCodes)
	}
}

func TestEvaluateEligibilityMissingUpgradeHistoryIsUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.listUpdatesOut = &awseks.ListUpdatesOutput{UpdateIds: []string{"config-1"}}
	client.describeUpdateOut = map[string]*awseks.DescribeUpdateOutput{
		"config-1": updateOutput("config-1", ekstypes.UpdateTypeConfigUpdate, ekstypes.UpdateStatusSuccessful, now.Add(-24*time.Hour), ""),
	}
	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	assessment := EvaluateEligibility(snap, now)
	if assessment.Eligibility.Status != rollback.EligibilityUnavailable {
		t.Fatalf("Eligibility = %q, want unavailable", assessment.Eligibility.Status)
	}
	if !hasReason(assessment.Eligibility.ReasonCodes, rollback.ReasonEKSUpgradeHistoryUnavailable) {
		t.Fatalf("ReasonCodes = %v, want missing upgrade history", assessment.Eligibility.ReasonCodes)
	}
}

func healthyFakeClient(upgradeCreatedAt time.Time) *fakeClient {
	return &fakeClient{
		describeClusterOut: &awseks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				Name:    awssdk.String("prod"),
				Version: awssdk.String("1.35"),
				Status:  ekstypes.ClusterStatusActive,
				UpgradePolicy: &ekstypes.UpgradePolicyResponse{
					SupportType: ekstypes.SupportTypeStandard,
				},
			},
		},
		listUpdatesOut: &awseks.ListUpdatesOutput{UpdateIds: []string{"upgrade-1"}},
		describeUpdateOut: map[string]*awseks.DescribeUpdateOutput{
			"upgrade-1": updateOutput("upgrade-1", ekstypes.UpdateTypeVersionUpdate, ekstypes.UpdateStatusSuccessful, upgradeCreatedAt, "1.35"),
		},
		describeClusterVersionsOut: &awseks.DescribeClusterVersionsOutput{
			ClusterVersions: []ekstypes.ClusterVersionInformation{
				{ClusterVersion: awssdk.String("1.34"), VersionStatus: ekstypes.VersionStatusStandardSupport},
				{ClusterVersion: awssdk.String("1.35"), VersionStatus: ekstypes.VersionStatusStandardSupport},
			},
		},
	}
}

func updateOutput(id string, updateType ekstypes.UpdateType, status ekstypes.UpdateStatus, createdAt time.Time, version string) *awseks.DescribeUpdateOutput {
	update := &ekstypes.Update{
		Id:        awssdk.String(id),
		Type:      updateType,
		Status:    status,
		CreatedAt: &createdAt,
	}
	if version != "" {
		update.Params = []ekstypes.UpdateParam{{
			Type:  ekstypes.UpdateParamTypeVersion,
			Value: awssdk.String(version),
		}}
	}
	return &awseks.DescribeUpdateOutput{Update: update}
}

func hasReason(reasons []rollback.ReasonCode, want rollback.ReasonCode) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
