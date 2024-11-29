package batch

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type JobQueueList struct {
	Items []JobQueue
}

type JobQueue struct {
	service.AbstractResource
	types.JobQueueDetail
	Tags map[string]string
}

func init() {
	gob.Register(JobQueue{})
}

func NewJobQueue(client AwsClient, jobQueue types.JobQueueDetail) JobQueue {
	jArn, _ := arn.Parse(aws.ToString(jobQueue.JobQueueArn))

	return JobQueue{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(jobQueue.JobQueueName),
			ARN:       &jArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeBatchJobQueue,
		},
		JobQueueDetail: jobQueue,
	}
}

func (e JobQueue) GetName() string {
	return aws.ToString(e.JobQueueName)
}

func (e JobQueue) GetTags() map[string]string {
	return e.Tags
}

func (e JobQueue) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
