package efs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/imunhatep/awslib/service"
)

type FileSystemList struct {
	Items []FileSystem
}

type FileSystem struct {
	service.AbstractResource
	types.FileSystemDescription
	Tags []types.Tag
}

func NewFileSystem(client AwsClient, fileSystem types.FileSystemDescription) FileSystem {
	efsArn, _ := arn.Parse(aws.ToString(fileSystem.FileSystemArn))

	return FileSystem{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(fileSystem.FileSystemId),
			ARN:       &efsArn,
			CreatedAt: aws.ToTime(fileSystem.CreationTime),
			Type:      cfg.ResourceTypeEFSFileSystem,
		},
		FileSystemDescription: fileSystem,
	}
}

func (e FileSystem) GetName() string {
	return aws.ToString(e.FileSystemDescription.Name)
}

func (e FileSystem) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.FileSystemDescription.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e FileSystem) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
