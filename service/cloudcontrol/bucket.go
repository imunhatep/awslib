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

type BucketList struct {
	Items []Bucket
}

type Bucket struct {
	service.AbstractResource
	cc.ResourceDescription
	attributes map[string]interface{}
	tags       map[string]string
}

func init() {
	gob.Register(Bucket{})
}

func NewBucket(
	client AwsClient,
	resource cc.ResourceDescription,
	attributes map[string]interface{},
	tags map[string]string,
) Bucket {
	s3bucket := Bucket{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(resource.Identifier),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "s3", "", resource.Identifier),
			CreatedAt: time.Unix(0, 0), // aws.ToTime(bucket.CreationDate),
			Type:      cfg.ResourceTypeBucket,
		},
		ResourceDescription: resource,
		attributes:          attributes,
		tags:                tags,
	}

	return s3bucket
}

func (e Bucket) GetName() string {
	return *e.ResourceDescription.Identifier
}

func (e Bucket) GetAttributes() map[string]interface{} {
	return e.attributes
}

func (e Bucket) GetTags() map[string]string {
	return e.tags
}

func (e Bucket) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
