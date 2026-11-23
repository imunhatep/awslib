package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

type Image struct {
	service.AbstractResource
	types.Image
}

func NewImage(client AwsClient, image types.Image) Image {
	return Image{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(image.ImageId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "image/", image.ImageId),
			CreatedAt: parseImageTime(image.CreationDate),
			Type:      ccfg.ResourceTypeEc2Image,
		},
		Image: image,
	}
}

// parseImageTime parses the RFC 3339 timestamps DescribeImages returns as strings, keeping
// the epoch convention for a missing or unparseable value.
func parseImageTime(value *string) time.Time {
	parsed, err := time.Parse(time.RFC3339, aws.ToString(value))
	if err != nil {
		return time.Unix(0, 0)
	}

	return parsed
}

// GetName prefers the Name tag, then the AMI name, which every image has.
func (e Image) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	if name := aws.ToString(e.Image.Name); name != "" {
		return name
	}

	return "-"
}

func (e Image) GetImageName() string {
	return aws.ToString(e.Image.Name)
}

func (e Image) GetDescription() string {
	return aws.ToString(e.Image.Description)
}

func (e Image) GetState() string {
	return string(e.Image.State)
}

func (e Image) GetOwnerId() string {
	return aws.ToString(e.Image.OwnerId)
}

// IsPublic reports whether the image has public launch permissions.
func (e Image) IsPublic() bool {
	return aws.ToBool(e.Image.Public)
}

func (e Image) GetArchitecture() string {
	return string(e.Image.Architecture)
}

func (e Image) GetPlatformDetails() string {
	return aws.ToString(e.Image.PlatformDetails)
}

func (e Image) GetRootDeviceType() string {
	return string(e.Image.RootDeviceType)
}

// GetLastLaunchedTime is when an instance was last launched from the image, the epoch when
// it never was or the API did not report it. AWS records it with up to 24 hours delay and
// only since 2017.
func (e Image) GetLastLaunchedTime() time.Time {
	return parseImageTime(e.Image.LastLaunchedTime)
}

// GetDeprecationTime is when the image is or was deprecated, the epoch when no deprecation
// is scheduled.
func (e Image) GetDeprecationTime() time.Time {
	return parseImageTime(e.Image.DeprecationTime)
}

// GetSnapshotIds returns the EBS snapshots behind the image's block devices. They are what
// an AMI costs, and they cannot be deleted while the image is registered.
func (e Image) GetSnapshotIds() []string {
	snapshotIds := []string{}
	for _, mapping := range e.Image.BlockDeviceMappings {
		if mapping.Ebs != nil && mapping.Ebs.SnapshotId != nil {
			snapshotIds = append(snapshotIds, aws.ToString(mapping.Ebs.SnapshotId))
		}
	}

	return snapshotIds
}

// GetSnapshotSizeGiB sums the volume sizes of the image's EBS block devices, the provisioned
// size its snapshots were taken from (snapshot storage is billed on changed blocks, so this
// is an upper bound).
func (e Image) GetSnapshotSizeGiB() int32 {
	var size int32
	for _, mapping := range e.Image.BlockDeviceMappings {
		if mapping.Ebs != nil {
			size += aws.ToInt32(mapping.Ebs.VolumeSize)
		}
	}

	return size
}

func (e Image) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Image.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Image) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
