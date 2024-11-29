package batch

import (
	"github.com/aws/aws-sdk-go-v2/service/batch"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

// cfg.ResourceTypeBatchComputeEnvironment
func (r *BatchRepository) ListComputeEnvironmentAll() ([]ComputeEnvironment, error) {
	return r.ListComputeEnvironmentByInput(&batch.DescribeComputeEnvironmentsInput{})
}

func (r *BatchRepository) ListComputeEnvironmentByInput(query *batch.DescribeComputeEnvironmentsInput) ([]ComputeEnvironment, error) {
	start := time.Now()
	var computeEnvs []ComputeEnvironment

	p := batch.NewDescribeComputeEnvironmentsPaginator(r.client.Batch(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeComputeEnvironments", cfg.ResourceTypeBatchComputeEnvironment)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeComputeEnvironments", cfg.ResourceTypeBatchComputeEnvironment)).
					Inc()
			}

			return computeEnvs, errors.New(err)
		}

		for _, v := range resp.ComputeEnvironments {
			cenv := NewComputeEnvironment(r.client, v)
			computeEnvs = append(computeEnvs, cenv)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeComputeEnvironments", cfg.ResourceTypeBatchComputeEnvironment)).
			Add(float64(len(computeEnvs)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListComputeEnvironmentByInput", cfg.ResourceTypeBatchComputeEnvironment)).
			Observe(time.Since(start).Seconds())
	}

	return computeEnvs, nil
}
