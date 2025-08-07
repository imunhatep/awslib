package glue

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type JobList struct {
	Items []Job
}

type Job struct {
	service.AbstractResource
	types.Job
	Tags map[string]string
}

func NewJob(client AwsClient, job types.Job, tags map[string]string) Job {
	return Job{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(job.Name),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "glue", "job/", job.Name),
			CreatedAt: aws.ToTime(job.CreatedOn),
			Type:      cfg.ResourceTypeGlueJob,
		},
		Job:  job,
		Tags: tags,
	}
}

func (e Job) GetName() string {
	return aws.ToString(e.Job.Name)
}

func (e Job) GetTags() map[string]string {
	return e.Tags
}

func (e Job) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
