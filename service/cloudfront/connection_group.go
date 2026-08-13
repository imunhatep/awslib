package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ConnectionGroupList holds a list of ConnectionGroup items.
type ConnectionGroupList struct {
	Items []ConnectionGroup
}

// ConnectionGroup is the routing target shared by a set of tenants. Its
// RoutingEndpoint is what a hostname's DNS record must point at, which makes
// it a required read before any DNS is written.
type ConnectionGroup struct {
	service.AbstractResource
	cftypes.ConnectionGroup
	ETag string
}

// connectionGroupFromSummary widens the shape returned by ListConnectionGroups
// into the full one. The summary is a strict subset — it carries every field
// callers use, including RoutingEndpoint, ETag and IsDefault — and differs only
// in that Tags are absent. Unlike distribution tenants, where the list response
// also drops functional configuration (Parameters), nothing here changes how the
// resource behaves, so one entity type serves both paths.
func connectionGroupFromSummary(summary cftypes.ConnectionGroupSummary) cftypes.ConnectionGroup {
	return cftypes.ConnectionGroup{
		AnycastIpListId:  summary.AnycastIpListId,
		Arn:              summary.Arn,
		CreatedTime:      summary.CreatedTime,
		Enabled:          summary.Enabled,
		Id:               summary.Id,
		IsDefault:        summary.IsDefault,
		LastModifiedTime: summary.LastModifiedTime,
		Name:             summary.Name,
		RoutingEndpoint:  summary.RoutingEndpoint,
		Status:           summary.Status,
	}
}

func NewConnectionGroup(client AwsClient, group cftypes.ConnectionGroup, etag string) ConnectionGroup {
	return ConnectionGroup{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(group.Id),
			ARN:       parseArn(group.Arn),
			CreatedAt: aws.ToTime(group.CreatedTime),
			Type:      ccfg.ResourceTypeCloudFrontConnectionGroup,
		},
		ConnectionGroup: group,
		ETag:            etag,
	}
}

func (e ConnectionGroup) GetName() string {
	return aws.ToString(e.Name)
}

func (e ConnectionGroup) GetTags() map[string]string {
	return tagsToMap(e.ConnectionGroup.Tags)
}

func (e ConnectionGroup) GetTagValue(tag string) string {
	return e.GetTags()[tag]
}

// GetRoutingEndpoint returns the CloudFront domain that tenant hostnames must
// CNAME to.
func (e ConnectionGroup) GetRoutingEndpoint() string {
	return aws.ToString(e.RoutingEndpoint)
}

func (e ConnectionGroup) IsEnabled() bool {
	return aws.ToBool(e.Enabled)
}

// IsDefaultGroup reports whether this is the account's auto-created group.
// The default group must not be deleted.
func (e ConnectionGroup) IsDefaultGroup() bool {
	return aws.ToBool(e.IsDefault)
}
