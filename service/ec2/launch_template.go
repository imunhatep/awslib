package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type LaunchTemplate struct {
	service.AbstractResource
	types.LaunchTemplate
}

func NewLaunchTemplate(client AwsClient, launchTemplate types.LaunchTemplate) LaunchTemplate {
	return LaunchTemplate{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(launchTemplate.LaunchTemplateId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "launch-template/", launchTemplate.LaunchTemplateId),
			CreatedAt: aws.ToTime(launchTemplate.CreateTime),
			Type:      cfg.ResourceTypeLaunchTemplate,
		},
		LaunchTemplate: launchTemplate,
	}
}

// GetName prefers the Name tag, then the launch template name, which every template has.
func (e LaunchTemplate) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	if name := aws.ToString(e.LaunchTemplate.LaunchTemplateName); name != "" {
		return name
	}

	return "-"
}

func (e LaunchTemplate) GetLaunchTemplateName() string {
	return aws.ToString(e.LaunchTemplate.LaunchTemplateName)
}

func (e LaunchTemplate) GetDefaultVersionNumber() int64 {
	return aws.ToInt64(e.LaunchTemplate.DefaultVersionNumber)
}

func (e LaunchTemplate) GetLatestVersionNumber() int64 {
	return aws.ToInt64(e.LaunchTemplate.LatestVersionNumber)
}

// GetCreatedBy is the ARN of the principal that created the template.
func (e LaunchTemplate) GetCreatedBy() string {
	return aws.ToString(e.LaunchTemplate.CreatedBy)
}

// IsManaged reports whether the template is managed by an AWS service (an EKS managed node
// group, for example) and so should not be edited directly.
func (e LaunchTemplate) IsManaged() bool {
	return e.LaunchTemplate.Operator != nil && aws.ToBool(e.LaunchTemplate.Operator.Managed)
}

func (e LaunchTemplate) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.LaunchTemplate.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e LaunchTemplate) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
