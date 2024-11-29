package cloudcontrol

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"time"
)

type Volume struct {
	service.AbstractResource
	cc.ResourceDescription
	attributes map[string]interface{}
	tags       map[string]string
}

func NewVolume(client AwsClient, resource cc.ResourceDescription, attributes map[string]interface{}, tags map[string]string) Volume {
	rsrc := Volume{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(resource.Identifier),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "volume/", resource.Identifier),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeVolume,
		},
		ResourceDescription: resource,
		attributes:          attributes,
		tags:                tags,
	}

	return rsrc
}

func init() {
	gob.Register(Volume{})
}

func (e Volume) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Volume) GetAttributes() map[string]interface{} {
	return e.attributes
}

func (e Volume) GetTags() map[string]string {
	return e.tags
}

func (e Volume) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
