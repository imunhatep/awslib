package cloudfront

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListConnectionGroupsAll lists every connection group in the account.
func (r *CloudFrontRepository) ListConnectionGroupsAll() ([]ConnectionGroup, error) {
	return r.ListConnectionGroupsByInput(&awscf.ListConnectionGroupsInput{})
}

func (r *CloudFrontRepository) ListConnectionGroupsByInput(query *awscf.ListConnectionGroupsInput) ([]ConnectionGroup, error) {
	start := time.Now()

	var groups []ConnectionGroup

	p := awscf.NewListConnectionGroupsPaginator(r.cloudFrontClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListConnectionGroups", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
		}

		output, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListConnectionGroups", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
			}

			return groups, classifyErr(err)
		}

		for _, summary := range output.ConnectionGroups {
			groups = append(groups, NewConnectionGroup(r.client, connectionGroupFromSummary(summary), aws.ToString(summary.ETag)))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListConnectionGroups", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Add(float64(len(groups)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListConnectionGroupsByInput", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return groups, nil
}

func (r *CloudFrontRepository) GetConnectionGroupByInput(query *awscf.GetConnectionGroupInput) (*ConnectionGroup, error) {
	start := time.Now()

	if query.Identifier == nil || aws.ToString(query.Identifier) == "" {
		return nil, errors.New("Identifier cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
	}

	output, err := r.cloudFrontClient().GetConnectionGroup(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.ConnectionGroup == nil {
		return nil, nil
	}

	group := NewConnectionGroup(r.client, *output.ConnectionGroup, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Inc()

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetConnectionGroupByInput", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return &group, nil
}

// GetConnectionGroup reads a connection group by ARN, ID or name.
func (r *CloudFrontRepository) GetConnectionGroup(identifier string) (*ConnectionGroup, error) {
	return r.GetConnectionGroupByInput(&awscf.GetConnectionGroupInput{
		Identifier: aws.String(identifier),
	})
}

// GetConnectionGroupByRoutingEndpoint resolves the group behind a CloudFront
// routing endpoint — the reverse of the lookup used when writing DNS.
func (r *CloudFrontRepository) GetConnectionGroupByRoutingEndpoint(routingEndpoint string) (*ConnectionGroup, error) {
	start := time.Now()

	if routingEndpoint == "" {
		return nil, errors.New("RoutingEndpoint cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetConnectionGroupByRoutingEndpoint", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
	}

	output, err := r.cloudFrontClient().GetConnectionGroupByRoutingEndpoint(r.ctx, &awscf.GetConnectionGroupByRoutingEndpointInput{
		RoutingEndpoint: aws.String(routingEndpoint),
	})
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetConnectionGroupByRoutingEndpoint", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.ConnectionGroup == nil {
		return nil, nil
	}

	group := NewConnectionGroup(r.client, *output.ConnectionGroup, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetConnectionGroupByRoutingEndpoint", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return &group, nil
}

func (r *CloudFrontRepository) CreateConnectionGroup(input *awscf.CreateConnectionGroupInput) (*ConnectionGroup, error) {
	start := time.Now()

	if aws.ToString(input.Name) == "" {
		return nil, errors.New("Name cannot be empty")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CreateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
	}

	output, err := r.cloudFrontClient().CreateConnectionGroup(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CreateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.ConnectionGroup == nil {
		return nil, nil
	}

	group := NewConnectionGroup(r.client, *output.ConnectionGroup, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return &group, nil
}

// UpdateConnectionGroup applies a caller-built update. IfMatch must carry the
// current ETag.
func (r *CloudFrontRepository) UpdateConnectionGroup(input *awscf.UpdateConnectionGroupInput) (*ConnectionGroup, error) {
	start := time.Now()

	if aws.ToString(input.Id) == "" {
		return nil, errors.New("Id cannot be empty")
	}

	if aws.ToString(input.IfMatch) == "" {
		return nil, errors.New("IfMatch cannot be empty: updates require the current ETag")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("UpdateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
	}

	output, err := r.cloudFrontClient().UpdateConnectionGroup(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
		}

		return nil, classifyErr(err)
	}

	if output.ConnectionGroup == nil {
		return nil, nil
	}

	group := NewConnectionGroup(r.client, *output.ConnectionGroup, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("UpdateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return &group, nil
}

// DeleteConnectionGroupByInput deletes a connection group with a
// caller-supplied ETag.
func (r *CloudFrontRepository) DeleteConnectionGroupByInput(input *awscf.DeleteConnectionGroupInput) error {
	start := time.Now()

	if aws.ToString(input.Id) == "" {
		return errors.New("Id cannot be empty")
	}

	if aws.ToString(input.IfMatch) == "" {
		return errors.New("IfMatch cannot be empty: deletes require the current ETag")
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DeleteConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
	}

	_, err := r.cloudFrontClient().DeleteConnectionGroup(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).Inc()
		}

		return classifyErr(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DeleteConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return nil
}

// DeleteConnectionGroup disables the group if needed, then deletes it. Refuses
// to touch the account's default group, and treats an already-absent group as
// success. A group that still has tenants attached fails with ErrInUse.
func (r *CloudFrontRepository) DeleteConnectionGroup(identifier string) error {
	group, err := r.GetConnectionGroup(identifier)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}

		return err
	}

	if group == nil {
		return nil
	}

	if group.IsDefaultGroup() {
		return errors.Errorf("connection group %s is the account default and cannot be deleted", group.GetId())
	}

	if group.IsEnabled() {
		log.Debug().
			Str("connection_group", group.GetId()).
			Msg("[CloudFrontRepository.DeleteConnectionGroup] disabling connection group before deletion")

		// The update response carries the new version, so no re-read is needed
		// to get a usable ETag for the delete.
		if group, err = r.UpdateConnectionGroup(&awscf.UpdateConnectionGroupInput{
			Id:      group.Id,
			IfMatch: aws.String(group.ETag),
			Enabled: aws.Bool(false),
		}); err != nil {
			return err
		}

		if group == nil {
			return nil
		}
	}

	err = r.DeleteConnectionGroupByInput(&awscf.DeleteConnectionGroupInput{
		Id:      group.Id,
		IfMatch: aws.String(group.ETag),
	})
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	return err
}
