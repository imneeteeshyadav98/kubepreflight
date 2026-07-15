package eks

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

type Snapshot struct {
	ClusterName     string
	Region          string
	CurrentVersion  string
	ClusterStatus   string
	SupportType     string
	ObservedAt      time.Time
	Insights        []InsightRecord
	Updates         []UpdateRecord
	ClusterVersions []ClusterVersionRecord
	Errors          map[string]error
}

type InsightRecord struct {
	ID                 string
	Name               string
	Category           string
	Status             string
	Reason             string
	Description        string
	Recommendation     string
	LastRefreshTime    time.Time
	LastTransitionTime time.Time
	AdditionalInfo     map[string]string
	Resources          []InsightResourceRecord
}

type InsightResourceRecord struct {
	ARN                   string
	KubernetesResourceURI string
	Status                string
	Reason                string
}

type UpdateRecord struct {
	ID        string
	Type      string
	Status    string
	CreatedAt time.Time
	Version   string
	Params    map[string]string
}

type ClusterVersionRecord struct {
	Version string
	Status  string
}

type Collector struct {
	client      Client
	clusterName string
	region      string
}

func NewCollector(client Client, clusterName, region string) *Collector {
	return &Collector{client: client, clusterName: clusterName, region: region}
}

func (c *Collector) Collect(ctx context.Context, timeout time.Duration, now time.Time) (*Snapshot, error) {
	snap := &Snapshot{
		ClusterName: c.clusterName,
		Region:      c.region,
		ObservedAt:  now,
		Errors:      map[string]error{},
	}

	c.collectCluster(ctx, timeout, snap)
	c.collectRollbackInsights(ctx, timeout, snap)
	c.collectUpdates(ctx, timeout, snap)
	c.collectClusterVersions(ctx, timeout, previousMinor(snap.CurrentVersion), snap)

	return snap, nil
}

func (c *Collector) collectRollbackInsights(ctx context.Context, timeout time.Duration, snap *Snapshot) {
	summaries, err := c.listRollbackInsightSummaries(ctx, timeout)
	if err != nil {
		snap.Errors["list-rollback-insights"] = err
		return
	}
	for _, summary := range summaries {
		c.collectRollbackInsightDetail(ctx, timeout, summary, snap)
	}
}

func (c *Collector) listRollbackInsightSummaries(ctx context.Context, timeout time.Duration) ([]ekstypes.InsightSummary, error) {
	filter := &ekstypes.InsightsFilter{Categories: []ekstypes.Category{ekstypes.CategoryRollbackReadiness}}
	var summaries []ekstypes.InsightSummary
	var nextToken *string
	for {
		var out *awseks.ListInsightsOutput
		err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
			var listErr error
			out, listErr = c.client.ListInsights(callCtx, &awseks.ListInsightsInput{
				ClusterName: awssdk.String(c.clusterName),
				Filter:      filter,
				NextToken:   nextToken,
			})
			return listErr
		})
		if err != nil {
			return nil, err
		}
		if out == nil {
			return summaries, nil
		}
		summaries = append(summaries, out.Insights...)
		if out.NextToken == nil || awssdk.ToString(out.NextToken) == "" {
			return summaries, nil
		}
		nextToken = out.NextToken
	}
}

func (c *Collector) collectRollbackInsightDetail(ctx context.Context, timeout time.Duration, summary ekstypes.InsightSummary, snap *Snapshot) {
	if summary.Id == nil {
		return
	}
	rec := insightRecordFromSummary(summary)

	var out *awseks.DescribeInsightOutput
	err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
		var describeErr error
		out, describeErr = c.client.DescribeInsight(callCtx, &awseks.DescribeInsightInput{
			ClusterName: awssdk.String(c.clusterName),
			Id:          summary.Id,
		})
		return describeErr
	})
	if err != nil {
		snap.Errors["describe-rollback-insight:"+rec.ID] = err
	} else if out != nil && out.Insight != nil {
		rec = mergeInsightRecord(rec, out.Insight)
	}

	snap.Insights = append(snap.Insights, rec)
}

func insightRecordFromSummary(summary ekstypes.InsightSummary) InsightRecord {
	return InsightRecord{
		ID:                 awssdk.ToString(summary.Id),
		Name:               awssdk.ToString(summary.Name),
		Category:           string(summary.Category),
		Status:             insightStatusValue(summary.InsightStatus),
		Reason:             insightStatusReason(summary.InsightStatus),
		Description:        awssdk.ToString(summary.Description),
		LastRefreshTime:    awssdk.ToTime(summary.LastRefreshTime),
		LastTransitionTime: awssdk.ToTime(summary.LastTransitionTime),
	}
}

func mergeInsightRecord(rec InsightRecord, insight *ekstypes.Insight) InsightRecord {
	if insight.Id != nil {
		rec.ID = awssdk.ToString(insight.Id)
	}
	if insight.Name != nil {
		rec.Name = awssdk.ToString(insight.Name)
	}
	if insight.Category != "" {
		rec.Category = string(insight.Category)
	}
	if insight.InsightStatus != nil {
		rec.Status = insightStatusValue(insight.InsightStatus)
		rec.Reason = insightStatusReason(insight.InsightStatus)
	}
	if insight.Description != nil {
		rec.Description = awssdk.ToString(insight.Description)
	}
	rec.Recommendation = awssdk.ToString(insight.Recommendation)
	if insight.LastRefreshTime != nil {
		rec.LastRefreshTime = awssdk.ToTime(insight.LastRefreshTime)
	}
	if insight.LastTransitionTime != nil {
		rec.LastTransitionTime = awssdk.ToTime(insight.LastTransitionTime)
	}
	if len(insight.AdditionalInfo) > 0 {
		rec.AdditionalInfo = map[string]string{}
		for k, v := range insight.AdditionalInfo {
			rec.AdditionalInfo[k] = v
		}
	}
	rec.Resources = insightResources(insight.Resources)
	return rec
}

func insightResources(resources []ekstypes.InsightResourceDetail) []InsightResourceRecord {
	out := make([]InsightResourceRecord, 0, len(resources))
	for _, resource := range resources {
		out = append(out, InsightResourceRecord{
			ARN:                   awssdk.ToString(resource.Arn),
			KubernetesResourceURI: awssdk.ToString(resource.KubernetesResourceUri),
			Status:                insightStatusValue(resource.InsightStatus),
			Reason:                insightStatusReason(resource.InsightStatus),
		})
	}
	return out
}

func insightStatusValue(status *ekstypes.InsightStatus) string {
	if status == nil || status.Status == "" {
		return string(ekstypes.InsightStatusValueUnknown)
	}
	return string(status.Status)
}

func insightStatusReason(status *ekstypes.InsightStatus) string {
	if status == nil || status.Reason == nil {
		return ""
	}
	return awssdk.ToString(status.Reason)
}

func callWithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(callCtx)
}

func (c *Collector) collectCluster(ctx context.Context, timeout time.Duration, snap *Snapshot) {
	var out *awseks.DescribeClusterOutput
	err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
		var describeErr error
		out, describeErr = c.client.DescribeCluster(callCtx, &awseks.DescribeClusterInput{Name: awssdk.String(c.clusterName)})
		return describeErr
	})
	if err != nil {
		snap.Errors["describe-cluster"] = err
		return
	}
	if out == nil || out.Cluster == nil {
		snap.Errors["describe-cluster"] = fmt.Errorf("cluster %q was not returned", c.clusterName)
		return
	}
	snap.CurrentVersion = awssdk.ToString(out.Cluster.Version)
	snap.ClusterStatus = string(out.Cluster.Status)
	if out.Cluster.UpgradePolicy != nil {
		snap.SupportType = string(out.Cluster.UpgradePolicy.SupportType)
	}
}

func (c *Collector) collectUpdates(ctx context.Context, timeout time.Duration, snap *Snapshot) {
	ids, err := c.listUpdateIDs(ctx, timeout)
	if err != nil {
		snap.Errors["list-updates"] = err
		return
	}
	for _, id := range ids {
		rec, err := c.describeUpdate(ctx, timeout, id)
		if err != nil {
			snap.Errors["describe-update:"+id] = err
			continue
		}
		snap.Updates = append(snap.Updates, rec)
	}
}

func (c *Collector) listUpdateIDs(ctx context.Context, timeout time.Duration) ([]string, error) {
	var ids []string
	var nextToken *string
	for {
		var out *awseks.ListUpdatesOutput
		err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
			var listErr error
			out, listErr = c.client.ListUpdates(callCtx, &awseks.ListUpdatesInput{
				Name:      awssdk.String(c.clusterName),
				NextToken: nextToken,
			})
			return listErr
		})
		if err != nil {
			return nil, err
		}
		if out == nil {
			return ids, nil
		}
		ids = append(ids, out.UpdateIds...)
		if out.NextToken == nil || awssdk.ToString(out.NextToken) == "" {
			return ids, nil
		}
		nextToken = out.NextToken
	}
}

func (c *Collector) describeUpdate(ctx context.Context, timeout time.Duration, id string) (UpdateRecord, error) {
	var out *awseks.DescribeUpdateOutput
	err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
		var describeErr error
		out, describeErr = c.client.DescribeUpdate(callCtx, &awseks.DescribeUpdateInput{
			Name:     awssdk.String(c.clusterName),
			UpdateId: awssdk.String(id),
		})
		return describeErr
	})
	if err != nil {
		return UpdateRecord{}, err
	}
	if out == nil || out.Update == nil {
		return UpdateRecord{}, fmt.Errorf("update %q was not returned", id)
	}
	return updateRecord(out.Update), nil
}

func updateRecord(update *ekstypes.Update) UpdateRecord {
	rec := UpdateRecord{
		ID:        awssdk.ToString(update.Id),
		Type:      string(update.Type),
		Status:    string(update.Status),
		CreatedAt: awssdk.ToTime(update.CreatedAt),
		Params:    map[string]string{},
	}
	for _, param := range update.Params {
		key := string(param.Type)
		value := awssdk.ToString(param.Value)
		if key == "" {
			continue
		}
		rec.Params[key] = value
		if param.Type == ekstypes.UpdateParamTypeVersion {
			rec.Version = value
		}
	}
	return rec
}

func (c *Collector) collectClusterVersions(ctx context.Context, timeout time.Duration, targetVersion string, snap *Snapshot) {
	var nextToken *string
	for {
		var out *awseks.DescribeClusterVersionsOutput
		err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
			var describeErr error
			// IncludeAll and ClusterVersions are mutually exclusive on the
			// real EKS API (InvalidParameterException if both are set,
			// confirmed against a real cluster -- the fake test client
			// doesn't enforce this, so it went unnoticed until then).
			// ClusterVersions is a specific-version lookup by identifier,
			// so it already returns the record regardless of support
			// tier -- IncludeAll only matters for the unfiltered listing
			// used when no target version is known yet.
			input := &awseks.DescribeClusterVersionsInput{
				NextToken: nextToken,
			}
			if targetVersion != "" {
				input.ClusterVersions = []string{targetVersion}
			} else {
				input.IncludeAll = awssdk.Bool(true)
			}
			out, describeErr = c.client.DescribeClusterVersions(callCtx, input)
			return describeErr
		})
		if err != nil {
			snap.Errors["describe-cluster-versions"] = err
			return
		}
		if out == nil {
			return
		}
		for _, version := range out.ClusterVersions {
			snap.ClusterVersions = append(snap.ClusterVersions, ClusterVersionRecord{
				Version: awssdk.ToString(version.ClusterVersion),
				Status:  string(version.VersionStatus),
			})
		}
		if out.NextToken == nil || awssdk.ToString(out.NextToken) == "" {
			return
		}
		nextToken = out.NextToken
	}
}
