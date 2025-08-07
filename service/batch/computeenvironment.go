package batch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type ComputeEnvironmentList struct {
	Items []ComputeEnvironment
}

type ComputeEnvironment struct {
	service.AbstractResource
	types.ComputeEnvironmentDetail
	Tags map[string]string
}

func NewComputeEnvironment(client AwsClient, cenv types.ComputeEnvironmentDetail) ComputeEnvironment {
	jArn, _ := arn.Parse(aws.ToString(cenv.ComputeEnvironmentArn))

	return ComputeEnvironment{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(cenv.ComputeEnvironmentName),
			ARN:       &jArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeBatchComputeEnvironment,
		},
		ComputeEnvironmentDetail: cenv,
		Tags:                     cenv.Tags,
	}
}

func (e ComputeEnvironment) GetName() string {
	return aws.ToString(e.ComputeEnvironmentName)
}

func (e ComputeEnvironment) GetTags() map[string]string {
	return e.Tags
}

func (e ComputeEnvironment) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
