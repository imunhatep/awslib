package rds

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// ListDBClusterSnapshotsAll returns the account's own DB cluster snapshots, manual and
// automated. Snapshots shared from other accounts or public ones are not included; ask for
// them with ListDBClusterSnapshotsByInput (IncludeShared / IncludePublic).
func (r *RdsRepository) ListDBClusterSnapshotsAll() ([]DBClusterSnapshot, error) {
	return r.ListDBClusterSnapshotsByInput(&awsrds.DescribeDBClusterSnapshotsInput{})
}

func (r *RdsRepository) ListDBClusterSnapshotsByInput(query *awsrds.DescribeDBClusterSnapshotsInput) ([]DBClusterSnapshot, error) {
	if query == nil {
		return []DBClusterSnapshot{}, nil
	}

	start := time.Now()
	var snapshots []DBClusterSnapshot

	p := awsrds.NewDescribeDBClusterSnapshotsPaginator(r.rdsClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeDBClusterSnapshots", cfg.ResourceTypeDBClusterSnapshot)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeDBClusterSnapshots", cfg.ResourceTypeDBClusterSnapshot)).
					Inc()
			}

			return snapshots, errors.New(err)
		}

		for _, v := range resp.DBClusterSnapshots {
			snapshots = append(snapshots, NewDBClusterSnapshot(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeDBClusterSnapshots", cfg.ResourceTypeDBClusterSnapshot)).
			Add(float64(len(snapshots)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDBClusterSnapshotsByInput", cfg.ResourceTypeDBClusterSnapshot)).
			Observe(time.Since(start).Seconds())
	}

	return snapshots, nil
}
