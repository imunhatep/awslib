package cloudfront

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListConnectionGroupsAll lists every connection group in the account.
func (r *CloudFrontRepository) ListConnectionGroupsAll() ([]ConnectionGroup, error) {
	return r.ListConnectionGroupsByInput(&awscf.ListConnectionGroupsInput{})
}

// ListConnectionGroupsByInput lists connection groups together with their tags.
//
// The list response carries no tags, so they come from one tagging sweep (see
// bulkTags). A group whose tags cannot be read fails the listing instead of
// being returned untagged.
func (r *CloudFrontRepository) ListConnectionGroupsByInput(query *awscf.ListConnectionGroupsInput) ([]ConnectionGroup, error) {
	start := time.Now()

	summaries, err := r.listConnectionGroupSummaries(query)
	if err != nil {
		return nil, err
	}

	arns := make([]string, len(summaries))
	for i, summary := range summaries {
		arns[i] = aws.ToString(summary.Arn)
	}

	tags, err := r.bulkTags(arns, connectionGroupTagSource)
	if err != nil {
		return nil, err
	}

	groups := make([]ConnectionGroup, 0, len(summaries))
	for i, summary := range summaries {
		group := connectionGroupFromSummary(summary)
		group.Tags = &cftypes.Tags{Items: tags[i]}

		groups = append(groups, NewConnectionGroup(r.client, group, aws.ToString(summary.ETag)))
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

func (r *CloudFrontRepository) listConnectionGroupSummaries(query *awscf.ListConnectionGroupsInput) ([]cftypes.ConnectionGroupSummary, error) {
	var summaries []cftypes.ConnectionGroupSummary

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

			return nil, classifyErr(err)
		}

		summaries = append(summaries, output.ConnectionGroups...)
	}

	return summaries, nil
}

// GetConnectionGroupByInput reads a connection group together with its tags.
func (r *CloudFrontRepository) GetConnectionGroupByInput(query *awscf.GetConnectionGroupInput) (*ConnectionGroup, error) {
	return r.getConnectionGroup(query, true)
}

// getConnectionGroup reads a connection group, with its tags when withTags is
// set. The delete sequence reads without them, as for tenants.
func (r *CloudFrontRepository) getConnectionGroup(query *awscf.GetConnectionGroupInput, withTags bool) (*ConnectionGroup, error) {
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

	read := *output.ConnectionGroup
	if withTags {
		if read, err = r.withConnectionGroupTags(read); err != nil {
			return nil, err
		}
	}

	group := NewConnectionGroup(r.client, read, aws.ToString(output.ETag))

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

	tagged, err := r.withConnectionGroupTags(*output.ConnectionGroup)
	if err != nil {
		return nil, err
	}

	group := NewConnectionGroup(r.client, tagged, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetConnectionGroupByRoutingEndpoint", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return &group, nil
}

// withConnectionGroupTags fills in the tags of a connection group read with a
// Get, with ListTagsForResource when the response carries none — the same
// treatment, and the same refusal to report an unreadable tag set as an empty
// one, as withTenantTags.
func (r *CloudFrontRepository) withConnectionGroupTags(group cftypes.ConnectionGroup) (cftypes.ConnectionGroup, error) {
	tags, err := r.completeTags(
		"connection group "+aws.ToString(group.Id),
		aws.ToString(group.Arn),
		group.Tags,
		ccfg.ResourceTypeCloudFrontConnectionGroup,
	)
	if err != nil {
		return group, err
	}

	group.Tags = tags

	return group, nil
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

	// A create applies exactly the tags it was sent.
	created := *output.ConnectionGroup
	created.Tags = carryTags(created.Tags, input.Tags)

	group := NewConnectionGroup(r.client, created, aws.ToString(output.ETag))

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("CreateConnectionGroup", ccfg.ResourceTypeCloudFrontConnectionGroup)).
			Observe(time.Since(start).Seconds())
	}

	return &group, nil
}

// UpdateConnectionGroup applies a caller-built update. IfMatch must carry the
// current ETag.
//
// When the response leaves the tags out they are read back, as for
// UpdateDistributionTenant: a failed read-back returns the updated group, with
// its new ETag but without tags, alongside the error.
func (r *CloudFrontRepository) UpdateConnectionGroup(input *awscf.UpdateConnectionGroupInput) (*ConnectionGroup, error) {
	group, err := r.updateConnectionGroup(input)
	if err != nil || group == nil {
		return group, err
	}

	tagged, err := r.withConnectionGroupTags(group.ConnectionGroup)
	if err != nil {
		return group, errors.Errorf("connection group %s was updated (ETag %s), but its tags could not be read: %w", group.GetId(), group.ETag, err)
	}

	group.ConnectionGroup = tagged

	return group, nil
}

// updateConnectionGroup sends an update and returns the group as the response
// describes it, without reading tags.
func (r *CloudFrontRepository) updateConnectionGroup(input *awscf.UpdateConnectionGroupInput) (*ConnectionGroup, error) {
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
//
// No tags are read, as in DeleteDistributionTenant.
func (r *CloudFrontRepository) DeleteConnectionGroup(identifier string) error {
	group, err := r.getConnectionGroup(&awscf.GetConnectionGroupInput{
		Identifier: aws.String(identifier),
	}, false)
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
		if group, err = r.updateConnectionGroup(&awscf.UpdateConnectionGroupInput{
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
