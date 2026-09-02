package secretmanager

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// Reading a secret value is not a passive operation.
//
// GetSecretValue sets the secret's LastAccessedDate to today, and nothing restores the
// previous value — not CloudTrail, whose retention is far shorter than the periods that
// field is used to measure. LastAccessedDate is the only signal AWS offers for "is this
// secret still used by anything", so a caller that judges secrets by their idleness must
// not call anything in this file: one read makes the secret it was examining look freshly
// used, and does so silently.
//
// Note which calls are safe. ListSecrets, DescribeSecret, TagResource and UntagResource
// all leave LastAccessedDate alone, so metadata and tag work carries no such hazard.
// LastChangedDate is not a substitute: a tag write updates it, so anything tagging a
// secret pollutes that field itself.
//
// A caller that never needs a value should deny secretsmanager:GetSecretValue and
// secretsmanager:PutSecretValue in its IAM role. That is the one form of this warning a
// later code change cannot undo.

// DescribeSecretValue returns the secret's current value.
//
// Updates LastAccessedDate — see the note above.
func (r *SecretManagerRepository) DescribeSecretValue(secret SecretEntry) (*SecretValue, error) {
	query := &secretsmanager.GetSecretValueInput{SecretId: aws.String(secret.GetArn())}
	return r.DescribeSecretValueByInput(query)
}

// DescribeSecretValueByInput returns a secret value by input.
//
// Updates LastAccessedDate — see the note above.
func (r *SecretManagerRepository) DescribeSecretValueByInput(query *secretsmanager.GetSecretValueInput) (*SecretValue, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetSecretValue", cfg.ResourceTypeSecret)).Inc()
	}

	secretValueOutput, err := r.smClient().GetSecretValue(r.ctx, query)
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
			With(r.promLabels("GetSecretValue", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return &secret, nil
}

// UpdateSecretValue replaces the secret's value, creating a new version.
//
// Writes LastChangedDate — see the note above. It takes a SecretValue, so the caller has
// already read one.
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
