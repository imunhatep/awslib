package cloudfront

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/service"
)

// DistributionList holds a list of Distribution items.
type DistributionList struct {
	Items []Distribution
}

// Distribution is a classic CloudFront distribution as returned by ListDistributions. The
// summary already carries the parts that are asked about — aliases, origins, viewer
// certificate, WAF ACL — so no per-distribution GetDistribution is needed.
//
// Tags are not part of the list response; ListDistributionsAll reads them per distribution
// and stores them here.
type Distribution struct {
	service.AbstractResource
	cftypes.DistributionSummary
	Tags []cftypes.Tag
}

func NewDistribution(client AwsClient, summary cftypes.DistributionSummary, tags []cftypes.Tag) Distribution {
	return Distribution{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Id),
			ARN:       parseArn(summary.ARN),
			// ListDistributions reports only LastModifiedTime; a modification date passed off as
			// a creation date would be wrong, so this follows the other entities without one.
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeDistribution,
		},
		DistributionSummary: summary,
		Tags:                tags,
	}
}

// GetName returns the first alias, which is how a distribution is known, and falls back to
// the CloudFront domain name.
func (e Distribution) GetName() string {
	if aliases := e.GetAliases(); len(aliases) > 0 {
		return aliases[0]
	}

	return aws.ToString(e.DomainName)
}

func (e Distribution) GetTags() map[string]string {
	out := make(map[string]string, len(e.Tags))

	for _, tag := range e.Tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}

func (e Distribution) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

// GetDomainName returns the distribution's *.cloudfront.net domain.
func (e Distribution) GetDomainName() string {
	return aws.ToString(e.DomainName)
}

func (e Distribution) GetStatus() string {
	return aws.ToString(e.Status)
}

func (e Distribution) IsEnabled() bool {
	return aws.ToBool(e.Enabled)
}

// GetAliases returns the alternate domain names (CNAMEs) served by the distribution.
func (e Distribution) GetAliases() []string {
	if e.Aliases == nil {
		return nil
	}

	return append([]string(nil), e.Aliases.Items...)
}

// GetOrigins returns the distribution's origins.
func (e Distribution) GetOrigins() []cftypes.Origin {
	if e.Origins == nil {
		return nil
	}

	return append([]cftypes.Origin(nil), e.Origins.Items...)
}

// GetOriginDomainNames returns the domain name of every origin, in configuration order.
func (e Distribution) GetOriginDomainNames() []string {
	var out []string

	for _, origin := range e.GetOrigins() {
		out = append(out, aws.ToString(origin.DomainName))
	}

	return out
}

// GetWebACLId returns the WAF web ACL attached to the distribution, or an empty string.
func (e Distribution) GetWebACLId() string {
	return aws.ToString(e.WebACLId)
}
