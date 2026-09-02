package iam

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

// Tag writes, one pair per IAM resource kind.
//
// Each takes the tags or keys computed by TagsToApply / TagKeysToRemove and treats an
// empty slice as "nothing to do", so the no-op case costs no API call and the caller
// does not have to check.
//
// The resource is addressed the way its operation requires: users and roles by name,
// policies by ARN. That asymmetry is IAM's, not a choice here — TagPolicy has no name
// parameter.

// CreateUserTags applies tags to an IAM user. An empty tag slice is a no-op.
func (r *IamRepository) CreateUserTags(userName string, tags []types.Tag) error {
	if len(tags) == 0 {
		return nil
	}

	return r.tagWrite("TagUser", cfg.ResourceTypeUser, userName, func() error {
		_, err := r.iamClient().TagUser(r.ctx, &iam.TagUserInput{
			UserName: aws.String(userName),
			Tags:     tags,
		})

		return err
	})
}

// DeleteUserTags removes tag keys from an IAM user. An empty key slice is a no-op.
func (r *IamRepository) DeleteUserTags(userName string, tagKeys []string) error {
	if len(tagKeys) == 0 {
		return nil
	}

	return r.tagWrite("UntagUser", cfg.ResourceTypeUser, userName, func() error {
		_, err := r.iamClient().UntagUser(r.ctx, &iam.UntagUserInput{
			UserName: aws.String(userName),
			TagKeys:  tagKeys,
		})

		return err
	})
}

// CreateRoleTags applies tags to an IAM role. An empty tag slice is a no-op.
func (r *IamRepository) CreateRoleTags(roleName string, tags []types.Tag) error {
	if len(tags) == 0 {
		return nil
	}

	return r.tagWrite("TagRole", cfg.ResourceTypeRole, roleName, func() error {
		_, err := r.iamClient().TagRole(r.ctx, &iam.TagRoleInput{
			RoleName: aws.String(RoleNameFromArn(roleName)),
			Tags:     tags,
		})

		return err
	})
}

// DeleteRoleTags removes tag keys from an IAM role. An empty key slice is a no-op.
func (r *IamRepository) DeleteRoleTags(roleName string, tagKeys []string) error {
	if len(tagKeys) == 0 {
		return nil
	}

	return r.tagWrite("UntagRole", cfg.ResourceTypeRole, roleName, func() error {
		_, err := r.iamClient().UntagRole(r.ctx, &iam.UntagRoleInput{
			RoleName: aws.String(RoleNameFromArn(roleName)),
			TagKeys:  tagKeys,
		})

		return err
	})
}

// CreatePolicyTags applies tags to a customer-managed IAM policy, addressed by ARN. An
// empty tag slice is a no-op.
func (r *IamRepository) CreatePolicyTags(policyArn string, tags []types.Tag) error {
	if len(tags) == 0 {
		return nil
	}

	return r.tagWrite("TagPolicy", cfg.ResourceTypePolicy, policyArn, func() error {
		_, err := r.iamClient().TagPolicy(r.ctx, &iam.TagPolicyInput{
			PolicyArn: aws.String(policyArn),
			Tags:      tags,
		})

		return err
	})
}

// DeletePolicyTags removes tag keys from a customer-managed IAM policy, addressed by
// ARN. An empty key slice is a no-op.
func (r *IamRepository) DeletePolicyTags(policyArn string, tagKeys []string) error {
	if len(tagKeys) == 0 {
		return nil
	}

	return r.tagWrite("UntagPolicy", cfg.ResourceTypePolicy, policyArn, func() error {
		_, err := r.iamClient().UntagPolicy(r.ctx, &iam.UntagPolicyInput{
			PolicyArn: aws.String(policyArn),
			TagKeys:   tagKeys,
		})

		return err
	})
}

// tagWrite carries the metrics and error wrapping the six methods above share. They
// differ only in which SDK call they make and how the resource is addressed, and
// repeating twenty lines of instrumentation six times is how one of them ends up
// labelled with another's method name.
func (r *IamRepository) tagWrite(method string, resourceType cfg.ResourceType, resourceRef string, call func() error) error {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels(method, resourceType)).Inc()
	}

	if err := call(); err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels(method, resourceType)).Inc()
		}

		return errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels(method, resourceType)).
			Observe(time.Since(start).Seconds())
	}

	log.Debug().
		Str("resource", resourceRef).
		Msgf("[IamRepository.%s] tags written", method)

	return nil
}
