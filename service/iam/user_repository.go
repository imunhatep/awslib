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

func (r *IamRepository) ListUsersAll() ([]User, error) {
	return r.ListUsersByInput(&iam.ListUsersInput{})
}

func (r *IamRepository) ListUsersByInput(query *iam.ListUsersInput) ([]User, error) {
	start := time.Now()
	var users []User

	p := iam.NewListUsersPaginator(r.client.IAM(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListUsers", cfg.ResourceTypeUser)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListUsers", cfg.ResourceTypeUser)).Inc()
			}
			return users, errors.New(err)
		}

		for _, v := range resp.Users {
			tags, _ := r.ListUserTags(v)
			v.Tags = tags

			users = append(users, NewUser(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListUsers", cfg.ResourceTypeUser)).
			Add(float64(len(users)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListUsersByInput", cfg.ResourceTypeUser)).
			Observe(time.Since(start).Seconds())
	}

	return users, nil
}

func (r *IamRepository) ListUserTags(user types.User) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("ListUserTags", cfg.ResourceTypeUser)).
			Inc()
	}

	query := &iam.ListUserTagsInput{UserName: user.UserName}
	tagOutput, err := r.client.IAM().ListUserTags(r.ctx, query)
	if err != nil {
		log.Debug().Str("user", aws.ToString(user.UserName)).Err(err).Msg("failed to fetch iam user tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}
