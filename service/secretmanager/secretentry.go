package secretmanager

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	types2 "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/imunhatep/awslib/service"
)

type SecretEntryList struct {
	Items []SecretEntry
}

type SecretEntry struct {
	service.AbstractResource
	*sm.DescribeSecretOutput
}

func init() {
	gob.Register(SecretEntry{})
}

func NewSecretEntry(client AwsClient, secret *sm.DescribeSecretOutput) SecretEntry {
	smArn, _ := arn.Parse(aws.ToString(secret.ARN))

	return SecretEntry{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(secret.Name),
			ARN:       &smArn,
			CreatedAt: aws.ToTime(secret.CreatedDate),
			Type:      cfg.ResourceTypeSecret,
		},
		DescribeSecretOutput: secret,
	}
}

func NewSecretEntryFromList(client AwsClient, secret types2.SecretListEntry) SecretEntry {
	describeSecretOutput := &sm.DescribeSecretOutput{
		ARN:               secret.ARN,
		Name:              secret.Name,
		CreatedDate:       secret.CreatedDate,
		Description:       secret.Description,
		KmsKeyId:          secret.KmsKeyId,
		DeletedDate:       secret.DeletedDate,
		RotationEnabled:   secret.RotationEnabled,
		LastAccessedDate:  secret.LastAccessedDate,
		LastChangedDate:   secret.LastChangedDate,
		LastRotatedDate:   secret.LastRotatedDate,
		NextRotationDate:  secret.NextRotationDate,
		OwningService:     secret.OwningService,
		PrimaryRegion:     secret.PrimaryRegion,
		RotationRules:     secret.RotationRules,
		RotationLambdaARN: secret.RotationLambdaARN,
		Tags:              secret.Tags,
	}

	return NewSecretEntry(client, describeSecretOutput)
}

func (e SecretEntry) GetName() string {
	return aws.ToString(e.DescribeSecretOutput.Name)
}

func (e SecretEntry) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e SecretEntry) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
