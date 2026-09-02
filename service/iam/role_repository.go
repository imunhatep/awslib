package iam

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

func (r *IamRepository) ListRolesAll() ([]Role, error) {
	return r.ListRolesByInput(&iam.ListRolesInput{})
}

func (r *IamRepository) ListRolesByInput(query *iam.ListRolesInput) ([]Role, error) {
	start := time.Now()
	var roles []Role

	p := iam.NewListRolesPaginator(r.iamClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListRoles", cfg.ResourceTypeRole)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListRoles", cfg.ResourceTypeRole)).Inc()
			}
			return roles, errors.New(err)
		}

		for _, v := range resp.Roles {
			tags, _ := r.ListRoleTags(v)
			v.Tags = tags

			roles = append(roles, NewRole(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListRoles", cfg.ResourceTypeRole)).
			Add(float64(len(roles)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListRolesByInput", cfg.ResourceTypeRole)).
			Observe(time.Since(start).Seconds())
	}

	return roles, nil
}

func (r *IamRepository) ListRoleTags(role types.Role) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("ListRoleTags", cfg.ResourceTypeRole)).
			Inc()
	}

	query := &iam.ListRoleTagsInput{RoleName: role.RoleName}
	tagOutput, err := r.iamClient().ListRoleTags(r.ctx, query)
	if err != nil {
		log.Debug().Str("role", aws.ToString(role.RoleName)).Err(err).Msg("failed to fetch iam role tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}

// DescribeRoleByArn reads one role, given either its ARN or its bare name.
//
// GetRole is keyed on RoleName and rejects an ARN, so the name has to be extracted
// first: a role ARN's resource segment is "role/<path>/<name>", and only the last
// segment is a name AWS will accept. Passing the ARN straight through — which this used
// to do — worked only when the caller had in fact handed over a name.
//
// Both forms are accepted, so callers relying on the form that worked keep working.
func (r *IamRepository) DescribeRoleByArn(roleArn string) (*Role, error) {
	return r.DescribeRoleByInput(&iam.GetRoleInput{RoleName: aws.String(RoleNameFromArn(roleArn))})
}

// RoleNameFromArn extracts the role name from a role ARN, or returns value unchanged
// when it is not an ARN.
//
// The path goes with the prefix: "arn:aws:iam::1234:role/svc/deploy" names the role
// "deploy", not "svc/deploy".
func RoleNameFromArn(value string) string {
	if !arn.IsARN(value) {
		return value
	}

	parsed, err := arn.Parse(value)
	if err != nil {
		return value
	}

	if i := strings.LastIndex(parsed.Resource, "/"); i >= 0 {
		return parsed.Resource[i+1:]
	}

	return parsed.Resource
}

// ListRolesAllWithLastUsed lists every role with RoleLastUsed populated.
//
// ListRoles leaves RoleLastUsed nil — only GetRole fills it in — so a caller scoring
// roles by dormancy cannot use ListRolesAll for it and would read every role as never
// used. This pays one GetRole per role to get the field, the one place in this package
// where a per-resource fan-out is unavoidable.
//
// It is a List* method so the generated cached wrapper covers it, which keeps the
// fan-out to once per cache TTL rather than once per read cycle. A role whose GetRole
// fails keeps whatever ListRoles returned rather than being dropped: a partial answer
// about a role that exists beats omitting the role.
func (r *IamRepository) ListRolesAllWithLastUsed() ([]Role, error) {
	roles, err := r.ListRolesAll()
	if err != nil {
		return roles, err
	}

	for i, role := range roles {
		detailed, err := r.DescribeRoleByInput(&iam.GetRoleInput{RoleName: role.RoleName})
		if err != nil {
			log.Warn().
				Err(err).
				Str("role", role.GetName()).
				Msg("[IamRepository.ListRolesAllWithLastUsed] failed to read role, RoleLastUsed left empty")

			continue
		}

		roles[i] = *detailed
	}

	return roles, nil
}

func (r *IamRepository) DescribeRoleByInput(query *iam.GetRoleInput) (*Role, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetRole", cfg.ResourceTypeRole)).Inc()
	}

	resp, err := r.iamClient().GetRole(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetRole", cfg.ResourceTypeRole)).Inc()
		}
		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetRole", cfg.ResourceTypeRole)).
			Add(1)

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetRoleByInput", cfg.ResourceTypeRole)).
			Observe(time.Since(start).Seconds())
	}

	tags, _ := r.ListRoleTags(*resp.Role)
	resp.Role.Tags = tags

	role := NewRole(r.client, *resp.Role)

	return &role, nil
}
