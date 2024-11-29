package rds

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *RdsRepository) ListDbSnapshotsAll() ([]DbSnapshot, error) {
	return r.ListDbSnapshotsByInput(&rds.DescribeDBSnapshotsInput{})
}

func (r *RdsRepository) ListDbSnapshotsByInput(query *rds.DescribeDBSnapshotsInput) ([]DbSnapshot, error) {
	start := time.Now()
	var snapshots []DbSnapshot

	p := rds.NewDescribeDBSnapshotsPaginator(r.client.RDS(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeDBSnapshots", cfg.ResourceTypeDBSnapshot)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeDBSnapshots", cfg.ResourceTypeDBSnapshot)).
					Inc()
			}

			return snapshots, errors.New(err)
		}

		for _, v := range resp.DBSnapshots {
			snapshots = append(snapshots, NewDbSnapshot(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeDbSnapshots", cfg.ResourceTypeDBSnapshot)).
			Add(float64(len(snapshots)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDbInstancesByInput", cfg.ResourceTypeDBSnapshot)).
			Observe(time.Since(start).Seconds())
	}

	return snapshots, nil
}
