package wafv2

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	awswaf "github.com/aws/aws-sdk-go-v2/service/wafv2"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/go-errors/errors"
	cfclient "github.com/imunhatep/awslib/provider/v3/clients/cloudfront"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// Resource kinds reported by FindWebACLAssociations for CloudFront, which the
// WAFv2 API does not answer for: a distribution names its web ACL, not the
// other way round, so the CloudFront API is asked instead.
const (
	AssociationCloudFrontDistribution       = "CLOUDFRONT_DISTRIBUTION"
	AssociationCloudFrontDistributionTenant = "CLOUDFRONT_DISTRIBUTION_TENANT"
)

// regionalAssociationTypes are the resource types a REGIONAL web ACL can
// protect. Each is a separate ListResourcesForWebACL call.
var regionalAssociationTypes = []waftypes.ResourceType{
	waftypes.ResourceTypeApplicationLoadBalancer,
	waftypes.ResourceTypeApiGateway,
	waftypes.ResourceTypeAppsync,
	waftypes.ResourceTypeCognitioUserPool,
	waftypes.ResourceTypeAppRunnerService,
	waftypes.ResourceTypeVerifiedAccessInstance,
	waftypes.ResourceTypeAgentcoreGateway,
}

// WebACLAssociation is one resource a web ACL protects.
type WebACLAssociation struct {
	// ResourceType is the WAFv2 resource type (APPLICATION_LOAD_BALANCER, …) or
	// one of the CloudFront kinds above.
	ResourceType string
	ResourceArn  string
	// Name is set where the lookup returns one: a distribution's first alias or
	// domain, a tenant's name.
	Name string
}

// WebACLAssociations is the answer to "what does this web ACL protect".
//
// Failures holds the resource types that could not be checked, with the
// reason. It is the difference between "protects nothing of this kind" and "we
// could not look", which an empty Resources list alone cannot express — a type
// a region does not offer, or a missing permission for one service, must not
// read as "unprotected".
type WebACLAssociations struct {
	Resources []WebACLAssociation
	Checked   []string
	Failures  map[string]string
}

// FindWebACLAssociations lists the resources a web ACL is attached to.
//
// REGIONAL: one ListResourcesForWebACL per protectable resource type. CLOUDFRONT:
// the CloudFront API's ListDistributionsByWebACLId and
// ListDistributionTenantsByCustomization, plus ListResourcesForWebACL for
// AMPLIFY. A type that fails is recorded in Failures; only if every type fails is
// the call an error.
//
// Deliberately not a Get*/List* method: associations change on every deploy, and
// the generated cached wrapper would answer from a stale cache.
func (r *Wafv2Repository) FindWebACLAssociations(acl WebACL) (*WebACLAssociations, error) {
	webACLArn := aws.ToString(acl.WebACL.ARN)
	if webACLArn == "" {
		return nil, errors.New("web ACL has no ARN")
	}

	out := &WebACLAssociations{Failures: map[string]string{}}

	record := func(kind string, found []WebACLAssociation, err error) {
		out.Checked = append(out.Checked, kind)
		if err != nil {
			out.Failures[kind] = err.Error()

			return
		}

		out.Resources = append(out.Resources, found...)
	}

	if acl.Scope == waftypes.ScopeCloudfront {
		found, err := r.distributionsForWebACL(webACLArn)
		record(AssociationCloudFrontDistribution, found, err)

		found, err = r.tenantsForWebACL(webACLArn)
		record(AssociationCloudFrontDistributionTenant, found, err)

		found, err = r.resourcesForWebACL(acl.Scope, webACLArn, waftypes.ResourceTypeAmplify)
		record(string(waftypes.ResourceTypeAmplify), found, err)
	} else {
		for _, resourceType := range regionalAssociationTypes {
			found, err := r.resourcesForWebACL(acl.Scope, webACLArn, resourceType)
			record(string(resourceType), found, err)
		}
	}

	if len(out.Failures) == len(out.Checked) {
		return nil, errors.Errorf("no resource type could be checked for web ACL %s: %v", acl.GetName(), out.Failures)
	}

	sort.SliceStable(out.Resources, func(i, j int) bool {
		if out.Resources[i].ResourceType != out.Resources[j].ResourceType {
			return out.Resources[i].ResourceType < out.Resources[j].ResourceType
		}

		return out.Resources[i].ResourceArn < out.Resources[j].ResourceArn
	})

	return out, nil
}

func (r *Wafv2Repository) resourcesForWebACL(scope waftypes.Scope, webACLArn string, resourceType waftypes.ResourceType) ([]WebACLAssociation, error) {
	r.countRequest("ListResourcesForWebACL", ccfg.ResourceTypeWafv2WebACLAssociation)

	output, err := r.wafClient().ListResourcesForWebACL(r.ctx, &awswaf.ListResourcesForWebACLInput{
		WebACLArn:    aws.String(webACLArn),
		ResourceType: resourceType,
	}, r.opts(scope, withRetries)...)
	if err != nil {
		r.countError("ListResourcesForWebACL", ccfg.ResourceTypeWafv2WebACLAssociation)

		return nil, classifyErr(err)
	}

	found := make([]WebACLAssociation, 0, len(output.ResourceArns))
	for _, resourceArn := range output.ResourceArns {
		found = append(found, WebACLAssociation{ResourceType: string(resourceType), ResourceArn: resourceArn})
	}

	return found, nil
}

func (r *Wafv2Repository) distributionsForWebACL(webACLArn string) ([]WebACLAssociation, error) {
	var (
		found  []WebACLAssociation
		marker *string
	)

	for {
		r.countRequest("ListDistributionsByWebACLId", ccfg.ResourceTypeWafv2WebACLAssociation)

		output, err := cfclient.GetClient(r.client).ListDistributionsByWebACLId(r.ctx, &awscf.ListDistributionsByWebACLIdInput{
			WebACLId: aws.String(webACLArn),
			Marker:   marker,
		}, func(o *awscf.Options) { o.RetryMaxAttempts = detailFetchMaxAttempts })
		if err != nil {
			r.countError("ListDistributionsByWebACLId", ccfg.ResourceTypeWafv2WebACLAssociation)

			return nil, errors.New(err)
		}

		list := output.DistributionList
		if list == nil {
			return found, nil
		}

		for _, d := range list.Items {
			name := aws.ToString(d.DomainName)
			if d.Aliases != nil && len(d.Aliases.Items) > 0 {
				name = d.Aliases.Items[0]
			}

			found = append(found, WebACLAssociation{
				ResourceType: AssociationCloudFrontDistribution,
				ResourceArn:  aws.ToString(d.ARN),
				Name:         name,
			})
		}

		if !aws.ToBool(list.IsTruncated) || aws.ToString(list.NextMarker) == "" {
			return found, nil
		}

		marker = list.NextMarker
	}
}

func (r *Wafv2Repository) tenantsForWebACL(webACLArn string) ([]WebACLAssociation, error) {
	var found []WebACLAssociation

	p := awscf.NewListDistributionTenantsByCustomizationPaginator(cfclient.GetClient(r.client), &awscf.ListDistributionTenantsByCustomizationInput{
		WebACLArn: aws.String(webACLArn),
	})

	for p.HasMorePages() {
		r.countRequest("ListDistributionTenantsByCustomization", ccfg.ResourceTypeWafv2WebACLAssociation)

		output, err := p.NextPage(r.ctx, func(o *awscf.Options) { o.RetryMaxAttempts = detailFetchMaxAttempts })
		if err != nil {
			r.countError("ListDistributionTenantsByCustomization", ccfg.ResourceTypeWafv2WebACLAssociation)

			return nil, errors.New(fmt.Errorf("listing distribution tenants: %w", err))
		}

		for _, tenant := range output.DistributionTenantList {
			found = append(found, WebACLAssociation{
				ResourceType: AssociationCloudFrontDistributionTenant,
				ResourceArn:  aws.ToString(tenant.Arn),
				Name:         aws.ToString(tenant.Name),
			})
		}
	}

	return found, nil
}
