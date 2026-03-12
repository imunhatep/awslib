package route53

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	domtypes "github.com/aws/aws-sdk-go-v2/service/route53domains/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// DomainSummaryList holds a list of DomainSummary items returned by listing operations.
type DomainSummaryList struct {
	Items []DomainSummary
}

// DomainSummary is a lightweight domain entity built from the ListDomains API response.
type DomainSummary struct {
	service.AbstractResource
	domtypes.DomainSummary
}

func NewDomainSummary(client AwsClient, summary domtypes.DomainSummary) DomainSummary {
	domainArn := helper.BuildArn(client.GetAccountID(), client.GetRegion(), "route53domains", "domain/", summary.DomainName)

	expiry := time.Unix(0, 0)
	if summary.Expiry != nil {
		expiry = *summary.Expiry
	}

	return DomainSummary{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.DomainName),
			ARN:       domainArn,
			CreatedAt: expiry,
			Type:      ccfg.ResourceTypeRoute53DomainSummary,
		},
		DomainSummary: summary,
	}
}

func (e DomainSummary) GetName() string {
	return aws.ToString(e.DomainName)
}

func (e DomainSummary) GetTags() map[string]string {
	return map[string]string{}
}

func (e DomainSummary) GetTagValue(_ string) string {
	return ""
}

// DomainList holds a list of Domain items with full details.
type DomainList struct {
	Items []Domain
}

// Domain is a full domain entity built from the GetDomainDetail API response.
type Domain struct {
	service.AbstractResource
	route53domains.GetDomainDetailOutput
	Tags []domtypes.Tag
}

func NewDomain(client AwsClient, detail route53domains.GetDomainDetailOutput, tags []domtypes.Tag) Domain {
	domainArn := helper.BuildArn(client.GetAccountID(), client.GetRegion(), "route53domains", "domain/", detail.DomainName)

	createdAt := time.Unix(0, 0)
	if detail.CreationDate != nil {
		createdAt = *detail.CreationDate
	}

	return Domain{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(detail.DomainName),
			ARN:       domainArn,
			CreatedAt: createdAt,
			Type:      ccfg.ResourceTypeRoute53Domain,
		},
		GetDomainDetailOutput: detail,
		Tags:                  tags,
	}
}

func (e Domain) GetName() string {
	return aws.ToString(e.DomainName)
}

func (e Domain) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Domain) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
