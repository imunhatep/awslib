package iam

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/imunhatep/awslib/service"
)

type UserList struct {
	Items []User
}

type User struct {
	service.AbstractResource
	types.User
}

func NewUser(client AwsClient, user types.User) User {
	usrArn, _ := arn.Parse(aws.ToString(user.Arn))

	return User{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(user.UserId),
			ARN:       &usrArn,
			CreatedAt: aws.ToTime(user.CreateDate),
			Type:      cfg.ResourceTypeUser,
		},
		User: user,
	}
}

func (e User) GetName() string {
	return aws.ToString(e.UserName)
}

func (e User) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e User) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
