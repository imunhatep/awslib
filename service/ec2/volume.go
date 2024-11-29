package ec2

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

func init() {
	gob.Register(Volume{})
}

type Volume struct {
	service.AbstractResource
	types.Volume
}

func NewVolume(client AwsClient, volume types.Volume) Volume {
	ebs := Volume{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(volume.VolumeId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "volume/", volume.VolumeId),
			CreatedAt: aws.ToTime(volume.CreateTime),
			Type:      cfg.ResourceTypeVolume,
		},
		Volume: volume,
	}

	return ebs
}

func (e Volume) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e Volume) GetState() types.VolumeState {
	return e.Volume.State
}

func (e Volume) GetSize() int32 {
	return aws.ToInt32(e.Volume.Size)
}

func (e Volume) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Volume) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
