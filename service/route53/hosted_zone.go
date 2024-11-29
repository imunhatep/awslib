package route53

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"time"
)

func init() {
	gob.Register(HostedZone{})
}

type HostedZoneList struct {
	Items []HostedZone
}

type HostedZone struct {
	service.AbstractResource
	types.HostedZone
	Tags []types.Tag
}

func NewHostedZone(client AwsClient, hostedZone types.HostedZone, tags []types.Tag) HostedZone {
	hzArn := helper.BuildArn(client.GetAccountID(), client.GetRegion(), "route53", "hostedzone/", hostedZone.Id)

	return HostedZone{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(hostedZone.Id),
			ARN:       hzArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeRoute53HostedZone,
		},
		HostedZone: hostedZone,
		Tags:       tags,
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
