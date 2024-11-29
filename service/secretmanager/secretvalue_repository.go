package secretmanager

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *SecretManagerRepository) DescribeSecretValue(secret SecretEntry) (*SecretValue, error) {
	query := &secretsmanager.GetSecretValueInput{SecretId: aws.String(secret.GetArn())}
	return r.DescribeSecretValueByInput(query)
}

// DescribeSecretValueByInput returns a secret value by input
func (r *SecretManagerRepository) DescribeSecretValueByInput(query *secretsmanager.GetSecretValueInput) (*SecretValue, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetSecretValue", cfg.ResourceTypeSecret)).Inc()
	}

	secretValueOutput, err := r.client.SecretsManager().GetSecretValue(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetSecretValue", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	// secret entry
	secret := NewSecretValue(r.client, secretValueOutput)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetSecretValueByInput", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return &secret, nil
}

func (r *SecretManagerRepository) UpdateSecretValue(secret SecretEntry, value SecretValue) (*SecretEntry, error) {
	// building request
	secretInput := &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(secret.GetIdOrArn()),
		KmsKeyId:     secret.KmsKeyId,
		Description:  secret.Description,
		SecretString: value.SecretString,
		SecretBinary: value.SecretBinary,
	}

	return r.UpdateSecret(secretInput)
}
