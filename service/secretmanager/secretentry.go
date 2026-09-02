package secretmanager

import (
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

// NewSecretEntryFromList builds a SecretEntry from a ListSecrets result.
//
// Every field SecretListEntry carries is copied across, which is all of them bar one:
// **SecretListEntry has no ReplicationStatus**, so an entry built here always reports
// nil replicas whether or not the secret has any. Read that nil as "unknown", never as
// "no replicas" — a caller that needs to know (deleting a replicated secret requires
// detaching the replicas first) has to call DescribeSecret for that secret.
//
// The rest of what the delete and lifecycle decisions need does come from the list call:
// LastAccessedDate, LastRotatedDate, DeletedDate, OwningService, PrimaryRegion,
// RotationEnabled and Tags. That is why a whole-account sweep costs one paginated
// ListSecrets rather than a DescribeSecret per secret.
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
