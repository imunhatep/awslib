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

type Instance struct {
	service.AbstractResource
	cc.ResourceDescription
	attributes map[string]interface{}
	tags       map[string]string
}

func NewInstance(
	client AwsClient,
	resource cc.ResourceDescription,
	attributes map[string]interface{},
	tags map[string]string,
) Instance {
	rsrc := Instance{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(resource.Identifier),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "instance/", resource.Identifier),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeInstance,
		},
		ResourceDescription: resource,
		attributes:          attributes,
		tags:                tags,
	}

	return rsrc
}

func init() {
	gob.Register(Instance{})
}

func (e Instance) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Instance) GetAttributes() map[string]interface{} {
	return e.attributes
}

func (e Instance) GetTags() map[string]string {
	return e.tags
}

func (e Instance) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
