package provider

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/provider/v3"
	"github.com/rs/zerolog/log"
)

// policyDocument represents an IAM policy document
type policyDocument struct {
	Version   string      `json:"Version"`
	Statement []statement `json:"Statement"`
}

// statement represents a statement in an IAM policy
type statement struct {
	Effect   string      `json:"Effect"`
	Action   interface{} `json:"Action"`
	Resource interface{} `json:"Resource"`
}

// AssumableRoles sets the roles that can be assumed for cross-account access.
// This allows you to either manually specify roles or use DiscoverAssumableRolesFromCurrentRole().
//
// Example manual usage:
//
//	roles := map[types.AwsAccountID]types.RoleArn{
//	    "123456789012": "arn:aws:iam::123456789012:role/MyRole",
//	    "987654321098": "arn:aws:iam::987654321098:role/MyRole",
//	}
//	NewClientPool(ctx, clientBuilder, roles)
//
// Example auto-discovery:
//
//	roles, _ := v3.DiscoverAssumableRolesFromCurrentRole(ctx, defaultClient)
//	v3.NewClientPool(ctx, clientBuilder, roles)

// DiscoverAssumableRolesFromCurrentRole discovers IAM roles that can be assumed by the current role
// by parsing the IAM policies attached to it. This is useful for EKS/IRSA deployments where you want
// to automatically discover cross-account roles.
//
// Example usage:
//
//	client, _ := v3.NewClient(ctx)
//	roles, _ := v3.DiscoverAssumableRolesFromCurrentRole(ctx, client)
//	clientPool := v3.NewClientPool(ctx, builder)
//	clientPool.SetAssumableRoles(roles)
func DiscoverAssumableRolesFromCurrentRole(ctx context.Context, client *v3.Client) (map[types.AwsAccountID]types.RoleArn, error) {
	// Get the caller identity
	callerIdentity, err := client.GetCallerIdentity(ctx)
	if err != nil {
		return nil, errors.New(err)
	}

	// Parse the callerIdentity to fetch role ARN
	roleArn, err := arn.Parse(aws.ToString(callerIdentity.Arn))
	if err != nil {
		return nil, errors.New(err)
	}

	// Extract role name from ARN (format: "role/RoleName")
	var roleName string
	if parts := strings.Split(roleArn.Resource, "/"); len(parts) > 1 {
		roleName = parts[1]
	} else {
		return nil, errors.Errorf("invalid role ARN format: %s", aws.ToString(callerIdentity.Arn))
	}

	log.Debug().
		Str("roleArn", aws.ToString(callerIdentity.Arn)).
		Str("roleName", roleName).
		Msg("[DiscoverAssumableRolesFromCurrentRole] discovering assumable roles from IAM policies")

	// Discover assumable roles from IAM policies
	roleArnList, err := discoverAssumableRoles(ctx, client, roleName)
	if err != nil {
		return nil, errors.New(err)
	}

	// Convert list to map by account ID
	roles := make(map[types.AwsAccountID]types.RoleArn)
	for _, assumedRoleArn := range roleArnList {
		parsedArn, err := arn.Parse(assumedRoleArn.String())
		if err != nil {
			log.Warn().Err(err).
				Str("roleArn", assumedRoleArn.String()).
				Msg("[DiscoverAssumableRolesFromCurrentRole] failed to parse role ARN, skipping")
			continue
		}

		accountID := types.AwsAccountID(parsedArn.AccountID)
		roles[accountID] = assumedRoleArn

		log.Debug().
			Str("accountID", string(accountID)).
			Str("roleArn", assumedRoleArn.String()).
			Msg("[DiscoverAssumableRolesFromCurrentRole] discovered assumable role")
	}

	log.Info().
		Int("count", len(roles)).
		Msg("[DiscoverAssumableRolesFromCurrentRole] discovered assumable roles")

	return roles, nil
}

// discoverAssumableRoles discovers IAM roles that can be assumed by parsing attached role policies
func discoverAssumableRoles(ctx context.Context, client *v3.Client, roleName string) ([]types.RoleArn, error) {
	iamClient := client.GetIAMClient()

	// List attached role policies
	policies, err := listAttachedRolePolicies(ctx, iamClient, roleName)
	if err != nil {
		return nil, errors.New(err)
	}

	roleArns := []types.RoleArn{}

	// For each policy, get its version and parse for AssumeRole permissions
	for _, policy := range policies {
		arns, err := extractAssumeRoleArnsFromPolicy(ctx, iamClient, policy)
		if err != nil {
			log.Warn().Err(err).
				Str("policyArn", aws.ToString(policy.PolicyArn)).
				Msg("[discoverAssumableRoles] failed to extract assume role ARNs from policy, skipping")
			continue
		}
		roleArns = append(roleArns, arns...)
	}

	return roleArns, nil
}

// listAttachedRolePolicies lists all policies attached to a role
func listAttachedRolePolicies(ctx context.Context, iamClient *iam.Client, roleName string) ([]iamtypes.AttachedPolicy, error) {
	var policies []iamtypes.AttachedPolicy

	paginator := iam.NewListAttachedRolePoliciesPaginator(iamClient, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})

	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, errors.New(err)
		}
		policies = append(policies, resp.AttachedPolicies...)
	}

	return policies, nil
}

// extractAssumeRoleArnsFromPolicy extracts role ARNs that can be assumed from a policy
func extractAssumeRoleArnsFromPolicy(ctx context.Context, iamClient *iam.Client, policy iamtypes.AttachedPolicy) ([]types.RoleArn, error) {
	// Get the default version of the policy
	resp, err := iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: policy.PolicyArn,
	})
	if err != nil {
		return nil, errors.New(err)
	}

	// Get the policy version document
	versionResp, err := iamClient.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: policy.PolicyArn,
		VersionId: resp.Policy.DefaultVersionId,
	})
	if err != nil {
		return nil, errors.New(err)
	}

	// Decode the policy document
	doc, err := decodePolicyDocument(aws.ToString(versionResp.PolicyVersion.Document))
	if err != nil {
		return nil, errors.New(err)
	}

	// Extract AssumeRole ARNs from the document
	return extractAssumeRoleArnsFromDocument(doc), nil
}

// decodePolicyDocument decodes a URL-encoded policy document JSON
func decodePolicyDocument(doc string) (policyDocument, error) {
	decodedValue, err := url.QueryUnescape(doc)
	if err != nil {
		log.Error().Err(err).
			Str("document", doc).
			Msg("[decodePolicyDocument] failed to url decode policy document")
		return policyDocument{}, errors.New(err)
	}

	var policyDoc policyDocument
	if err := json.Unmarshal([]byte(decodedValue), &policyDoc); err != nil {
		log.Error().Err(err).
			Str("document", doc).
			Msg("[decodePolicyDocument] failed to unmarshal policy document")
		return policyDocument{}, errors.New(err)
	}

	return policyDoc, nil
}

// extractAssumeRoleArnsFromDocument extracts role ARNs from a policy document
func extractAssumeRoleArnsFromDocument(doc policyDocument) []types.RoleArn {
	assumedRoles := []types.RoleArn{}

	// Iterate over the statements in the policy document
	for _, stmt := range doc.Statement {
		// Check if the Action is "sts:AssumeRole"
		if containsAction(stmt.Action, "sts:AssumeRole") {
			// Resource can be a string or a slice of strings
			switch resource := stmt.Resource.(type) {
			case string:
				assumedRoles = append(assumedRoles, types.RoleArn(resource))
			case []interface{}:
				for _, r := range resource {
					if roleArn, ok := r.(string); ok {
						assumedRoles = append(assumedRoles, types.RoleArn(roleArn))
					} else {
						log.Error().Any("resource", r).Msg("[extractAssumeRoleArnsFromDocument] unexpected type for roleArn")
					}
				}
			}
		}
	}

	return assumedRoles
}

// containsAction checks if an action contains the target action
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
