package emrserverless

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type ApplicationList struct {
	Items []Application
}

type Application struct {
	service.AbstractResource
	*types.Application
}

func init() {
	gob.Register(Application{})
}

func NewApplication(client AwsClient, application *types.Application) Application {
	appArn, _ := arn.Parse(aws.ToString(application.Arn))

	return Application{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(application.ApplicationId),
			ARN:       &appArn,
			CreatedAt: aws.ToTime(application.CreatedAt),
			Type:      cfg.ResourceTypeEmrServerlessApplication,
		},
		Application: application,
	}
}

func (e Application) GetName() string {
	return aws.ToString(e.Application.Name)
}

func (e Application) GetTags() map[string]string {
	return e.Application.Tags
}

func (e Application) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
