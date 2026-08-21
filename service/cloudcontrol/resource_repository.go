package cloudcontrol

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListResourcesByType lists every resource of resourceType through the Cloud
// Control API, using only the properties the LIST handler returns.
//
// This is the generic path: it works for any AWS::*::* type whose Cloud Control
// registry entry implements LIST, with no per-type Go code. Types that only
// implement READ (or that need parent identifiers to be listed) come back as a
// Cloud Control error rather than an empty list — use ListResourcesByInput with
// a ResourceModel for the latter.
func (r *CloudControlRepository) ListResourcesByType(resourceType cfg.ResourceType) ([]Resource, error) {
	query := &cc.ListResourcesInput{
		TypeName: aws.String(string(resourceType)),
	}

	return r.ListResourcesByInput(query, false)
}

// ListResourcesByTypeDetailed is ListResourcesByType plus a GetResource per
// item, for types whose LIST handler returns only identifiers.
//
// That is one extra API call per resource, so it is a separate method rather
// than the default: callers should reach for it only when they actually need
// full properties (S3 buckets need it, EC2 instances do not).
func (r *CloudControlRepository) ListResourcesByTypeDetailed(resourceType cfg.ResourceType) ([]Resource, error) {
	query := &cc.ListResourcesInput{
		TypeName: aws.String(string(resourceType)),
	}

	return r.ListResourcesByInput(query, true)
}

// ListResourcesByInput is the workhorse behind the two methods above. Passing
// the input directly gives callers ResourceModel (required to list nested types
// such as AWS::ApiGateway::Method) and MaxResults.
//
// Neither a failed detail fetch nor unparsable properties drops the resource:
// both degrade to what was already known and log, because a silently shorter
// list is worse than a resource with missing attributes — the caller cannot tell
// the difference between "dropped" and "does not exist".
func (r *CloudControlRepository) ListResourcesByInput(query *cc.ListResourcesInput, detailed bool) ([]Resource, error) {
	resourceType := cfg.ResourceType(aws.ToString(query.TypeName))

	ccResources, err := r.FindResources(query)
	if err != nil {
		return nil, errors.New(err)
	}

	resources := make([]Resource, 0, len(ccResources))
	for _, ccResource := range ccResources {
		description := ccResource

		if detailed {
			detail, err := r.DescribeResource(resourceType, ccResource.Identifier)
			switch {
			case err != nil:
				log.Warn().Err(err).
					Str("id", aws.ToString(ccResource.Identifier)).
					Str("type", ccfg.ResourceTypeToString(resourceType)).
					Msg("[CloudControlRepository.ListResourcesByInput] failed fetching resource details, keeping list properties")
			case detail.ResourceDescription != nil:
				description = *detail.ResourceDescription
			}
		}

		attributes, tags, err := ParseAttributes(description)
		if err != nil {
			log.Warn().Err(err).
				Str("id", aws.ToString(ccResource.Identifier)).
				Str("type", ccfg.ResourceTypeToString(resourceType)).
				Msg("[CloudControlRepository.ListResourcesByInput] failed parsing resource properties")
		}

		resources = append(resources, NewResource(r.client, resourceType, description, attributes, tags))
	}

	return resources, nil
}
