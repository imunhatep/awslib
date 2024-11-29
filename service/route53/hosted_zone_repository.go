package route53

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/imunhatep/awslib/metrics"
	"time"
)

func (r *Route53Repository) ListHostedZonesAll() ([]HostedZone, error) {
	return r.ListHostedZonesByInput(&route53.ListHostedZonesInput{})
}

func (r *Route53Repository) ListHostedZonesByInput(query *route53.ListHostedZonesInput) ([]HostedZone, error) {
	start := time.Now()

	output, err := r.client.Route53().ListHostedZones(r.ctx, query)
	if err != nil {
		return nil, err
	}

	var hostedZones []HostedZone
	for _, hostedZone := range output.HostedZones {
		hostedZones = append(hostedZones, NewHostedZone(r.client, hostedZone, r.GetHostedZoneTags(hostedZone)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListHostedZones", cfg.ResourceTypeInstance)).
			Add(float64(len(hostedZones)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListHostedZonesByInput", cfg.ResourceTypeInstance)).
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

	output, err := r.client.Route53().ListTagsForResource(r.ctx, query)
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
