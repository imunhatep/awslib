package wafv2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// IPSet is an IP set as returned by GetIPSet, addresses included, with its tags.
type IPSet struct {
	service.AbstractResource
	waftypes.IPSet
	Scope waftypes.Scope
	Tags  []waftypes.Tag
}

func NewIPSet(client AwsClient, scope waftypes.Scope, set waftypes.IPSet, tags []waftypes.Tag) IPSet {
	return IPSet{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    regionForScope(client, scope),
			ID:        aws.ToString(set.Id),
			ARN:       parseArn(set.ARN),
			CreatedAt: time.Unix(0, 0),
			Type:      ccfg.ResourceTypeWafv2IPSet,
		},
		IPSet: set,
		Scope: scope,
		Tags:  tags,
	}
}

func (e IPSet) GetName() string {
	return aws.ToString(e.Name)
}

func (e IPSet) GetTags() map[string]string {
	return tagMap(e.Tags)
}

func (e IPSet) GetScope() waftypes.Scope {
	return e.Scope
}

// GetAddresses returns a copy of the set's CIDR blocks.
func (e IPSet) GetAddresses() []string {
	return append([]string(nil), e.Addresses...)
}

// RegexPatternSet is a regex pattern set as returned by GetRegexPatternSet,
// patterns included, with its tags.
type RegexPatternSet struct {
	service.AbstractResource
	waftypes.RegexPatternSet
	Scope waftypes.Scope
	Tags  []waftypes.Tag
}

func NewRegexPatternSet(client AwsClient, scope waftypes.Scope, set waftypes.RegexPatternSet, tags []waftypes.Tag) RegexPatternSet {
	return RegexPatternSet{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    regionForScope(client, scope),
			ID:        aws.ToString(set.Id),
			ARN:       parseArn(set.ARN),
			CreatedAt: time.Unix(0, 0),
			Type:      ccfg.ResourceTypeWafv2RegexPatternSet,
		},
		RegexPatternSet: set,
		Scope:           scope,
		Tags:            tags,
	}
}

func (e RegexPatternSet) GetName() string {
	return aws.ToString(e.Name)
}

func (e RegexPatternSet) GetTags() map[string]string {
	return tagMap(e.Tags)
}

func (e RegexPatternSet) GetScope() waftypes.Scope {
	return e.Scope
}

// GetPatterns returns the set's regular expressions.
func (e RegexPatternSet) GetPatterns() []string {
	out := make([]string, 0, len(e.RegularExpressionList))
	for _, regex := range e.RegularExpressionList {
		out = append(out, aws.ToString(regex.RegexString))
	}

	return out
}
