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

	listInsightsOutByToken map[string]*awseks.ListInsightsOutput
	listInsightsOut        *awseks.ListInsightsOutput
	listInsightsErr        error
	listInsightsInputs     []*awseks.ListInsightsInput

	describeInsightOut map[string]*awseks.DescribeInsightOutput
	describeInsightErr map[string]error

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

func (f *fakeClient) ListInsights(ctx context.Context, params *awseks.ListInsightsInput, optFns ...func(*awseks.Options)) (*awseks.ListInsightsOutput, error) {
	f.listInsightsInputs = append(f.listInsightsInputs, params)
	if f.listInsightsOutByToken != nil {
		return f.listInsightsOutByToken[awssdk.ToString(params.NextToken)], f.listInsightsErr
	}
	return f.listInsightsOut, f.listInsightsErr
}

func (f *fakeClient) DescribeInsight(ctx context.Context, params *awseks.DescribeInsightInput, optFns ...func(*awseks.Options)) (*awseks.DescribeInsightOutput, error) {
	id := awssdk.ToString(params.Id)
	if f.describeInsightErr != nil {
		if err, ok := f.describeInsightErr[id]; ok {
			return nil, err
		}
	}
	return f.describeInsightOut[id], nil
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
	if len(client.listInsightsInputs) != 1 {
		t.Fatalf("ListInsights calls = %d, want rollback readiness collection", len(client.listInsightsInputs))
	}
	insightFilter := client.listInsightsInputs[0].Filter
	if insightFilter == nil || len(insightFilter.Categories) != 1 || insightFilter.Categories[0] != ekstypes.CategoryRollbackReadiness {
		t.Fatalf("ListInsights filter = %+v, want ROLLBACK_READINESS", insightFilter)
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
	if assessment.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %q, want insufficient evidence until rollback insights are applied", assessment.Readiness.Status)
	}
	if assessment.Cluster.RollbackTargetVersion != "1.34" {
		t.Fatalf("RollbackTargetVersion = %q, want 1.34", assessment.Cluster.RollbackTargetVersion)
	}
	if assessment.Eligibility.WindowExpiresAt == nil || assessment.Eligibility.RemainingMinutes == nil {
		t.Fatalf("rollback window not populated: %+v", assessment.Eligibility)
	}
	if assessment.Evidence.WindowCalculation != "conservative" || assessment.Evidence.TimestampSource != "eks_update_created_at" {
		t.Fatalf("window evidence = %+v, want conservative createdAt basis", assessment.Evidence)
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

func TestCollectorCollectsRollbackInsightsWithDetailsAndPagination(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	client := healthyFakeClient(now.Add(-24 * time.Hour))
	client.listInsightsOut = nil
	client.listInsightsOutByToken = map[string]*awseks.ListInsightsOutput{
		"":       {Insights: []ekstypes.InsightSummary{insightSummary("passing", "Passing check", ekstypes.InsightStatusValuePassing, now.Add(-time.Hour))}, NextToken: awssdk.String("page-2")},
		"page-2": {Insights: []ekstypes.InsightSummary{insightSummary("warning", "Warning check", ekstypes.InsightStatusValueWarning, now.Add(-2*time.Hour))}},
	}
	client.describeInsightOut = map[string]*awseks.DescribeInsightOutput{
		"passing": insightOutput("passing", "Passing check detail", ekstypes.InsightStatusValuePassing, now.Add(-time.Hour)),
		"warning": insightOutput("warning", "Warning check detail", ekstypes.InsightStatusValueWarning, now.Add(-2*time.Hour)),
	}

	snap, err := NewCollector(client, "prod", "ap-south-1").Collect(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(client.listInsightsInputs) != 2 {
		t.Fatalf("ListInsights calls = %d, want pagination", len(client.listInsightsInputs))
	}
	if len(snap.Insights) != 2 {
		t.Fatalf("Insights = %+v, want both pages", snap.Insights)
	}
	if snap.Insights[1].Description != "Warning check detail" || snap.Insights[1].Recommendation != "review rollback readiness" {
		t.Fatalf("detailed insight not merged: %+v", snap.Insights[1])
	}
	if len(snap.Insights[1].Resources) != 1 || snap.Insights[1].Resources[0].KubernetesResourceURI == "" {
		t.Fatalf("insight resources not preserved: %+v", snap.Insights[1].Resources)
	}
}

func TestApplyRollbackInsightsBlockingStatuses(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap := eligibleSnapshot(now)
	snap.Insights = []InsightRecord{
		{ID: "error", Name: "Blocking insight", Status: string(ekstypes.InsightStatusValueError), LastRefreshTime: now.Add(-time.Hour)},
		{ID: "unknown", Name: "Unknown insight", Status: string(ekstypes.InsightStatusValueUnknown), LastRefreshTime: now.Add(-time.Hour)},
	}

	assessment := ApplyRollbackInsights(EvaluateEligibility(snap, now), snap, now)
	if assessment.Readiness.Status != rollback.ReadinessBlocked || assessment.Readiness.Blockers != 2 {
		t.Fatalf("Readiness = %+v, want 2 blockers", assessment.Readiness)
	}
	if assessment.Recommendation.Decision != rollback.RecommendationDoNotProceed {
		t.Fatalf("Recommendation = %q, want do not proceed", assessment.Recommendation.Decision)
	}
	if !hasReason(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSInsightsBlocking) {
		t.Fatalf("Recommendation reasons = %v, want EKS insights blocking", assessment.Recommendation.ReasonCodes)
	}
}

func TestApplyRollbackInsightsWarningIsHighRisk(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap := eligibleSnapshot(now)
	snap.Insights = []InsightRecord{
		{ID: "warning", Name: "Advisory insight", Status: string(ekstypes.InsightStatusValueWarning), LastRefreshTime: now.Add(-time.Hour)},
	}

	assessment := ApplyRollbackInsights(EvaluateEligibility(snap, now), snap, now)
	if assessment.Readiness.Status != rollback.ReadinessHighRisk || assessment.Readiness.Warnings != 1 {
		t.Fatalf("Readiness = %+v, want high risk warning", assessment.Readiness)
	}
	if assessment.Recommendation.Decision != rollback.RecommendationOperatorDecisionRequired {
		t.Fatalf("Recommendation = %q, want operator decision", assessment.Recommendation.Decision)
	}
}

func TestApplyRollbackInsightsPassingClearsFeatureUnknownButKeepsOtherUnknowns(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap := eligibleSnapshot(now)
	snap.Insights = []InsightRecord{
		{ID: "passing", Name: "Clean insight", Status: string(ekstypes.InsightStatusValuePassing), LastRefreshTime: now.Add(-time.Hour)},
	}

	assessment := ApplyRollbackInsights(EvaluateEligibility(snap, now), snap, now)
	if assessment.Readiness.Status != rollback.ReadinessInsufficientEvidence || assessment.Readiness.Unknowns != 1 {
		t.Fatalf("Readiness = %+v, want one remaining unknown for auto-upgrade origin", assessment.Readiness)
	}
	if hasReason(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSFeatureCompatibilityUnverified) {
		t.Fatalf("Recommendation reasons = %v, feature compatibility should be satisfied by fresh rollback insights", assessment.Recommendation.ReasonCodes)
	}
}

func TestApplyRollbackInsightsStaleEvidenceIsIncomplete(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap := eligibleSnapshot(now)
	snap.Insights = []InsightRecord{
		{ID: "stale", Name: "Stale insight", Status: string(ekstypes.InsightStatusValuePassing), LastRefreshTime: now.Add(-25 * time.Hour)},
	}

	assessment := ApplyRollbackInsights(EvaluateEligibility(snap, now), snap, now)
	if assessment.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %+v, want insufficient evidence", assessment.Readiness)
	}
	if !hasReason(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSInsightsStale) {
		t.Fatalf("Recommendation reasons = %v, want stale insight reason", assessment.Recommendation.ReasonCodes)
	}
}

func TestApplyRollbackInsightsUnavailableIsIncomplete(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	snap := eligibleSnapshot(now)
	snap.Errors["list-rollback-insights"] = errors.New("access denied")

	assessment := ApplyRollbackInsights(EvaluateEligibility(snap, now), snap, now)
	if assessment.Readiness.Status != rollback.ReadinessInsufficientEvidence {
		t.Fatalf("Readiness = %+v, want insufficient evidence", assessment.Readiness)
	}
	if !hasReason(assessment.Recommendation.ReasonCodes, rollback.ReasonEKSInsightsUnavailable) {
		t.Fatalf("Recommendation reasons = %v, want insights unavailable", assessment.Recommendation.ReasonCodes)
	}
}

func eligibleSnapshot(now time.Time) *Snapshot {
	return &Snapshot{
		ClusterName:    "prod",
		Region:         "ap-south-1",
		CurrentVersion: "1.35",
		ClusterStatus:  "ACTIVE",
		SupportType:    string(ekstypes.SupportTypeStandard),
		ObservedAt:     now,
		Updates: []UpdateRecord{{
			ID:        "upgrade-1",
			Type:      string(ekstypes.UpdateTypeVersionUpdate),
			Status:    string(ekstypes.UpdateStatusSuccessful),
			CreatedAt: now.Add(-24 * time.Hour),
			Version:   "1.35",
		}},
		ClusterVersions: []ClusterVersionRecord{{Version: "1.34", Status: string(ekstypes.VersionStatusStandardSupport)}},
		Errors:          map[string]error{},
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
		listUpdatesOut:     &awseks.ListUpdatesOutput{UpdateIds: []string{"upgrade-1"}},
		listInsightsOut:    &awseks.ListInsightsOutput{},
		describeInsightOut: map[string]*awseks.DescribeInsightOutput{},
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

func insightSummary(id, name string, status ekstypes.InsightStatusValue, refresh time.Time) ekstypes.InsightSummary {
	return ekstypes.InsightSummary{
		Id:              awssdk.String(id),
		Name:            awssdk.String(name),
		Category:        ekstypes.CategoryRollbackReadiness,
		InsightStatus:   &ekstypes.InsightStatus{Status: status, Reason: awssdk.String("reason")},
		LastRefreshTime: &refresh,
		Description:     awssdk.String("summary " + name),
	}
}

func insightOutput(id, description string, status ekstypes.InsightStatusValue, refresh time.Time) *awseks.DescribeInsightOutput {
	transition := refresh.Add(15 * time.Minute)
	return &awseks.DescribeInsightOutput{Insight: &ekstypes.Insight{
		Id:                 awssdk.String(id),
		Name:               awssdk.String("detail " + id),
		Category:           ekstypes.CategoryRollbackReadiness,
		InsightStatus:      &ekstypes.InsightStatus{Status: status, Reason: awssdk.String("detail reason")},
		Description:        awssdk.String(description),
		Recommendation:     awssdk.String("review rollback readiness"),
		LastRefreshTime:    &refresh,
		LastTransitionTime: &transition,
		AdditionalInfo:     map[string]string{"docs": "https://docs.aws.amazon.com/eks/"},
		Resources: []ekstypes.InsightResourceDetail{{
			Arn:                   awssdk.String("arn:aws:eks:ap-south-1:123:cluster/prod"),
			KubernetesResourceUri: awssdk.String("deployment/default/api"),
			InsightStatus:         &ekstypes.InsightStatus{Status: status, Reason: awssdk.String("resource reason")},
		}},
	}}
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
