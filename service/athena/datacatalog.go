package athena

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"time"
)

type DataCatalogList struct {
	Items []DataCatalog
}

type DataCatalog struct {
	service.AbstractResource
	types.DataCatalog
	Tags []types.Tag
}

func NewDataCatalog(client AwsClient, dataCatalog types.DataCatalog, tags []types.Tag) DataCatalog {
	return DataCatalog{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(dataCatalog.Name),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "athena", "", dataCatalog.Name),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeAthenaDataCatalog,
		},
		DataCatalog: dataCatalog,
		Tags:        tags,
	}
}

func (e DataCatalog) GetName() string {
	return aws.ToString(e.Name)
}

func (e DataCatalog) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e DataCatalog) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
