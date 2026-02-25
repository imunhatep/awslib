package route53

import (
	"fmt"
	"maps"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type ResourceRecordList struct {
	Items []ResourceRecord
}

type ResourceRecord struct {
	service.AbstractResource
	types.ResourceRecordSet
	hostedZone HostedZone
	Tags       map[string]string
}

func NewResourceRecord(client AwsClient, hostedZone HostedZone, resourceRecord types.ResourceRecordSet) ResourceRecord {
	rrArn := helper.BuildArn(client.GetAccountID(), client.GetRegion(), "route53", fmt.Sprintf("hostedzone/%s/", hostedZone.GetId()), resourceRecord.Name)

	return ResourceRecord{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(resourceRecord.Name),
			ARN:       rrArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeRoute53ResourceRecord,
		},
		ResourceRecordSet: resourceRecord,
		hostedZone:        hostedZone,
		Tags:              hostedZone.GetTags(),
	}
}

func (e ResourceRecord) GetName() string {
	return aws.ToString(e.Name)
}

func (e ResourceRecord) GetHostedZone() HostedZone {
	return e.hostedZone
}

func (e ResourceRecord) GetTags() map[string]string {
	return maps.Clone(e.Tags)
}

func (e ResourceRecord) GetTagValue(tag string) string {
	if val, ok := e.Tags[tag]; ok {
		return val
	}

	return ""
}
