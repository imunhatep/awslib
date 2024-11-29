package s3

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type BucketList struct {
	Items []Bucket
}

type Bucket struct {
	service.AbstractResource
	types.Bucket
	Tags []types.Tag
}

func init() {
	gob.Register(Bucket{})
}

func NewBucket(client AwsClient, bucket types.Bucket, tags []types.Tag) Bucket {
	s3bucket := Bucket{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(bucket.Name),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "s3", "", bucket.Name),
			CreatedAt: aws.ToTime(bucket.CreationDate),
			Type:      cfg.ResourceTypeBucket,
		},
		Bucket: bucket,
		Tags:   tags,
	}

	return s3bucket
}

func (e Bucket) GetName() string {
	return aws.ToString(e.Name)
}

func (e Bucket) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Bucket) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
