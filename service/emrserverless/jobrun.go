package emrserverless

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type JobRunList struct {
	Items []JobRun
}

type JobRun struct {
	service.AbstractResource
	*types.JobRun
}

func init() {
	gob.Register(JobRun{})
	gob.Register(types.JobDriverMemberSparkSubmit{})
	gob.Register(types.JobDriverMemberHive{})
}

func NewJobRun(client AwsClient, jobRun *types.JobRun) JobRun {
	appArn, _ := arn.Parse(aws.ToString(jobRun.Arn))

	return JobRun{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(jobRun.JobRunId),
			ARN:       &appArn,
			CreatedAt: aws.ToTime(jobRun.CreatedAt),
			Type:      cfg.ResourceTypeEmrServerlessJobRun,
		},
		JobRun: jobRun,
	}
}

func (e JobRun) GetName() string {
	return aws.ToString(e.JobRun.Name)
}

func (e JobRun) GetTags() map[string]string {
	return e.JobRun.Tags
}

func (e JobRun) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
