package glue

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type DatabaseList struct {
	Items []Database
}

type Database struct {
	service.AbstractResource
	types.Database
	Tags map[string]string
}

func NewDatabase(client AwsClient, database types.Database, tags map[string]string) Database {
	return Database{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(database.Name),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "glue", "database/", database.Name),
			CreatedAt: aws.ToTime(database.CreateTime),
			Type:      cfg.ResourceTypeGlueDatabase,
		},
		Database: database,
		Tags:     tags,
	}
}

func (e Database) GetName() string {
	return aws.ToString(e.Database.Name)
}

func (e Database) GetTags() map[string]string {
	return e.Tags
}

func (e Database) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
