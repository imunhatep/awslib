package proxy

import (
	"context"

	"github.com/imunhatep/awslib/cache"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cloudfront"
	"github.com/imunhatep/gocollection/slice"
)

// The CloudFront configuration objects a distribution references by ID or ARN.
// All are global — the control plane is — so RepoProxyPool.List calls these once
// per account, from us-east-1.

// FindCloudFrontCachePolicies returns the account's cache policies, managed and custom
func FindCloudFrontCachePolicies(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListCachePoliciesAll()
		return slice.Map(items, cast[cloudfront.CachePolicy]), err
	}
	items, err := repo.ListCachePoliciesAll()
	return slice.Map(items, cast[cloudfront.CachePolicy]), err
}

// FindCloudFrontOriginRequestPolicies returns the account's origin request policies, managed and custom
func FindCloudFrontOriginRequestPolicies(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListOriginRequestPoliciesAll()
		return slice.Map(items, cast[cloudfront.OriginRequestPolicy]), err
	}
	items, err := repo.ListOriginRequestPoliciesAll()
	return slice.Map(items, cast[cloudfront.OriginRequestPolicy]), err
}

// FindCloudFrontResponseHeadersPolicies returns the account's response headers policies, managed and custom
func FindCloudFrontResponseHeadersPolicies(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListResponseHeadersPoliciesAll()
		return slice.Map(items, cast[cloudfront.ResponseHeadersPolicy]), err
	}
	items, err := repo.ListResponseHeadersPoliciesAll()
	return slice.Map(items, cast[cloudfront.ResponseHeadersPolicy]), err
}

// FindCloudFrontFunctions returns CloudFront Functions, one row per stage (LIVE and DEVELOPMENT)
func FindCloudFrontFunctions(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListFunctionsAll()
		return slice.Map(items, cast[cloudfront.Function]), err
	}
	items, err := repo.ListFunctionsAll()
	return slice.Map(items, cast[cloudfront.Function]), err
}

// FindCloudFrontOriginAccessControls returns origin access controls (OAC)
func FindCloudFrontOriginAccessControls(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListOriginAccessControlsAll()
		return slice.Map(items, cast[cloudfront.OriginAccessControl]), err
	}
	items, err := repo.ListOriginAccessControlsAll()
	return slice.Map(items, cast[cloudfront.OriginAccessControl]), err
}

// FindCloudFrontOriginAccessIdentities returns legacy origin access identities (OAI)
func FindCloudFrontOriginAccessIdentities(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListOriginAccessIdentitiesAll()
		return slice.Map(items, cast[cloudfront.OriginAccessIdentity]), err
	}
	items, err := repo.ListOriginAccessIdentitiesAll()
	return slice.Map(items, cast[cloudfront.OriginAccessIdentity]), err
}

// FindCloudFrontKeyGroups returns key groups trusted for signed URLs and cookies
func FindCloudFrontKeyGroups(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListKeyGroupsAll()
		return slice.Map(items, cast[cloudfront.KeyGroup]), err
	}
	items, err := repo.ListKeyGroupsAll()
	return slice.Map(items, cast[cloudfront.KeyGroup]), err
}

// FindCloudFrontPublicKeys returns public keys for signed URLs and cookies
func FindCloudFrontPublicKeys(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListPublicKeysAll()
		return slice.Map(items, cast[cloudfront.PublicKey]), err
	}
	items, err := repo.ListPublicKeysAll()
	return slice.Map(items, cast[cloudfront.PublicKey]), err
}

// FindCloudFrontRealtimeLogConfigs returns real-time log configurations
func FindCloudFrontRealtimeLogConfigs(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListRealtimeLogConfigsAll()
		return slice.Map(items, cast[cloudfront.RealtimeLogConfig]), err
	}
	items, err := repo.ListRealtimeLogConfigsAll()
	return slice.Map(items, cast[cloudfront.RealtimeLogConfig]), err
}

// FindCloudFrontKeyValueStores returns key value stores read by CloudFront Functions
func FindCloudFrontKeyValueStores(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListKeyValueStoresAll()
		return slice.Map(items, cast[cloudfront.KeyValueStore]), err
	}
	items, err := repo.ListKeyValueStoresAll()
	return slice.Map(items, cast[cloudfront.KeyValueStore]), err
}

// FindCloudFrontVpcOrigins returns VPC origins, with their tags
func FindCloudFrontVpcOrigins(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListVpcOriginsAll()
		return slice.Map(items, cast[cloudfront.VpcOrigin]), err
	}
	items, err := repo.ListVpcOriginsAll()
	return slice.Map(items, cast[cloudfront.VpcOrigin]), err
}

// FindCloudFrontContinuousDeploymentPolicies returns continuous deployment (staging) policies
func FindCloudFrontContinuousDeploymentPolicies(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := cloudfront.NewCloudFrontRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListContinuousDeploymentPoliciesAll()
		return slice.Map(items, cast[cloudfront.ContinuousDeploymentPolicy]), err
	}
	items, err := repo.ListContinuousDeploymentPoliciesAll()
	return slice.Map(items, cast[cloudfront.ContinuousDeploymentPolicy]), err
}
