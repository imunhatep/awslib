package rds

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// ListDBClustersAll returns every DB cluster in the region, Neptune and DocumentDB included.
func (r *RdsRepository) ListDBClustersAll() ([]DBCluster, error) {
	return r.ListDBClustersByInput(&awsrds.DescribeDBClustersInput{})
}

func (r *RdsRepository) ListDBClustersByInput(query *awsrds.DescribeDBClustersInput) ([]DBCluster, error) {
	if query == nil {
		return []DBCluster{}, nil
	}

	start := time.Now()
	var clusters []DBCluster

	p := awsrds.NewDescribeDBClustersPaginator(r.rdsClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeDBClusters", cfg.ResourceTypeDBCluster)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeDBClusters", cfg.ResourceTypeDBCluster)).
					Inc()
			}

			return clusters, errors.New(err)
		}

		for _, v := range resp.DBClusters {
			clusters = append(clusters, NewDBCluster(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeDBClusters", cfg.ResourceTypeDBCluster)).
			Add(float64(len(clusters)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDBClustersByInput", cfg.ResourceTypeDBCluster)).
			Observe(time.Since(start).Seconds())
	}

	return clusters, nil
}
