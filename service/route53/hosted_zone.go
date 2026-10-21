package route53

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type HostedZoneList struct {
	Items []HostedZone
}

type HostedZone struct {
	service.AbstractResource
	types.HostedZone
	DelegationSet *types.DelegationSet
	VPCs          []types.VPC
	Tags          []types.Tag
}

// NormalizeHostedZoneID returns the bare hosted zone id ("Z123") for either
// spelling AWS uses: the "/hostedzone/Z123" path form that GetHostedZone,
// ListHostedZones and CreateHostedZone return, or the bare id.
func NormalizeHostedZoneID(id string) string {
	return id[strings.LastIndex(id, "/")+1:]
}

// NewHostedZone wraps hostedZone. The entity's identity — GetId() and the ARN —
// is the bare zone id; the embedded types.HostedZone keeps Id exactly as AWS
// returned it, in the "/hostedzone/Z123" form.
func NewHostedZone(client AwsClient, hostedZone types.HostedZone, delegationSet *types.DelegationSet, vpcs []types.VPC, tags []types.Tag) HostedZone {
	id := NormalizeHostedZoneID(aws.ToString(hostedZone.Id))
	hzArn := helper.BuildArn(client.GetAccountID(), client.GetRegion(), "route53", "hostedzone/", &id)

	return HostedZone{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        id,
			ARN:       hzArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeRoute53HostedZone,
		},
		HostedZone:    hostedZone,
		DelegationSet: delegationSet,
		VPCs:          vpcs,
		Tags:          tags,
	}
}

func (e HostedZone) GetName() string {
	return aws.ToString(e.Name)
}

func (e HostedZone) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e HostedZone) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
