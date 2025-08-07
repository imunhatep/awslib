package iam

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/imunhatep/awslib/service"
)

type RoleArn string

func (r RoleArn) String() string { return string(r) }

type RoleList struct {
	Items []Role
}

type Role struct {
	service.AbstractResource
	types.Role
}

func NewRole(client AwsClient, role types.Role) Role {
	usrArn, _ := arn.Parse(aws.ToString(role.Arn))

	return Role{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(role.RoleId),
			ARN:       &usrArn,
			CreatedAt: aws.ToTime(role.CreateDate),
			Type:      cfg.ResourceTypeRole,
		},
		Role: role,
	}
}

func (e Role) GetName() string {
	return aws.ToString(e.RoleName)
}

func (e Role) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Role) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
