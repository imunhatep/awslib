package secretmanager

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type SecretManagerClient interface {
}

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	SecretsManager() *secretsmanager.Client
}

type SecretManagerRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewSecretManagerRepository(ctx context.Context, client AwsClient) *SecretManagerRepository {
	repo := &SecretManagerRepository{ctx, client}

	return repo
}

func (r *SecretManagerRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *SecretManagerRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *SecretManagerRepository) ListSecretsAll() ([]SecretEntry, error) {
	return r.ListSecretsByInput(&secretsmanager.ListSecretsInput{})
}

func (r *SecretManagerRepository) ListSecretsByInput(query *secretsmanager.ListSecretsInput) ([]SecretEntry, error) {
	start := time.Now()
	var secrets []SecretEntry

	p := secretsmanager.NewListSecretsPaginator(r.client.SecretsManager(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListSecretsByInput", cfg.ResourceTypeSecret)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListSecretsByInput", cfg.ResourceTypeSecret)).Inc()
			}

			return secrets, errors.New(err)
		}

		for _, v := range resp.SecretList {
			secret := NewSecretEntryFromList(r.client, v)
			secrets = append(secrets, secret)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListSecretsByInput", cfg.ResourceTypeSecret)).
			Add(float64(len(secrets)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListSecretsByInput", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return secrets, nil
}

func (r *SecretManagerRepository) DescribeSecret(query *secretsmanager.DescribeSecretInput) (*SecretEntry, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Inc()
	}

	secretOutput, err := r.client.SecretsManager().DescribeSecret(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	if secretOutput == nil {
		return nil, errors.New("secret not found")
	}

	secret := NewSecretEntry(r.client, secretOutput)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return &secret, nil
}

//secretInput := secretsmanager.CreateSecretInput{
//	KmsKeyId:          entry.KmsKeyId,
//	Name:              entry.Name,
//	Description:       entry.Description,
//	Tags:              entry.Tags,
//	SecretString:      value.SecretString,
//	SecretBinary:      value.SecretBinary,
//	AddReplicaRegions: replicas,
//}

// CreateSecret a new secret
func (r *SecretManagerRepository) CreateSecret(secretInput *secretsmanager.CreateSecretInput) (*SecretEntry, error) {
	start := time.Now()

	secretInput.ForceOverwriteReplicaSecret = false

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CreateSecret", cfg.ResourceTypeSecret)).Inc()
	}

	createSecretOutput, err := r.client.SecretsManager().CreateSecret(r.ctx, secretInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CreateSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	query := &secretsmanager.DescribeSecretInput{SecretId: createSecretOutput.ARN}
	describeSecretOutput, err := r.client.SecretsManager().DescribeSecret(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Add(1)
	}

	// secret entry
	secret := NewSecretEntry(r.client, describeSecretOutput)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateSecret", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return &secret, nil
}

// UpdateSecretByInput updates a secret
func (r *SecretManagerRepository) UpdateSecret(input *secretsmanager.UpdateSecretInput) (*SecretEntry, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("UpdateSecret", cfg.ResourceTypeSecret)).
			Inc()
	}

	updateSecretOutput, err := r.client.SecretsManager().UpdateSecret(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	query := &secretsmanager.DescribeSecretInput{SecretId: updateSecretOutput.ARN}
	describeSecretOutput, err := r.client.SecretsManager().DescribeSecret(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Add(1)
	}

	// secret entry
	secretUpdated := NewSecretEntry(r.client, describeSecretOutput)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("Update", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return &secretUpdated, nil
}

// DeleteSecretByInput updates a secret
func (r *SecretManagerRepository) DeleteSecretByInput(input *secretsmanager.DeleteSecretInput) error {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DeleteSecret", cfg.ResourceTypeSecret)).
			Inc()
	}

	_, err := r.client.SecretsManager().DeleteSecret(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteSecret", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}
