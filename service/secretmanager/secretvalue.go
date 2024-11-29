package secretmanager

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/imunhatep/awslib/service"
)

type SecretValue struct {
	service.AbstractResource
	*sm.GetSecretValueOutput
}

func init() {
	gob.Register(SecretValue{})
}

func NewSecretValue(client AwsClient, value *sm.GetSecretValueOutput) SecretValue {
	smArn, _ := arn.Parse(aws.ToString(value.ARN))

	return SecretValue{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(value.Name),
			ARN:       &smArn,
			CreatedAt: aws.ToTime(value.CreatedDate),
			Type:      cfg.ResourceTypeSecret,
		},
		GetSecretValueOutput: value,
	}
}

func (e SecretValue) GetName() string {
	return aws.ToString(e.GetSecretValueOutput.Name)
}

// GetTags secretValue does not have Tags
func (e SecretValue) GetTags() map[string]string {
	return map[string]string{}
}

func (e SecretValue) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
