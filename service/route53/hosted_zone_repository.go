package route53

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

func (r *Route53Repository) ListHostedZonesAll() ([]HostedZone, error) {
	return r.ListHostedZonesByInput(&route53.ListHostedZonesInput{})
}

func (r *Route53Repository) ListHostedZonesByInput(query *route53.ListHostedZonesInput) ([]HostedZone, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListHostedZones", cfg.ResourceTypeRoute53HostedZone)).Inc()
	}

	output, err := r.route53Client().ListHostedZones(r.ctx, query)
	if err != nil {
		return nil, errors.New(err)
	}

	var hostedZones []HostedZone
	for _, hostedZone := range output.HostedZones {
		hostedZones = append(hostedZones, NewHostedZone(r.client, hostedZone, []types.VPC{}, r.GetHostedZoneTags(hostedZone)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListHostedZones", cfg.ResourceTypeRoute53HostedZone)).
			Add(float64(len(hostedZones)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListHostedZonesByInput", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return hostedZones, nil
}

func (r *Route53Repository) GetHostedZoneTags(hostedZone types.HostedZone) []types.Tag {
	start := time.Now()

	query := &route53.ListTagsForResourceInput{
		ResourceId:   hostedZone.Id,
		ResourceType: types.TagResourceTypeHostedzone,
	}

	output, err := r.route53Client().ListTagsForResource(r.ctx, query)
	if err != nil {
		return nil
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListTagsForResourceInput", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return output.ResourceTagSet.Tags
}

func (r *Route53Repository) GetHostedZoneByInput(query *route53.GetHostedZoneInput) (*HostedZone, error) {
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

	hostedZone := NewHostedZone(r.client, *output.HostedZone, output.VPCs, r.GetHostedZoneTags(*output.HostedZone))
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
func (r *Route53Repository) CreateHostedZone(input *route53.CreateHostedZoneInput) (*HostedZone, error) {
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
	getQuery := &route53.GetHostedZoneInput{Id: output.HostedZone.Id}
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

	hostedZone := NewHostedZone(r.client, *getOutput.HostedZone, getOutput.VPCs, r.GetHostedZoneTags(*getOutput.HostedZone))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateHostedZone", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return &hostedZone, nil
}

// UpdateHostedZoneComment updates the comment of a hosted zone
func (r *Route53Repository) UpdateHostedZoneComment(input *route53.UpdateHostedZoneCommentInput) (*HostedZone, error) {
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
	getQuery := &route53.GetHostedZoneInput{Id: output.HostedZone.Id}
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

	hostedZone := NewHostedZone(r.client, *getOutput.HostedZone, getOutput.VPCs, r.GetHostedZoneTags(*getOutput.HostedZone))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("UpdateHostedZoneComment", cfg.ResourceTypeRoute53HostedZone)).
			Observe(time.Since(start).Seconds())
	}

	return &hostedZone, nil
}

// DeleteHostedZoneByInput deletes a hosted zone
func (r *Route53Repository) DeleteHostedZoneByInput(input *route53.DeleteHostedZoneInput) error {
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
func (r *Route53Repository) ChangeTagsForHostedZone(input *route53.ChangeTagsForResourceInput) error {
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
