package resources

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/provider/v2"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/awslib/service/cloudtrail"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"strings"
	"time"
)

type AwsClientPool interface {
	GetContext() context.Context
	GetClient(ptypes.AwsAccountID, ptypes.AwsRegion) (*v2.Client, error)
}

const ResourceCreatorUnknown = "unknown"

type AwsBlame struct {
	ctx           context.Context
	clients       AwsClientPool
	ttl           time.Duration
	resourceTypes []types.ResourceType
}

func NewAwsBlame(ctx context.Context, clients AwsClientPool) *AwsBlame {
	return &AwsBlame{ctx: ctx, clients: clients, ttl: 30 * 24 * time.Hour}
}

func (b *AwsBlame) WithTtl(ttl time.Duration) *AwsBlame {
	return &AwsBlame{ctx: b.ctx, clients: b.clients, ttl: ttl}
}

func (b *AwsBlame) WithResourceTypeList(resourceTypes []types.ResourceType) *AwsBlame {
	return &AwsBlame{
		ctx:           b.ctx,
		clients:       b.clients,
		ttl:           b.ttl,
		resourceTypes: resourceTypes,
	}
}

func (b *AwsBlame) getRepo(accountID ptypes.AwsAccountID, region ptypes.AwsRegion) (*cloudtrail.CloudTrailRepository, error) {
	client, err := b.clients.GetClient(accountID, region)
	return cloudtrail.NewCloudTrailRepository(b.ctx, client), err
}

func (b *AwsBlame) LookupAll(items ...service.EntityInterface) (map[string]*ResourceEvents, error) {
	resources := map[string]*ResourceEvents{}
	for _, resource := range items {
		events, err := b.Lookup(resource)
		if err != nil {
			return resources, err
		}

		resources[resource.GetIdOrArn()] = events
	}

	return resources, nil
}

func (b *AwsBlame) Lookup(resource service.EntityInterface) (*ResourceEvents, error) {
	if !slice.IsEmpty(b.resourceTypes) && !slice.Contains(b.resourceTypes, resource.GetType()) {
		log.
			Trace().
			Str("resource", cfg.ResourceTypeToString(resource.GetType())).
			Str("arn", resource.GetArn()).
			Msg("[AwsBlame.Lookup] resource type is not listed, skipping")

		return NewResourceEvents(resource), nil
	}

	if time.Now().Add(-b.ttl).After(resource.GetCreatedAt()) {
		log.
			Trace().
			Str("resource", cfg.ResourceTypeToString(resource.GetType())).
			Str("arn", resource.GetArn()).
			Time("createdAt", resource.GetCreatedAt()).
			Msgf("[AwsBlame.Lookup] resource is older than %d days, skipping", int(b.ttl.Hours()/24))

		return NewResourceEvents(resource), nil
	}

	// get aws cloudtrail repository for resource AWS Region
	eventsRepo, err := b.getRepo(resource.GetAccountID(), resource.GetRegion())
	if err != nil {
		return NewResourceEvents(resource), errors.New(err)
	}

	// build cloudtrail events lookup
	lookup := cloudtrail.NewLookupMiddleware().
		WithStartTime(resource.GetCreatedAt().Add(-2 * time.Minute)).
		WithEndTime(resource.GetCreatedAt().Add(2 * time.Minute)).
		WithResource(resource)

	events, err := eventsRepo.ListEventsByLookup(lookup)

	return NewResourceEvents(resource, events...), errors.New(err)
}

type ResourceEvents struct {
	resource service.EntityInterface
	events   []cloudtrail.Event
}

func NewResourceEvents(resource service.EntityInterface, events ...cloudtrail.Event) *ResourceEvents {
	return &ResourceEvents{resource: resource, events: events}
}

func (r *ResourceEvents) GetEvents() []cloudtrail.Event {
	return r.events
}

func (r *ResourceEvents) GetEntity() service.EntityInterface {
	return r.resource
}

func (r *ResourceEvents) GetUsername() string {
	events := slice.FilterNot(r.events, func(e cloudtrail.Event) bool { return e.IsReadOnly() })
	if creator, ok := slice.Head(events).Get(); ok {
		return creator.GetUsername()
	}

	return ResourceCreatorUnknown
}

func (r *ResourceEvents) GetUser() string {
	parts := strings.Split(r.GetUsername(), "@")

	return slice.Head(parts).OrEmpty()
}
