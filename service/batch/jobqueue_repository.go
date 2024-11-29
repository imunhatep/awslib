package batch

import (
	"github.com/aws/aws-sdk-go-v2/service/batch"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *BatchRepository) ListJobQueueAll() ([]JobQueue, error) {
	return r.ListJobQueueByInput(&batch.DescribeJobQueuesInput{})
}

func (r *BatchRepository) ListJobQueueByInput(query *batch.DescribeJobQueuesInput) ([]JobQueue, error) {
	start := time.Now()
	var computeEnvs []JobQueue

	p := batch.NewDescribeJobQueuesPaginator(r.client.Batch(), query)
	for p.HasMorePages() {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeJobQueues", cfg.ResourceTypeBatchJobQueue)).
			Inc()

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			// metrics
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DescribeJobQueues", cfg.ResourceTypeBatchJobQueue)).
				Inc()

			return computeEnvs, errors.New(err)
		}

		for _, v := range resp.JobQueues {
			cenv := NewJobQueue(r.client, v)
			computeEnvs = append(computeEnvs, cenv)
		}
	}

	// metrics
	metrics.AwsApiResourcesFetched.
		With(r.promLabels("DescribeJobQueues", cfg.ResourceTypeBatchJobQueue)).
		Add(float64(len(computeEnvs)))

	// metrics
	metrics.AwsRepoCallDuration.
		With(r.promLabels("ListJobQueueByInput", cfg.ResourceTypeBatchJobQueue)).
		Observe(time.Since(start).Seconds())

	return computeEnvs, nil
}
