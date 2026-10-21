package route53

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsr53 "github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

func (r *Route53Repository) ListHostedZonesAll() ([]HostedZone, error) {
	return r.ListHostedZonesByInput(&awsr53.ListHostedZonesInput{})
}

func (r *Route53Repository) ListHostedZonesByInput(query *awsr53.ListHostedZonesInput) ([]HostedZone, error) {
	start := time.Now()
	var hostedZones []HostedZone
	marker := ""

	for {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListHostedZones", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		// Set the marker for pagination if this is not the first request
		if marker != "" {
			query.Marker = aws.String(marker)
		}

		output, err := r.route53Client().ListHostedZones(r.ctx, query)
		if err != nil {
			return nil, errors.New(err)
		}

		// Process the current page of results
		for _, hostedZone := range output.HostedZones {
			hostedZones = append(hostedZones, NewHostedZone(r.client, hostedZone, nil, []types.VPC{}, r.hostedZoneTags(hostedZone)))
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiResourcesFetched.
				With(r.promLabels("ListHostedZones", cfg.ResourceTypeRoute53HostedZone)).
				Add(float64(len(output.HostedZones)))
		}

		// Check if there are more results to fetch
		if !output.IsTruncated {
			break
		}

		// Set the marker for the next iteration
		if output.NextMarker != nil {
			marker = *output.NextMarker
		} else {
			break
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListHostedZonesByInput", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return hostedZones, nil
}

// ListHostedZoneTags returns the tags of the hosted zone hostedZoneID, given
// bare ("Z123") or in the "/hostedzone/Z123" form AWS returns. A failed read is
// reported, never answered with "no tags".
//
// ListTagsForResource takes the bare id: the SDK strips the "/hostedzone/"
// prefix for the hosted-zone operations, but not for the tag operations.
func (r *Route53Repository) ListHostedZoneTags(hostedZoneID string) ([]types.Tag, error) {
	start := time.Now()

	id := NormalizeHostedZoneID(hostedZoneID)
	if id == "" {
		return nil, errors.New("hostedZoneID cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeRoute53HostedZone)).Inc()
	}

	output, err := r.route53Client().ListTagsForResource(r.ctx, &awsr53.ListTagsForResourceInput{
		ResourceId:   aws.String(id),
		ResourceType: types.TagResourceTypeHostedzone,
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListTagsForResource", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListHostedZoneTags", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	if output.ResourceTagSet == nil {
		return []types.Tag{}, nil
	}

	return output.ResourceTagSet.Tags, nil
}

// GetHostedZoneTags returns the tags of hostedZone, or nil when they cannot be
// read.
//
// Deprecated: a failed read is indistinguishable from an untagged zone. Use
// ListHostedZoneTags, which reports the error.
func (r *Route53Repository) GetHostedZoneTags(hostedZone types.HostedZone) []types.Tag {
	tags, _ := r.ListHostedZoneTags(aws.ToString(hostedZone.Id))

	return tags
}

// hostedZoneTags reads the tags the zone entity is built with. The zone reads
// keep answering when only the tag read fails, as they always have, but the
// failure is logged rather than dropped: the entity's Tags are then empty for a
// reason other than the zone being untagged.
func (r *Route53Repository) hostedZoneTags(hostedZone types.HostedZone) []types.Tag {
	tags, err := r.ListHostedZoneTags(aws.ToString(hostedZone.Id))
	if err != nil {
		log.Warn().Err(err).
			Str("hostedZone", aws.ToString(hostedZone.Id)).
			Msg("[Route53Repository] failed to read hosted zone tags; zone returned without tags")
	}

	return tags
}

func (r *Route53Repository) GetHostedZoneByInput(query *awsr53.GetHostedZoneInput) (*HostedZone, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetHostedZone", cfg.ResourceTypeRoute53HostedZone)).Inc()
	}

	output, err := r.route53Client().GetHostedZone(r.ctx, query)
	if err != nil {
		return nil, errors.New(err)
	}

	if output.HostedZone == nil {
		return nil, nil
	}

	hostedZone := NewHostedZone(r.client, *output.HostedZone, output.DelegationSet, output.VPCs, r.hostedZoneTags(*output.HostedZone))
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetHostedZone", cfg.ResourceTypeRoute53HostedZone)).
			Inc()

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetHostedZoneByInput", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return &hostedZone, nil
}

// CreateHostedZone creates a new hosted zone
func (r *Route53Repository) CreateHostedZone(input *awsr53.CreateHostedZoneInput) (*HostedZone, error) {
	if aws.ToString(input.CallerReference) == "" {
		return nil, errors.New("CallerReference cannot be empty: hosted zone creation requests must ensure idempotency")
	}

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CreateHostedZone", cfg.ResourceTypeRoute53HostedZone)).Inc()
	}

	output, err := r.route53Client().CreateHostedZone(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CreateHostedZone", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return nil, errors.New(err)
	}

	if output.HostedZone == nil {
		return nil, nil
	}

	// Fetch the created hosted zone with full details
	getQuery := &awsr53.GetHostedZoneInput{Id: output.HostedZone.Id}
	getOutput, err := r.route53Client().GetHostedZone(r.ctx, getQuery)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetHostedZone", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("GetHostedZone", cfg.ResourceTypeRoute53HostedZone)).Add(1)
	}

	hostedZone := NewHostedZone(r.client, *getOutput.HostedZone, getOutput.DelegationSet, getOutput.VPCs, r.hostedZoneTags(*getOutput.HostedZone))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateHostedZone", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return &hostedZone, nil
}

// UpdateHostedZoneComment updates the comment of a hosted zone
func (r *Route53Repository) UpdateHostedZoneComment(input *awsr53.UpdateHostedZoneCommentInput) (*HostedZone, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("UpdateHostedZoneComment", cfg.ResourceTypeRoute53HostedZone)).
			Inc()
	}

	output, err := r.route53Client().UpdateHostedZoneComment(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateHostedZoneComment", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return nil, errors.New(err)
	}

	if output.HostedZone == nil {
		return nil, nil
	}

	// Fetch the updated hosted zone with full details
	getQuery := &awsr53.GetHostedZoneInput{Id: output.HostedZone.Id}
	getOutput, err := r.route53Client().GetHostedZone(r.ctx, getQuery)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetHostedZone", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("GetHostedZone", cfg.ResourceTypeRoute53HostedZone)).Add(1)
	}

	hostedZone := NewHostedZone(r.client, *getOutput.HostedZone, getOutput.DelegationSet, getOutput.VPCs, r.hostedZoneTags(*getOutput.HostedZone))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("UpdateHostedZoneComment", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return &hostedZone, nil
}

// DeleteHostedZoneByInput deletes a hosted zone
func (r *Route53Repository) DeleteHostedZoneByInput(input *awsr53.DeleteHostedZoneInput) error {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DeleteHostedZone", cfg.ResourceTypeRoute53HostedZone)).
			Inc()
	}

	_, err := r.route53Client().DeleteHostedZone(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteHostedZone", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteHostedZone", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}

// ChangeTagsForHostedZone adds or removes tags for a hosted zone
func (r *Route53Repository) ChangeTagsForHostedZone(input *awsr53.ChangeTagsForResourceInput) error {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("ChangeTagsForResource", cfg.ResourceTypeRoute53HostedZone)).
			Inc()
	}

	_, err := r.route53Client().ChangeTagsForResource(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ChangeTagsForResource", cfg.ResourceTypeRoute53HostedZone)).Inc()
		}

		return errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ChangeTagsForResource", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}
