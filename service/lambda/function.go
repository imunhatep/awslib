package lambda

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	types2 "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type Function struct {
	service.AbstractResource
	types2.FunctionConfiguration
	Tags map[string]string
}

func init() {
	gob.Register(Function{})
}

func NewFunction(client AwsClient, fn types2.FunctionConfiguration, tags map[string]string) Function {
	fnArn, _ := arn.Parse(aws.ToString(fn.FunctionArn))
	updatedAt, _ := time.Parse("2006-01-02T15:04:05-0700", aws.ToString(fn.LastModified))

	return Function{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(fn.FunctionName),
			ARN:       &fnArn,
			CreatedAt: updatedAt,
			Type:      cfg.ResourceTypeFunction,
		},
		FunctionConfiguration: fn,
		Tags:                  tags,
	}
}

func (e Function) GetName() string {
	return aws.ToString(e.FunctionConfiguration.FunctionName)
}

func (e Function) GetTags() map[string]string {
	return e.Tags
}

func (e Function) GetTagValue(tag string) string {
	val, ok := e.Tags[tag]
	if !ok {
		return ""
	}

	return val
}
