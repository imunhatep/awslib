package iam

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
	"net/url"
	"time"
)

func (r *IamRepository) ListPoliciesAll() ([]Policy, error) {
	return r.ListPoliciesByInput(&iam.ListPoliciesInput{})
}

func (r *IamRepository) ListPoliciesByInput(query *iam.ListPoliciesInput) ([]Policy, error) {
	start := time.Now()
	var policies []Policy

	p := iam.NewListPoliciesPaginator(r.client.IAM(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListPolicies", cfg.ResourceTypePolicy)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListPolicies", cfg.ResourceTypePolicy)).Inc()
			}

			return policies, errors.New(err)
		}

		for _, v := range resp.Policies {
			policies = append(policies, NewPolicy(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListPolicies", cfg.ResourceTypePolicy)).
			Add(float64(len(policies)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListPoliciesByInput", cfg.ResourceTypePolicy)).
			Observe(time.Since(start).Seconds())
	}

	return policies, nil
}

func (r *IamRepository) ListPolicyTags(policy types.Policy) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("ListPolicyTags", cfg.ResourceTypePolicy)).
			Inc()
	}

	query := &iam.ListPolicyTagsInput{PolicyArn: policy.Arn}
	tagOutput, err := r.client.IAM().ListPolicyTags(r.ctx, query)
	if err != nil {
		log.Debug().Str("policy", aws.ToString(policy.PolicyName)).Err(err).Msg("failed to fetch iam policy tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}

func (r *IamRepository) DescribePolicyByInput(query *iam.GetPolicyInput) (*Policy, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetPolicy", cfg.ResourceTypePolicy)).Inc()
	}

	resp, err := r.client.IAM().GetPolicy(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetPolicy", cfg.ResourceTypePolicy)).Inc()
		}
		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetPolicy", cfg.ResourceTypePolicy)).
			Add(1)

		metrics.AwsRepoCallDuration.
			With(r.promLabels("DescribePolicyByInput", cfg.ResourceTypePolicy)).
			Observe(time.Since(start).Seconds())
	}

	policy := NewPolicy(r.client, *resp.Policy)

	return &policy, nil
}

func (r *IamRepository) ListAttachedRolePoliciesByRole(role Role) ([]Policy, error) {
	return r.ListAttachedRolePoliciesByInput(&iam.ListAttachedRolePoliciesInput{RoleName: role.RoleName})
}

func (r *IamRepository) ListAttachedRolePoliciesByInput(query *iam.ListAttachedRolePoliciesInput) ([]Policy, error) {
	start := time.Now()
	var policies []Policy

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListAttachedRolePolicies", cfg.ResourceTypePolicy)).Inc()
	}

	policiesOutput, err := r.client.IAM().ListAttachedRolePolicies(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListAttachedRolePolicies", cfg.ResourceTypePolicy)).Inc()
		}
		return policies, errors.New(err)
	}

	for _, attachedPolicy := range policiesOutput.AttachedPolicies {
		policy, err := r.DescribePolicyByInput(&iam.GetPolicyInput{PolicyArn: attachedPolicy.PolicyArn})
		if err != nil {
			log.Debug().Str("policy", aws.ToString(attachedPolicy.PolicyArn)).Err(err).Msg("failed to fetch iam policy")
			continue
		}

		policies = append(policies, *policy)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListPolicies", cfg.ResourceTypePolicy)).
			Add(float64(len(policies)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListPoliciesByInput", cfg.ResourceTypePolicy)).
			Observe(time.Since(start).Seconds())
	}

	return policies, nil
}

func (r *IamRepository) ListAttachedRolePolicyVersionsByRoleName(name string) ([]PolicyVersion, error) {
	return r.ListAttachedRolePolicyVersionsByInput(&iam.ListAttachedRolePoliciesInput{RoleName: aws.String(name)})
}

func (r *IamRepository) ListAttachedRolePolicyVersionsByRole(role Role) ([]PolicyVersion, error) {
	return r.ListAttachedRolePolicyVersionsByInput(&iam.ListAttachedRolePoliciesInput{RoleName: role.RoleName})
}

func (r *IamRepository) ListAttachedRolePolicyVersionsByInput(query *iam.ListAttachedRolePoliciesInput) ([]PolicyVersion, error) {
	policyVersions := []PolicyVersion{}

	policies, err := r.ListAttachedRolePoliciesByInput(query)
	if err != nil {
		return nil, errors.New(err)
	}

	for _, policy := range policies {
		policyVersion, err := r.DescribePolicyVersion(policy)
		if err != nil {
			log.Error().Err(err).Str("policy", policy.GetArn()).Msg("[IamRepository.ListAttachedRolePolicyVersionsByInput] failed to fetch iam policy version")
			continue
		}

		policyVersions = append(policyVersions, *policyVersion)
	}

	return policyVersions, nil
}

func (r *IamRepository) DescribePolicyVersion(policy Policy) (*PolicyVersion, error) {
	return r.DescribePolicyVersionByInput(&iam.GetPolicyVersionInput{PolicyArn: policy.Arn, VersionId: policy.DefaultVersionId})
}

func (r *IamRepository) DescribePolicyVersionByInput(query *iam.GetPolicyVersionInput) (*PolicyVersion, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetPolicyVersion", cfg.ResourceTypePolicy)).Inc()
	}

	resp, err := r.client.IAM().GetPolicyVersion(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetPolicyVersion", cfg.ResourceTypePolicy)).Inc()
		}
		return nil, errors.New(err)
	}

	// Decode the policy document JSON
	policyDocument, err := decodePolicyDocument(aws.ToString(resp.PolicyVersion.Document))
	if err != nil {
		return nil, errors.New(err)
	}

	policyVersion := NewPolicyVersion(*resp.PolicyVersion, policyDocument)

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetPolicyVersion", cfg.ResourceTypePolicy)).
			Add(1)

		metrics.AwsRepoCallDuration.
			With(r.promLabels("DescribePolicyVersionByInput", cfg.ResourceTypePolicy)).
			Observe(time.Since(start).Seconds())
	}

	return &policyVersion, nil
}

func (r *IamRepository) ListAssumedRoleArn(policyVersion PolicyVersion) []RoleArn {
	assumedRoles := []RoleArn{}

	// Iterate over the statements in the policy document
	for _, statement := range policyVersion.GetDocument().Statement {
		// Check if the Action is "sts:AssumeRole"
		if containsAction(statement.Action, "sts:AssumeRole") {
			// Resource can be a string or a slice of strings
			switch statementResourceType := statement.Resource.(type) {
			case string:
				assumedRoles = append(assumedRoles, RoleArn(statementResourceType))
			case []interface{}:
				for _, resourceType := range statementResourceType {
					if roleArn, ok := resourceType.(string); ok {
						assumedRoles = append(assumedRoles, RoleArn(roleArn))
					} else {
						log.Error().Any("resource", r).Msg("[IamRepository.ListAssumedRoleArn] unexpected type for roleArn")
					}
				}
			}
		}
	}

	return assumedRoles
}

func decodePolicyDocument(doc string) (PolicyDocument, error) {
	decodedValue, err := url.QueryUnescape(doc)
	if err != nil {
		log.Error().Err(err).
			Str("document", doc).
			Msg("[IamRepository.DescribePolicyVersion] failed to url decode policy document")

		return PolicyDocument{}, errors.New(err)
	}

	var policyDoc PolicyDocument
	if err := json.Unmarshal([]byte(decodedValue), &policyDoc); err != nil {
		log.Error().Err(err).
			Str("document", doc).
			Msg("[IamRepository.DescribePolicyVersion] failed to unmarshal policy document")

		return PolicyDocument{}, errors.New(err)
	}

	return policyDoc, nil
}

// Helper function to check if an action contains "sts:AssumeRole"
func containsAction(action interface{}, targetAction string) bool {
	switch act := action.(type) {
	case string:
		return act == targetAction
	case []interface{}:
		for _, a := range act {
			if aStr, ok := a.(string); ok && aStr == targetAction {
				return true
			}
		}
	}
	return false
}
