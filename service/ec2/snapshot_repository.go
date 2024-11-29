package ec2

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"time"
)

func (r *Ec2Repository) ListSnapshotsAll() ([]Snapshot, error) {
	return r.ListSnapshotsByInput(&ec2.DescribeSnapshotsInput{
		OwnerIds: []string{r.client.GetAccountID().String()},
	})
}

func (r *Ec2Repository) ListSnapshotsByInput(query *ec2.DescribeSnapshotsInput) ([]Snapshot, error) {
	start := time.Now()
	var snapshots []Snapshot

	p := ec2.NewDescribeSnapshotsPaginator(r.client.EC2(), query)

	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeSnapshots", ccfg.ResourceTypeSnapshot)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeSnapshots", ccfg.ResourceTypeSnapshot)).Inc()
			}

			return snapshots, errors.New(err)
		}

		for _, v := range resp.Snapshots {
			snapshots = append(snapshots, NewSnapshot(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeSnapshots", ccfg.ResourceTypeSnapshot)).
			Add(float64(len(snapshots)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListSnapshotsByInput", ccfg.ResourceTypeSnapshot)).
			Observe(time.Since(start).Seconds())
	}

	return snapshots, nil
}
