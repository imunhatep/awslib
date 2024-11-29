package service

import (
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"time"
)

type ResourceInterface interface {
	GetAccountID() ptypes.AwsAccountID
	GetRegion() ptypes.AwsRegion
	GetCreatedAt() time.Time
	GetArn() string
	GetId() string
	GetIdOrArn() string
	GetType() cfg.ResourceType
	GetTags() map[string]string
}

type AbstractResource struct {
	AccountID ptypes.AwsAccountID
	Region    ptypes.AwsRegion
	ID        string
	ARN       *arn.ARN
	CreatedAt time.Time
	Type      cfg.ResourceType
}

func (e AbstractResource) GetType() cfg.ResourceType {
	return e.Type
}

func (e AbstractResource) GetAccountID() ptypes.AwsAccountID {
	return e.AccountID
}

func (e AbstractResource) GetRegion() ptypes.AwsRegion {
	return e.Region
}

func (e AbstractResource) GetCreatedAt() time.Time {
	return e.CreatedAt
}

func (e AbstractResource) GetArn() string {
	if e.ARN == nil {
		return ""
	}

	return e.ARN.String()
}

func (e AbstractResource) GetId() string {
	return e.ID
}

func (e AbstractResource) GetIdOrArn() string {
	if id := e.GetId(); id != "" {
		return id
	}

	return e.GetArn()
}
