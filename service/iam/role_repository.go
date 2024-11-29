package iam

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *IamRepository) ListRolesAll() ([]Role, error) {
	return r.ListRolesByInput(&iam.ListRolesInput{})
}

func (r *IamRepository) ListRolesByInput(query *iam.ListRolesInput) ([]Role, error) {
	start := time.Now()
	var roles []Role

	p := iam.NewListRolesPaginator(r.client.IAM(), query)
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
	tagOutput, err := r.client.IAM().ListRoleTags(r.ctx, query)
	if err != nil {
		log.Debug().Str("role", aws.ToString(role.RoleName)).Err(err).Msg("failed to fetch iam role tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}

func (r *IamRepository) DescribeRoleByArn(roleArn string) (*Role, error) {
	return r.DescribeRoleByInput(&iam.GetRoleInput{RoleName: aws.String(roleArn)})
}

func (r *IamRepository) DescribeRoleByInput(query *iam.GetRoleInput) (*Role, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetRole", cfg.ResourceTypeRole)).Inc()
	}

	resp, err := r.client.IAM().GetRole(r.ctx, query)
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
