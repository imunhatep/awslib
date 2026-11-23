package ecr

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/gocollection/dict"
)

type RepoList struct {
	Items []Repo
}

// Repo is an ECR repository as returned by DescribeRepositories, with its tags.
// Images are not listed.
type Repo struct {
	service.AbstractResource
	ecrtypes.Repository
	Tags map[string]string
}

func NewRepo(client AwsClient, repository ecrtypes.Repository, tags map[string]string) Repo {
	return Repo{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(repository.RepositoryName),
			ARN:       parseArn(repository.RepositoryArn),
			CreatedAt: aws.ToTime(repository.CreatedAt),
			Type:      cfg.ResourceTypeECRRepository,
		},
		Repository: repository,
		Tags:       tags,
	}
}

func (e Repo) GetName() string {
	return aws.ToString(e.RepositoryName)
}

func (e Repo) GetTags() map[string]string {
	return dict.Copy(e.Tags)
}

func (e Repo) GetTagValue(tag string) string {
	return e.Tags[tag]
}

// GetUri returns the URI images are pushed to and pulled from
// (<registry>.dkr.ecr.<region>.amazonaws.com/<name>).
func (e Repo) GetUri() string {
	return aws.ToString(e.RepositoryUri)
}

// GetRegistryId returns the ID of the registry (the account) holding the repository.
func (e Repo) GetRegistryId() string {
	return aws.ToString(e.RegistryId)
}

// GetTagMutability returns MUTABLE, IMMUTABLE, or one of the *_WITH_EXCLUSION
// variants whose exceptions are in ImageTagMutabilityExclusionFilters.
func (e Repo) GetTagMutability() string {
	return string(e.ImageTagMutability)
}

// IsTagImmutable reports whether image tags cannot be overwritten, with or
// without exclusion filters.
func (e Repo) IsTagImmutable() bool {
	return e.ImageTagMutability == ecrtypes.ImageTagMutabilityImmutable ||
		e.ImageTagMutability == ecrtypes.ImageTagMutabilityImmutableWithExclusion
}

// IsScanOnPush reports whether images are scanned when pushed. This is the
// repository-level basic scanning setting; registry-level scanning
// configuration can scan regardless of it.
func (e Repo) IsScanOnPush() bool {
	return e.ImageScanningConfiguration != nil && e.ImageScanningConfiguration.ScanOnPush
}

// GetEncryptionType returns AES256, KMS or KMS_DSSE. ECR encrypts every
// repository, so a repository without an encryption configuration reports
// AES256, the default.
func (e Repo) GetEncryptionType() string {
	if e.EncryptionConfiguration == nil || e.EncryptionConfiguration.EncryptionType == "" {
		return string(ecrtypes.EncryptionTypeAes256)
	}

	return string(e.EncryptionConfiguration.EncryptionType)
}

// GetKmsKey returns the KMS key used for KMS or KMS_DSSE encryption as the API
// reports it, empty when it reports none (as for AES256).
func (e Repo) GetKmsKey() string {
	if e.EncryptionConfiguration == nil {
		return ""
	}

	return aws.ToString(e.EncryptionConfiguration.KmsKey)
}

func parseArn(value *string) *arn.ARN {
	if value == nil || *value == "" {
		return nil
	}

	parsed, err := arn.Parse(*value)
	if err != nil {
		return nil
	}

	return &parsed
}

func tagsToMap(tags []ecrtypes.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}
