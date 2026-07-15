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
	Updates         []UpdateRecord
	ClusterVersions []ClusterVersionRecord
	Errors          map[string]error
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
	c.collectUpdates(ctx, timeout, snap)
	c.collectClusterVersions(ctx, timeout, snap)

	return snap, nil
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

func (c *Collector) collectClusterVersions(ctx context.Context, timeout time.Duration, snap *Snapshot) {
	var nextToken *string
	for {
		var out *awseks.DescribeClusterVersionsOutput
		err := callWithTimeout(ctx, timeout, func(callCtx context.Context) error {
			var describeErr error
			out, describeErr = c.client.DescribeClusterVersions(callCtx, &awseks.DescribeClusterVersionsInput{
				IncludeAll: awssdk.Bool(true),
				NextToken:  nextToken,
			})
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
