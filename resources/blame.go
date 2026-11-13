package resources

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/awslib/service/cloudtrail"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

type AwsClientPool interface {
	GetContext() context.Context
	GetClient(ptypes.AwsAccountID, ptypes.AwsRegion) (*v3.Client, error)
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

func (b *AwsBlame) LookupAll(items ...service.ResourceInterface) (map[string]*ResourceEvents, error) {
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

func (b *AwsBlame) Lookup(resource service.ResourceInterface) (*ResourceEvents, error) {
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

	// errors.New(nil) is a non-nil "<nil>" error, which made every lookup fail and LookupAll stop
	// after the first resource
	events, err := eventsRepo.ListEventsByLookup(lookup)
	if err != nil {
		return NewResourceEvents(resource, events...), errors.New(err)
	}

	return NewResourceEvents(resource, events...), nil
}

type ResourceEvents struct {
	resource service.ResourceInterface
	events   []cloudtrail.Event
}

func NewResourceEvents(resource service.ResourceInterface, events ...cloudtrail.Event) *ResourceEvents {
	return &ResourceEvents{resource: resource, events: events}
}

func (r *ResourceEvents) GetEvents() []cloudtrail.Event {
	return r.events
}

func (r *ResourceEvents) GetEntity() service.ResourceInterface {
	return r.resource
}

// GetCreationEvent is the earliest successful write event in the lookup window, the call that
// created the resource. LookupEvents lists the newest event first, so taking the head would name
// whoever touched the resource last, e.g. a tagging automation seconds after creation. Failed
// calls are skipped: an AccessDenied create attempt before the real one created nothing.
func (r *ResourceEvents) GetCreationEvent() (cloudtrail.Event, bool) {
	var creation cloudtrail.Event
	found := false

	for _, e := range r.events {
		if e.IsReadOnly() || e.IsFailed() {
			continue
		}

		if !found || e.GetTime().Before(creation.GetTime()) {
			creation, found = e, true
		}
	}

	return creation, found
}

func (r *ResourceEvents) GetUsername() string {
	if creator, ok := r.GetCreationEvent(); ok {
		return creator.GetUsername()
	}

	return ResourceCreatorUnknown
}

func (r *ResourceEvents) GetUser() string {
	parts := strings.Split(r.GetUsername(), "@")

	return slice.Head(parts).OrEmpty()
}
