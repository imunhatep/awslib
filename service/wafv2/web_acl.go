package wafv2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// WebACL is a complete web ACL as returned by GetWebACL — rules, default action,
// visibility (CloudWatch metric) configuration and Firewall Manager rule groups —
// with its tags and logging configuration. ListWebACLs returns only names and
// IDs, so a listing reads every ACL in full: the rules are what is asked about,
// and their metric names are what CloudWatch request counts are keyed on.
type WebACL struct {
	service.AbstractResource
	waftypes.WebACL
	Scope waftypes.Scope
	Tags  []waftypes.Tag
	// Logging is the ACL's logging configuration; nil means logging is off, but
	// only when LoggingKnown is true.
	Logging *waftypes.LoggingConfiguration
	// LoggingKnown is false when the logging configuration could not be read, so
	// that a failed read is not reported as "logging disabled".
	LoggingKnown bool
}

func NewWebACL(client AwsClient, scope waftypes.Scope, acl waftypes.WebACL, tags []waftypes.Tag) WebACL {
	return WebACL{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    regionForScope(client, scope),
			ID:        aws.ToString(acl.Id),
			ARN:       parseArn(acl.ARN),
			// WAFv2 reports no creation time for any object.
			CreatedAt: time.Unix(0, 0),
			Type:      ccfg.ResourceTypeWafv2WebACL,
		},
		WebACL: acl,
		Scope:  scope,
		Tags:   tags,
	}
}

func (e WebACL) GetName() string {
	return aws.ToString(e.Name)
}

func (e WebACL) GetTags() map[string]string {
	return tagMap(e.Tags)
}

func (e WebACL) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

// GetScope returns REGIONAL or CLOUDFRONT.
func (e WebACL) GetScope() waftypes.Scope {
	return e.Scope
}

// GetMetricName returns the ACL's CloudWatch metric name — the value of the
// WebACL dimension on every request-count metric WAF publishes for it.
func (e WebACL) GetMetricName() string {
	if e.VisibilityConfig == nil {
		return ""
	}

	return aws.ToString(e.VisibilityConfig.MetricName)
}

// GetDefaultAction returns "allow" or "block".
func (e WebACL) GetDefaultAction() string {
	if e.DefaultAction == nil {
		return ""
	}

	switch {
	case e.DefaultAction.Block != nil:
		return "block"
	case e.DefaultAction.Allow != nil:
		return "allow"
	}

	return ""
}

// GetLogDestinations returns where the ACL logs to, empty when logging is off or
// unknown (see LoggingKnown).
func (e WebACL) GetLogDestinations() []string {
	if e.Logging == nil {
		return nil
	}

	return append([]string(nil), e.Logging.LogDestinationConfigs...)
}

type webACLWire struct {
	Resource     service.AbstractResource
	WebACL       waftypes.WebACL
	Scope        waftypes.Scope
	Tags         []waftypes.Tag
	Logging      *waftypes.LoggingConfiguration
	LoggingKnown bool
}

// GobEncode serializes through JSON; see gob.go for why.
func (e WebACL) GobEncode() ([]byte, error) {
	return gobEncodeJSON(webACLWire{
		Resource:     e.AbstractResource,
		WebACL:       e.WebACL,
		Scope:        e.Scope,
		Tags:         e.Tags,
		Logging:      e.Logging,
		LoggingKnown: e.LoggingKnown,
	})
}

func (e *WebACL) GobDecode(data []byte) error {
	var wire webACLWire
	if err := gobDecodeJSON(data, &wire); err != nil {
		return err
	}

	*e = WebACL{
		AbstractResource: wire.Resource,
		WebACL:           wire.WebACL,
		Scope:            wire.Scope,
		Tags:             wire.Tags,
		Logging:          wire.Logging,
		LoggingKnown:     wire.LoggingKnown,
	}

	return nil
}

// regionForScope is the region an object of the given scope is attributed to:
// us-east-1 for CLOUDFRONT, which is also the region in its ARN.
func regionForScope(client AwsClient, scope waftypes.Scope) ptypes.AwsRegion {
	if scope == waftypes.ScopeCloudfront {
		return CloudFrontRegion
	}

	return client.GetRegion()
}
