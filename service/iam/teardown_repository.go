package iam

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
)

// Deleting an IAM identity is a transaction with no rollback.
//
// DeleteUser, DeleteRole and DeletePolicy all fail with DeleteConflict unless every
// dependent object is removed first, in a specific order. That order is an AWS API
// constraint, not a policy decision, so it belongs here rather than being reassembled by
// each caller — and keeping it in one place is what makes a retry after a partial
// failure idempotent.
//
// Every step is written to be safe to repeat. The lists are re-read on each attempt, and
// NoSuchEntity is treated as success throughout: a step that deleted the thing and then
// failed to record it must not block the retry that follows.
//
// None of these read the cache. A teardown driven by a cached list would delete against
// a stale view of the world, and for DescribePolicyEntities — the "is anyone still using
// this" gate — a stale answer is the difference between an orphan and a policy some
// other account's role still references. The unexported readers below are unexported
// partly for that reason: the generated cached wrapper only wraps exported Get*/List*
// methods, so naming them this way keeps them live by construction.

// TeardownStepError names the step of a teardown that failed.
//
// A teardown that stops halfway leaves the identity in a partially dismantled state —
// for a user, typically credential-less but still present, which is safe but is not
// what the caller asked for. Reporting which step failed is what lets the caller say so
// rather than reporting a generic delete failure.
type TeardownStepError struct {
	Step     string
	Resource string
	Err      error
}

func (e *TeardownStepError) Error() string {
	return fmt.Sprintf("iam teardown of %q failed at step %s: %v", e.Resource, e.Step, e.Err)
}

func (e *TeardownStepError) Unwrap() error {
	return e.Err
}

func teardownStep(step, resource string, err error) error {
	if err == nil || isBenignNotFound(err) {
		return nil
	}

	return &TeardownStepError{Step: step, Resource: resource, Err: err}
}

// isBenignNotFound reports whether the error means the thing is already gone.
//
// NoSuchEntity is the expected answer for a step whose target was never there — a user
// with no login profile, a role with no permissions boundary — and for a retry after a
// step that succeeded but whose result was not recorded. Either way there is nothing
// left to do, which is success.
func isBenignNotFound(err error) bool {
	if err == nil {
		return true
	}

	var noSuchEntity *types.NoSuchEntityException
	return stderrors.As(err, &noSuchEntity)
}

// DeleteUserWithDependencies removes everything attached to a user, then the user.
//
// The order is AWS's:
//
//  1. DeleteLoginProfile                       (console password)
//  2. ListAccessKeys           → DeleteAccessKey
//  3. ListMFADevices           → DeactivateMFADevice, then DeleteVirtualMFADevice
//  4. ListSigningCertificates  → DeleteSigningCertificate
//  5. ListSSHPublicKeys        → DeleteSSHPublicKey
//  6. ListServiceSpecificCredentials → DeleteServiceSpecificCredential
//  7. ListUserPolicies         → DeleteUserPolicy            (inline)
//  8. ListAttachedUserPolicies → DetachUserPolicy            (managed)
//  9. ListGroupsForUser        → RemoveUserFromGroup
//  10. DeleteUserPermissionsBoundary
//  11. DeleteUser
//
// Credentials go first on purpose. If the teardown fails partway, the steps already
// taken are the ones that make the identity unusable, so the resting state after a
// failure is a disabled user rather than a half-detached one that still has keys.
//
// Step 8 detaches managed policies without deleting them. Dropping a policy's last
// attachment can leave it orphaned, but deleting it here would remove a resource the
// caller never announced — that decision belongs to the caller, on its own schedule.
func (r *IamRepository) DeleteUserWithDependencies(userName string) error {
	log.Info().Str("user", userName).Msg("[IamRepository.DeleteUserWithDependencies] starting teardown")

	// 1. console password
	_, err := r.iamClient().DeleteLoginProfile(r.ctx, &iam.DeleteLoginProfileInput{
		UserName: aws.String(userName),
	})
	if err := teardownStep("DeleteLoginProfile", userName, err); err != nil {
		return err
	}

	// 2. access keys
	keyIds, err := r.userAccessKeyIds(userName)
	if err := teardownStep("ListAccessKeys", userName, err); err != nil {
		return err
	}
	for _, keyId := range keyIds {
		_, err := r.iamClient().DeleteAccessKey(r.ctx, &iam.DeleteAccessKeyInput{
			UserName:    aws.String(userName),
			AccessKeyId: aws.String(keyId),
		})
		if err := teardownStep("DeleteAccessKey", userName, err); err != nil {
			return err
		}
	}

	// 3. MFA devices. Deactivation detaches the device from the user; a virtual device
	// is also an IAM resource of its own and leaks if only deactivated. Hardware
	// devices belong to the account, not the user, so they are left alone.
	serials, err := r.userMfaSerialNumbers(userName)
	if err := teardownStep("ListMFADevices", userName, err); err != nil {
		return err
	}
	for _, serial := range serials {
		_, err := r.iamClient().DeactivateMFADevice(r.ctx, &iam.DeactivateMFADeviceInput{
			UserName:     aws.String(userName),
			SerialNumber: aws.String(serial),
		})
		if err := teardownStep("DeactivateMFADevice", userName, err); err != nil {
			return err
		}

		if !isVirtualMfaDevice(serial) {
			continue
		}

		_, err = r.iamClient().DeleteVirtualMFADevice(r.ctx, &iam.DeleteVirtualMFADeviceInput{
			SerialNumber: aws.String(serial),
		})
		if err := teardownStep("DeleteVirtualMFADevice", userName, err); err != nil {
			return err
		}
	}

	// 4. signing certificates
	certIds, err := r.userSigningCertificateIds(userName)
	if err := teardownStep("ListSigningCertificates", userName, err); err != nil {
		return err
	}
	for _, certId := range certIds {
		_, err := r.iamClient().DeleteSigningCertificate(r.ctx, &iam.DeleteSigningCertificateInput{
			UserName:      aws.String(userName),
			CertificateId: aws.String(certId),
		})
		if err := teardownStep("DeleteSigningCertificate", userName, err); err != nil {
			return err
		}
	}

	// 5. SSH public keys (CodeCommit)
	sshKeyIds, err := r.userSshPublicKeyIds(userName)
	if err := teardownStep("ListSSHPublicKeys", userName, err); err != nil {
		return err
	}
	for _, sshKeyId := range sshKeyIds {
		_, err := r.iamClient().DeleteSSHPublicKey(r.ctx, &iam.DeleteSSHPublicKeyInput{
			UserName:       aws.String(userName),
			SSHPublicKeyId: aws.String(sshKeyId),
		})
		if err := teardownStep("DeleteSSHPublicKey", userName, err); err != nil {
			return err
		}
	}

	// 6. service-specific credentials
	credentialIds, err := r.userServiceSpecificCredentialIds(userName)
	if err := teardownStep("ListServiceSpecificCredentials", userName, err); err != nil {
		return err
	}
	for _, credentialId := range credentialIds {
		_, err := r.iamClient().DeleteServiceSpecificCredential(r.ctx, &iam.DeleteServiceSpecificCredentialInput{
			UserName:                    aws.String(userName),
			ServiceSpecificCredentialId: aws.String(credentialId),
		})
		if err := teardownStep("DeleteServiceSpecificCredential", userName, err); err != nil {
			return err
		}
	}

	// 7. inline policies
	inlineNames, err := r.userInlinePolicyNames(userName)
	if err := teardownStep("ListUserPolicies", userName, err); err != nil {
		return err
	}
	for _, policyName := range inlineNames {
		_, err := r.iamClient().DeleteUserPolicy(r.ctx, &iam.DeleteUserPolicyInput{
			UserName:   aws.String(userName),
			PolicyName: aws.String(policyName),
		})
		if err := teardownStep("DeleteUserPolicy", userName, err); err != nil {
			return err
		}
	}

	// 8. managed policies — detached, never deleted
	attachedArns, err := r.userAttachedPolicyArns(userName)
	if err := teardownStep("ListAttachedUserPolicies", userName, err); err != nil {
		return err
	}
	for _, policyArn := range attachedArns {
		_, err := r.iamClient().DetachUserPolicy(r.ctx, &iam.DetachUserPolicyInput{
			UserName:  aws.String(userName),
			PolicyArn: aws.String(policyArn),
		})
		if err := teardownStep("DetachUserPolicy", userName, err); err != nil {
			return err
		}
	}

	// 9. group memberships
	groupNames, err := r.userGroupNames(userName)
	if err := teardownStep("ListGroupsForUser", userName, err); err != nil {
		return err
	}
	for _, groupName := range groupNames {
		_, err := r.iamClient().RemoveUserFromGroup(r.ctx, &iam.RemoveUserFromGroupInput{
			UserName:  aws.String(userName),
			GroupName: aws.String(groupName),
		})
		if err := teardownStep("RemoveUserFromGroup", userName, err); err != nil {
			return err
		}
	}

	// 10. permissions boundary. Called unconditionally: AWS answers NoSuchEntity when
	// there is none, which is cheaper than reading the user to find out.
	_, err = r.iamClient().DeleteUserPermissionsBoundary(r.ctx, &iam.DeleteUserPermissionsBoundaryInput{
		UserName: aws.String(userName),
	})
	if err := teardownStep("DeleteUserPermissionsBoundary", userName, err); err != nil {
		return err
	}

	// 11. the user
	_, err = r.iamClient().DeleteUser(r.ctx, &iam.DeleteUserInput{UserName: aws.String(userName)})
	if err := teardownStep("DeleteUser", userName, err); err != nil {
		return err
	}

	log.Info().Str("user", userName).Msg("[IamRepository.DeleteUserWithDependencies] user removed")

	return nil
}

// DeleteRoleWithDependencies removes everything attached to a role, then the role.
//
//  1. ListRolePolicies            → DeleteRolePolicy
//  2. ListAttachedRolePolicies    → DetachRolePolicy
//  3. ListInstanceProfilesForRole → RemoveRoleFromInstanceProfile
//  4. DeleteRolePermissionsBoundary
//  5. DeleteRole
//
// Service-linked roles are not deletable this way — they need DeleteServiceLinkedRole,
// which is asynchronous — so this refuses one rather than failing at step 5 with a
// message about the wrong thing. Callers are expected to exclude them earlier; this is
// the backstop for a role that reaches here anyway.
func (r *IamRepository) DeleteRoleWithDependencies(roleName string) error {
	roleName = RoleNameFromArn(roleName)

	if isServiceLinkedRoleName(roleName) {
		return errors.Errorf(
			"role %q is service-linked and needs DeleteServiceLinkedRole, not DeleteRole",
			roleName,
		)
	}

	log.Info().Str("role", roleName).Msg("[IamRepository.DeleteRoleWithDependencies] starting teardown")

	// 1. inline policies
	inlineNames, err := r.roleInlinePolicyNames(roleName)
	if err := teardownStep("ListRolePolicies", roleName, err); err != nil {
		return err
	}
	for _, policyName := range inlineNames {
		_, err := r.iamClient().DeleteRolePolicy(r.ctx, &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(policyName),
		})
		if err := teardownStep("DeleteRolePolicy", roleName, err); err != nil {
			return err
		}
	}

	// 2. managed policies — detached, never deleted
	attachedArns, err := r.roleAttachedPolicyArns(roleName)
	if err := teardownStep("ListAttachedRolePolicies", roleName, err); err != nil {
		return err
	}
	for _, policyArn := range attachedArns {
		_, err := r.iamClient().DetachRolePolicy(r.ctx, &iam.DetachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: aws.String(policyArn),
		})
		if err := teardownStep("DetachRolePolicy", roleName, err); err != nil {
			return err
		}
	}

	// 3. instance profiles
	profileNames, err := r.roleInstanceProfileNames(roleName)
	if err := teardownStep("ListInstanceProfilesForRole", roleName, err); err != nil {
		return err
	}
	for _, profileName := range profileNames {
		_, err := r.iamClient().RemoveRoleFromInstanceProfile(r.ctx, &iam.RemoveRoleFromInstanceProfileInput{
			RoleName:            aws.String(roleName),
			InstanceProfileName: aws.String(profileName),
		})
		if err := teardownStep("RemoveRoleFromInstanceProfile", roleName, err); err != nil {
			return err
		}
	}

	// 4. permissions boundary
	_, err = r.iamClient().DeleteRolePermissionsBoundary(r.ctx, &iam.DeleteRolePermissionsBoundaryInput{
		RoleName: aws.String(roleName),
	})
	if err := teardownStep("DeleteRolePermissionsBoundary", roleName, err); err != nil {
		return err
	}

	// 5. the role
	_, err = r.iamClient().DeleteRole(r.ctx, &iam.DeleteRoleInput{RoleName: aws.String(roleName)})
	if err := teardownStep("DeleteRole", roleName, err); err != nil {
		return err
	}

	log.Info().Str("role", roleName).Msg("[IamRepository.DeleteRoleWithDependencies] role removed")

	return nil
}

// DeletePolicyWithVersions deletes a customer-managed policy and its non-default
// versions, refusing if anything is still attached.
//
//  1. DescribePolicyEntities → must be empty
//  2. ListPolicyVersions     → DeletePolicyVersion for each non-default version
//  3. DeletePolicy
//
// Step 1 is a live read and is the whole point of this method. Policy.AttachmentCount
// comes from a listing that may be a full read cycle old, and a policy attached in the
// meantime would be deleted out from under whoever attached it. DeletePolicy would
// itself fail with DeleteConflict, but by then the versions from step 2 are already
// gone — so the check has to come first, and it has to be fresh.
//
// The default version is not deletable on its own; DeletePolicy removes it.
func (r *IamRepository) DeletePolicyWithVersions(policyArn string) error {
	users, roles, groups, err := r.DescribePolicyEntities(policyArn)
	if err := teardownStep("ListEntitiesForPolicy", policyArn, err); err != nil {
		return err
	}

	if len(users)+len(roles)+len(groups) > 0 {
		return errors.Errorf(
			"policy %q is still attached to %d user(s), %d role(s), %d group(s)",
			policyArn, len(users), len(roles), len(groups),
		)
	}

	log.Info().Str("policy", policyArn).Msg("[IamRepository.DeletePolicyWithVersions] starting teardown")

	versionIds, err := r.policyNonDefaultVersionIds(policyArn)
	if err := teardownStep("ListPolicyVersions", policyArn, err); err != nil {
		return err
	}
	for _, versionId := range versionIds {
		_, err := r.iamClient().DeletePolicyVersion(r.ctx, &iam.DeletePolicyVersionInput{
			PolicyArn: aws.String(policyArn),
			VersionId: aws.String(versionId),
		})
		if err := teardownStep("DeletePolicyVersion", policyArn, err); err != nil {
			return err
		}
	}

	_, err = r.iamClient().DeletePolicy(r.ctx, &iam.DeletePolicyInput{PolicyArn: aws.String(policyArn)})
	if err := teardownStep("DeletePolicy", policyArn, err); err != nil {
		return err
	}

	log.Info().Str("policy", policyArn).Msg("[IamRepository.DeletePolicyWithVersions] policy removed")

	return nil
}

// DescribePolicyEntities returns the user, role and group names a policy is attached to.
//
// This is the authoritative answer to "is anyone still using this policy", and it is
// deliberately named Describe* rather than List*: the generated cached wrapper only
// wraps Get*/List* methods, and a cached answer here is worse than no answer.
// Policy.AttachmentCount from a listing is the cheap approximation; this is the one to
// act on.
func (r *IamRepository) DescribePolicyEntities(policyArn string) (users, roles, groups []string, err error) {
	query := &iam.ListEntitiesForPolicyInput{PolicyArn: aws.String(policyArn)}

	p := iam.NewListEntitiesForPolicyPaginator(r.iamClient(), query)
	for p.HasMorePages() {
		resp, err := p.NextPage(r.ctx)
		if err != nil {
			return nil, nil, nil, errors.New(err)
		}

		for _, entity := range resp.PolicyUsers {
			users = append(users, aws.ToString(entity.UserName))
		}
		for _, entity := range resp.PolicyRoles {
			roles = append(roles, aws.ToString(entity.RoleName))
		}
		for _, entity := range resp.PolicyGroups {
			groups = append(groups, aws.ToString(entity.GroupName))
		}
	}

	return users, roles, groups, nil
}

// isVirtualMfaDevice reports whether an MFA serial number identifies a virtual device.
//
// A virtual device's serial number is its ARN; a hardware device's is a manufacturer
// serial. Only the virtual kind is an IAM resource that has to be deleted after being
// deactivated.
func isVirtualMfaDevice(serialNumber string) bool {
	return strings.HasPrefix(serialNumber, "arn:")
}

// isServiceLinkedRoleName reports whether a role is service-linked, which AWS marks by
// putting it under the /aws-service-role/ path.
//
// The path is not part of the name GetRole and DeleteRole take, so a caller holding only
// a name cannot always tell. This catches the ARN form and the reserved name prefix; the
// authoritative check is Role.Path, which callers reading whole roles should prefer.
func isServiceLinkedRoleName(roleName string) bool {
	return strings.HasPrefix(roleName, "AWSServiceRoleFor") ||
		strings.Contains(roleName, "/aws-service-role/")
}
