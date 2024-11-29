package rds

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *RdsRepository) ListDbInstancesAll() ([]DbInstance, error) {
	return r.ListDbInstancesByInput(&rds.DescribeDBInstancesInput{})
}

func (r *RdsRepository) ListDbInstancesByInput(query *rds.DescribeDBInstancesInput) ([]DbInstance, error) {
	start := time.Now()
	var instances []DbInstance

	p := rds.NewDescribeDBInstancesPaginator(r.client.RDS(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeDBInstances", cfg.ResourceTypeDBInstance)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeDBInstances", cfg.ResourceTypeDBInstance)).
					Inc()
			}

			return instances, errors.New(err)
		}

		for _, db := range resp.DBInstances {
			instance := NewDbInstance(r.client, db)
			instances = append(instances, instance)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeDBInstances", cfg.ResourceTypeDBInstance)).
			Add(float64(len(instances)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListDbInstancesByInput", cfg.ResourceTypeDBInstance)).
			Observe(time.Since(start).Seconds())
	}

	return instances, nil
}
