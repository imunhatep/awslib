package wafv2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// RuleGroup is a customer-owned rule group as returned by GetRuleGroup — its
// rules, capacity, labels and visibility configuration — with its tags. Managed
// rule groups (AWS and Marketplace) are not listed: they are not objects in the
// account, and a web ACL refers to them by vendor and name.
type RuleGroup struct {
	service.AbstractResource
	waftypes.RuleGroup
	Scope waftypes.Scope
	Tags  []waftypes.Tag
}

func NewRuleGroup(client AwsClient, scope waftypes.Scope, group waftypes.RuleGroup, tags []waftypes.Tag) RuleGroup {
	return RuleGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    regionForScope(client, scope),
			ID:        aws.ToString(group.Id),
			ARN:       parseArn(group.ARN),
			CreatedAt: time.Unix(0, 0),
			Type:      ccfg.ResourceTypeWafv2RuleGroup,
		},
		RuleGroup: group,
		Scope:     scope,
		Tags:      tags,
	}
}

func (e RuleGroup) GetName() string {
	return aws.ToString(e.Name)
}

func (e RuleGroup) GetTags() map[string]string {
	return tagMap(e.Tags)
}

func (e RuleGroup) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

func (e RuleGroup) GetScope() waftypes.Scope {
	return e.Scope
}

// GetMetricName returns the rule group's own CloudWatch metric name.
func (e RuleGroup) GetMetricName() string {
	if e.VisibilityConfig == nil {
		return ""
	}

	return aws.ToString(e.VisibilityConfig.MetricName)
}

type ruleGroupWire struct {
	Resource  service.AbstractResource
	RuleGroup waftypes.RuleGroup
	Scope     waftypes.Scope
	Tags      []waftypes.Tag
}

// GobEncode serializes through JSON; see gob.go for why.
func (e RuleGroup) GobEncode() ([]byte, error) {
	return gobEncodeJSON(ruleGroupWire{
		Resource:  e.AbstractResource,
		RuleGroup: e.RuleGroup,
		Scope:     e.Scope,
		Tags:      e.Tags,
	})
}

func (e *RuleGroup) GobDecode(data []byte) error {
	var wire ruleGroupWire
	if err := gobDecodeJSON(data, &wire); err != nil {
		return err
	}

	*e = RuleGroup{
		AbstractResource: wire.Resource,
		RuleGroup:        wire.RuleGroup,
		Scope:            wire.Scope,
		Tags:             wire.Tags,
	}

	return nil
}
