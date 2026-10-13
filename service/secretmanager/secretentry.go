package secretmanager

import (
	"encoding/json"

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
// Every field SecretListEntry carries is copied across. SecretVersionsToStages lands in
// VersionIdsToStages, the same version id → staging labels map under its DescribeSecret
// name, so the AWSCURRENT version id — which changes with every new value — is known
// without reading the value.
//
// One DescribeSecret field cannot be filled: **SecretListEntry has no ReplicationStatus**,
// so an entry built here always reports nil replicas whether or not the secret has any.
// Read that nil as "unknown", never as "no replicas" — a caller that needs to know
// (deleting a replicated secret requires detaching the replicas first) has to call
// DescribeSecret for that secret.
//
// The rest of what the delete and lifecycle decisions need does come from the list call:
// LastAccessedDate, LastRotatedDate, DeletedDate, OwningService, PrimaryRegion,
// RotationEnabled and Tags. That is why a whole-account sweep costs one paginated
// ListSecrets rather than a DescribeSecret per secret.
func NewSecretEntryFromList(client AwsClient, secret types2.SecretListEntry) SecretEntry {
	describeSecretOutput := &sm.DescribeSecretOutput{
		ARN:                secret.ARN,
		Name:               secret.Name,
		CreatedDate:        secret.CreatedDate,
		Description:        secret.Description,
		KmsKeyId:           secret.KmsKeyId,
		DeletedDate:        secret.DeletedDate,
		RotationEnabled:    secret.RotationEnabled,
		LastAccessedDate:   secret.LastAccessedDate,
		LastChangedDate:    secret.LastChangedDate,
		LastRotatedDate:    secret.LastRotatedDate,
		NextRotationDate:   secret.NextRotationDate,
		OwningService:      secret.OwningService,
		PrimaryRegion:      secret.PrimaryRegion,
		RotationRules:      secret.RotationRules,
		RotationLambdaARN:  secret.RotationLambdaARN,
		Tags:               secret.Tags,
		VersionIdsToStages: secret.SecretVersionsToStages,
	}

	return NewSecretEntry(client, describeSecretOutput)
}

// secretEntryWire is the cache form of SecretEntry.
//
// gob cannot encode SecretEntry by reflection: DescribeSecretOutput carries
// ResultMetadata, a middleware.Metadata with no exported fields, and gob rejects such
// a type outright ("has no exported fields") whatever its value. JSON encodes it as {}
// and keeps every other field, including ones a newer SDK adds, which a hand-written
// mirror of DescribeSecretOutput would silently drop. The fields are named rather
// than embedded because AbstractResource and DescribeSecretOutput both have ARN and
// Type, and JSON drops conflicting promoted fields.
type secretEntryWire struct {
	Resource service.AbstractResource
	Secret   *sm.DescribeSecretOutput
}

// GobEncode makes SecretEntry cacheable, see secretEntryWire.
func (e SecretEntry) GobEncode() ([]byte, error) {
	return json.Marshal(secretEntryWire{Resource: e.AbstractResource, Secret: e.DescribeSecretOutput})
}

// GobDecode is the inverse of GobEncode.
func (e *SecretEntry) GobDecode(data []byte) error {
	var wire secretEntryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	e.AbstractResource = wire.Resource
	e.DescribeSecretOutput = wire.Secret

	return nil
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
