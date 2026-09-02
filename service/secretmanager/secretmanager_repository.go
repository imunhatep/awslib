package secretmanager

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awssecrets "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/secretsmanager"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type SecretManagerRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewSecretManagerRepository(ctx context.Context, client *v3.Client) *SecretManagerRepository {
	repo := &SecretManagerRepository{ctx, client}

	return repo
}

func (r *SecretManagerRepository) smClient() *awssecrets.Client {
	return secretsmanager.GetClient(r.client)
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
	return r.ListSecretsByInput(&awssecrets.ListSecretsInput{})
}

func (r *SecretManagerRepository) ListSecretsByInput(query *awssecrets.ListSecretsInput) ([]SecretEntry, error) {
	start := time.Now()
	var secrets []SecretEntry

	p := awssecrets.NewListSecretsPaginator(r.smClient(), query)
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

func (r *SecretManagerRepository) DescribeSecret(query *awssecrets.DescribeSecretInput) (*SecretEntry, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DescribeSecret", cfg.ResourceTypeSecret)).Inc()
	}

	secretOutput, err := r.smClient().DescribeSecret(r.ctx, query)
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

func (r *SecretManagerRepository) CreateSecret(secretInput *awssecrets.CreateSecretInput) (*SecretEntry, error) {
	start := time.Now()

	secretInput.ForceOverwriteReplicaSecret = false

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CreateSecret", cfg.ResourceTypeSecret)).Inc()
	}

	createSecretOutput, err := r.smClient().CreateSecret(r.ctx, secretInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CreateSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	query := &awssecrets.DescribeSecretInput{SecretId: createSecretOutput.ARN}
	describeSecretOutput, err := r.smClient().DescribeSecret(r.ctx, query)
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

func (r *SecretManagerRepository) UpdateSecret(input *awssecrets.UpdateSecretInput) (*SecretEntry, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("UpdateSecret", cfg.ResourceTypeSecret)).
			Inc()
	}

	updateSecretOutput, err := r.smClient().UpdateSecret(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateSecret", cfg.ResourceTypeSecret)).Inc()
		}

		return nil, errors.New(err)
	}

	query := &awssecrets.DescribeSecretInput{SecretId: updateSecretOutput.ARN}
	describeSecretOutput, err := r.smClient().DescribeSecret(r.ctx, query)
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
			With(r.promLabels("UpdateSecret", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return &secretUpdated, nil
}

func (r *SecretManagerRepository) DeleteSecretByInput(input *awssecrets.DeleteSecretInput) error {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DeleteSecret", cfg.ResourceTypeSecret)).
			Inc()
	}

	_, err := r.smClient().DeleteSecret(r.ctx, input)
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

// CreateSecretTags applies the tags in input to one secret. A nil input is a no-op, so
// BuildTagResourceInput's "already in sync" result can be passed straight through.
//
// TagResource is an upsert and carries no read, so this is safe to repeat: a reconciler
// working from a cached listing may re-send a tag it already applied, and AWS accepts it.
func (r *SecretManagerRepository) CreateSecretTags(input *awssecrets.TagResourceInput) (*awssecrets.TagResourceOutput, error) {
	if input == nil {
		return &awssecrets.TagResourceOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("CreateSecretTags", cfg.ResourceTypeSecret)).
			Inc()
	}

	output, err := r.smClient().TagResource(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("CreateSecretTags", cfg.ResourceTypeSecret)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateSecretTags", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// DeleteSecretTags removes the tag keys in input from one secret. A nil input is a
// no-op, matching BuildUntagResourceInput's "nothing to remove" result.
//
// UntagResource ignores a key the secret does not carry, so this is idempotent too.
func (r *SecretManagerRepository) DeleteSecretTags(input *awssecrets.UntagResourceInput) (*awssecrets.UntagResourceOutput, error) {
	if input == nil {
		return &awssecrets.UntagResourceOutput{}, nil
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DeleteSecretTags", cfg.ResourceTypeSecret)).
			Inc()
	}

	output, err := r.smClient().UntagResource(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DeleteSecretTags", cfg.ResourceTypeSecret)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteSecretTags", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// RemoveRegionsFromReplication detaches replica regions from a secret.
//
// It exists for the caller that needs to delete a replicated secret: DeleteSecret on a
// primary that still has replicas is refused, and a replica cannot be deleted directly
// at all. So the order is replicas first, primary second.
//
// The output's ReplicationStatus reports the regions that remain, which is the only way
// to tell a partial detach from a complete one — AWS does not fail the call when it
// removes some regions and not others. A caller acting on the result should check it
// rather than treating a nil error as "all replicas gone".
func (r *SecretManagerRepository) RemoveRegionsFromReplication(input *awssecrets.RemoveRegionsFromReplicationInput) (*awssecrets.RemoveRegionsFromReplicationOutput, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("RemoveRegionsFromReplication", cfg.ResourceTypeSecret)).
			Inc()
	}

	output, err := r.smClient().RemoveRegionsFromReplication(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("RemoveRegionsFromReplication", cfg.ResourceTypeSecret)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("RemoveRegionsFromReplication", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// RestoreSecret cancels a scheduled deletion and makes the secret readable again.
//
// DeleteSecret does not destroy anything immediately: it schedules deletion after a
// recovery window of 7 to 30 days, and this undoes that within the window. It is the
// counterpart every caller of DeleteSecretByInput should know about, and the reason a
// deletion driven by automation should always leave the window at its default rather
// than forcing an immediate delete.
func (r *SecretManagerRepository) RestoreSecret(input *awssecrets.RestoreSecretInput) (*awssecrets.RestoreSecretOutput, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("RestoreSecret", cfg.ResourceTypeSecret)).
			Inc()
	}

	output, err := r.smClient().RestoreSecret(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("RestoreSecret", cfg.ResourceTypeSecret)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("RestoreSecret", cfg.ResourceTypeSecret)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// ListSecretsAllIncludingPlannedDeletion lists secrets including those already
// scheduled for deletion, which ListSecretsAll leaves out.
//
// ListSecrets excludes scheduled-for-deletion secrets by default, and that default is
// the right one for enumeration — a secret in its recovery window should not be
// reported as live inventory. This is the deliberate opposite: the secrets it adds are
// exactly the ones with DeletedDate set, so a caller can report what is pending
// permanent deletion and still restorable. Read DeletedDate to tell the two apart.
func (r *SecretManagerRepository) ListSecretsAllIncludingPlannedDeletion() ([]SecretEntry, error) {
	return r.ListSecretsByInput(&awssecrets.ListSecretsInput{IncludePlannedDeletion: aws.Bool(true)})
}
