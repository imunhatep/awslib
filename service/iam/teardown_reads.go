package iam

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/go-errors/errors"
)

// The reads a teardown walks, one per dependent-object kind.
//
// All unexported, and that is load-bearing rather than tidiness: the generated cached
// wrapper wraps exported Get*/List* methods, so an exported name here would silently
// acquire a cache, and a teardown driven by a cached list deletes against a stale view.
// Keeping them unexported makes them live by construction.
//
// Each returns only the identifier its delete call needs. The teardown does not want the
// objects, and returning them would invite a caller to make decisions on data that is
// stale the moment it is read.

func (r *IamRepository) userAccessKeyIds(userName string) ([]string, error) {
	var keyIds []string

	p := iam.NewListAccessKeysPaginator(r.iamClient(), &iam.ListAccessKeysInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return keyIds, errors.New(err)
		}

		for _, key := range resp.AccessKeyMetadata {
			keyIds = append(keyIds, aws.ToString(key.AccessKeyId))
		}
	}

	return keyIds, nil
}

func (r *IamRepository) userMfaSerialNumbers(userName string) ([]string, error) {
	var serials []string

	p := iam.NewListMFADevicesPaginator(r.iamClient(), &iam.ListMFADevicesInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return serials, errors.New(err)
		}

		for _, device := range resp.MFADevices {
			serials = append(serials, aws.ToString(device.SerialNumber))
		}
	}

	return serials, nil
}

func (r *IamRepository) userSigningCertificateIds(userName string) ([]string, error) {
	var certIds []string

	p := iam.NewListSigningCertificatesPaginator(r.iamClient(), &iam.ListSigningCertificatesInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return certIds, errors.New(err)
		}

		for _, cert := range resp.Certificates {
			certIds = append(certIds, aws.ToString(cert.CertificateId))
		}
	}

	return certIds, nil
}

func (r *IamRepository) userSshPublicKeyIds(userName string) ([]string, error) {
	var keyIds []string

	p := iam.NewListSSHPublicKeysPaginator(r.iamClient(), &iam.ListSSHPublicKeysInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return keyIds, errors.New(err)
		}

		for _, key := range resp.SSHPublicKeys {
			keyIds = append(keyIds, aws.ToString(key.SSHPublicKeyId))
		}
	}

	return keyIds, nil
}

// userServiceSpecificCredentialIds is the one read here with no paginator: AWS does not
// paginate ListServiceSpecificCredentials, so one call returns the lot.
func (r *IamRepository) userServiceSpecificCredentialIds(userName string) ([]string, error) {
	resp, err := r.iamClient().ListServiceSpecificCredentials(r.ctx, &iam.ListServiceSpecificCredentialsInput{
		UserName: aws.String(userName),
	})
	if err != nil {
		return nil, errors.New(err)
	}

	var credentialIds []string
	for _, credential := range resp.ServiceSpecificCredentials {
		credentialIds = append(credentialIds, aws.ToString(credential.ServiceSpecificCredentialId))
	}

	return credentialIds, nil
}

func (r *IamRepository) userInlinePolicyNames(userName string) ([]string, error) {
	var policyNames []string

	p := iam.NewListUserPoliciesPaginator(r.iamClient(), &iam.ListUserPoliciesInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return policyNames, errors.New(err)
		}

		policyNames = append(policyNames, resp.PolicyNames...)
	}

	return policyNames, nil
}

func (r *IamRepository) userAttachedPolicyArns(userName string) ([]string, error) {
	var policyArns []string

	p := iam.NewListAttachedUserPoliciesPaginator(r.iamClient(), &iam.ListAttachedUserPoliciesInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return policyArns, errors.New(err)
		}

		for _, policy := range resp.AttachedPolicies {
			policyArns = append(policyArns, aws.ToString(policy.PolicyArn))
		}
	}

	return policyArns, nil
}

func (r *IamRepository) userGroupNames(userName string) ([]string, error) {
	var groupNames []string

	p := iam.NewListGroupsForUserPaginator(r.iamClient(), &iam.ListGroupsForUserInput{
		UserName: aws.String(userName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return groupNames, errors.New(err)
		}

		for _, group := range resp.Groups {
			groupNames = append(groupNames, aws.ToString(group.GroupName))
		}
	}

	return groupNames, nil
}

func (r *IamRepository) roleInlinePolicyNames(roleName string) ([]string, error) {
	var policyNames []string

	p := iam.NewListRolePoliciesPaginator(r.iamClient(), &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return policyNames, errors.New(err)
		}

		policyNames = append(policyNames, resp.PolicyNames...)
	}

	return policyNames, nil
}

func (r *IamRepository) roleAttachedPolicyArns(roleName string) ([]string, error) {
	var policyArns []string

	p := iam.NewListAttachedRolePoliciesPaginator(r.iamClient(), &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return policyArns, errors.New(err)
		}

		for _, policy := range resp.AttachedPolicies {
			policyArns = append(policyArns, aws.ToString(policy.PolicyArn))
		}
	}

	return policyArns, nil
}

func (r *IamRepository) roleInstanceProfileNames(roleName string) ([]string, error) {
	var profileNames []string

	p := iam.NewListInstanceProfilesForRolePaginator(r.iamClient(), &iam.ListInstanceProfilesForRoleInput{
		RoleName: aws.String(roleName),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return profileNames, errors.New(err)
		}

		for _, profile := range resp.InstanceProfiles {
			profileNames = append(profileNames, aws.ToString(profile.InstanceProfileName))
		}
	}

	return profileNames, nil
}

// policyNonDefaultVersionIds returns the versions that have to be deleted before
// DeletePolicy will accept the policy.
//
// The default version is excluded because it cannot be deleted on its own —
// DeletePolicyVersion rejects it — and DeletePolicy removes it along with the policy.
func (r *IamRepository) policyNonDefaultVersionIds(policyArn string) ([]string, error) {
	var versionIds []string

	p := iam.NewListPolicyVersionsPaginator(r.iamClient(), &iam.ListPolicyVersionsInput{
		PolicyArn: aws.String(policyArn),
	})
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return versionIds, errors.New(err)
		}

		for _, version := range resp.Versions {
			if version.IsDefaultVersion {
				continue
			}

			versionIds = append(versionIds, aws.ToString(version.VersionId))
		}
	}

	return versionIds, nil
}
