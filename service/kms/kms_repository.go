// Package kms lists KMS keys with their aliases and tags.
//
// KMS access is governed by each key's key policy as well as by IAM, so one
// key in an account can refuse a read that every other key allows. A listing
// therefore keeps an unreadable key, with what ListKeys said about it and the
// error on the entity, instead of either failing the whole listing or
// dropping the key.
package kms

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	kmsclient "github.com/imunhatep/awslib/provider/v3/clients/kms"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// keyDetailConcurrency and keyDetailMaxAttempts shape the per-key reads of a
// listing (DescribeKey, and ListResourceTags for customer managed keys).
// ListKeys returns only IDs and ARNs, so a listing is unavoidably 1+N calls.
// The values follow the WAFv2 per-object reads: a modest fan-out with a retry
// budget large enough that throttling does not end up recorded as a per-key
// read error.
const (
	keyDetailConcurrency = 4
	keyDetailMaxAttempts = 10
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type KmsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewKmsRepository(ctx context.Context, client *v3.Client) *KmsRepository {
	repo := &KmsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *KmsRepository) kmsClient() *awskms.Client {
	return kmsclient.GetClient(r.client)
}

func (r *KmsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *KmsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func withDetailRetries(o *awskms.Options) {
	o.RetryMaxAttempts = keyDetailMaxAttempts
}
