package proxy

import (
	"context"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/cache"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	cfgEntity "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/awslib/service/cloudcontrol"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

// FindGenericResources returns resources of any type through the Cloud Control
// API. It is the single implementation behind both RepoProxy.FindAllCC and
// GenericRepoProxy.FindAll, and the type-agnostic counterpart to the Find*
// helpers in helper.go: there is one of those per resource type because each
// wraps a hand-written repository, whereas this one takes the type as an
// argument.
//
// What it gives up relative to a typed repository, so a caller can decide
// knowingly:
//
//   - No ARN unless the type exposes one as a property. ResourceDescription
//     carries only an opaque identifier and the resource-path segment differs per
//     type (instance/, role/, function:, and S3 omits region and account
//     altogether), so a synthesized ARN would be wrong often enough to be worse
//     than none. GetIdOrArn falls back to the identifier.
//   - No creation time. Cloud Control does not report one.
//   - Untyped properties: a map, not an SDK struct.
//   - No LIST handler for every type, and nested types need a ResourceModel.
//
// detailed adds a GetResource call per item, for types whose LIST handler
// returns identifiers only. It costs one API call per resource, so leave it off
// unless the caller needs full properties.
func FindGenericResources(
	ctx context.Context,
	client *v3.Client,
	dc *cache.DataCache,
	resourceType cfg.ResourceType,
	detailed bool,
) ([]service.ResourceInterface, error) {
	repo := cloudcontrol.NewCloudControlRepository(ctx, client)

	// The plain and cached repositories expose the same two methods, so both the
	// cache and the detail level are chosen by picking a method value rather than
	// by duplicating the call four ways.
	var list func(cfg.ResourceType) ([]cloudcontrol.Resource, error)
	switch {
	case dc != nil && detailed:
		list = repo.WithCache(dc).ListResourcesByTypeDetailed
	case dc != nil:
		list = repo.WithCache(dc).ListResourcesByType
	case detailed:
		list = repo.ListResourcesByTypeDetailed
	default:
		list = repo.ListResourcesByType
	}

	found, err := list(resourceType)
	items := slice.Map(found, cast[cloudcontrol.Resource])

	log.Info().
		Str("accountID", client.GetAccountID().String()).
		Str("region", client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(resourceType)).
		Bool("detailed", detailed).
		Msgf("[proxy.FindGenericResources] aws resources found: %d", len(items))

	return items, err
}

// GenericRepoProxy answers FindAll for *any* resource type by going through the
// Cloud Control API, instead of dispatching to a hand-written repository.
//
// It satisfies RepoProxyInterface, which is the whole point: a pool of these
// drops straight into resources.NewProvider, so the generic path reuses the
// existing parallel fan-out, streaming reader, cache and global-resource-type
// collapsing without any of them needing to know it exists. RepoProxy.FindAll
// and RepoProxy.FindAllCC are left untouched.
//
// Cloud Control is a fallback, not a replacement: types whose registry entry has
// no LIST handler return an error rather than an empty list, nested types need a
// ResourceModel this proxy does not supply, and the properties come back as an
// untyped bag rather than a typed SDK struct.
type GenericRepoProxy struct {
	*RepoProxy
	detailed bool
}

// Generic returns a view of this proxy that resolves every resource type through
// Cloud Control. The underlying client, context and cache are shared.
func (e *RepoProxy) Generic(detailed bool) *GenericRepoProxy {
	return &GenericRepoProxy{RepoProxy: e, detailed: detailed}
}

// WithCache mirrors RepoProxy.WithCache so a GenericRepoProxy keeps caching when
// it goes through RepoProxyPool.WithCache.
func (e *GenericRepoProxy) WithCache(dc *cache.DataCache) *GenericRepoProxy {
	return &GenericRepoProxy{RepoProxy: e.RepoProxy.WithCache(dc), detailed: e.detailed}
}

// FindAll shadows RepoProxy.FindAll with the type-agnostic Cloud Control lookup.
// It differs from RepoProxy.FindAllCC only in honouring this proxy's detailed
// setting, which FindAllCC cannot express without breaking FindAll's signature.
func (e *GenericRepoProxy) FindAll(resourceType cfg.ResourceType) ([]service.ResourceInterface, error) {
	return FindGenericResources(e.ctx, e.client, e.cache, resourceType, e.detailed)
}

// NewGenericRepoProxyPool builds a pool that resolves every resource type through
// Cloud Control. Interchangeable with NewRepoProxyPool at the call site.
func NewGenericRepoProxyPool(ctx context.Context, clients []*v3.Client, detailed bool) *RepoProxyPool {
	var services []RepoProxyInterface
	for _, client := range clients {
		log.Trace().
			Str("accountID", client.GetAccountID().String()).
			Str("region", client.GetRegion().String()).
			Msg("[RepoProxyPool.NewGenericRepoProxyPool] adding client to the pool")

		services = append(services, NewRepoProxy(ctx, client).Generic(detailed))
	}

	return &RepoProxyPool{services}
}
