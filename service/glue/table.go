package glue

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type TableList struct {
	Items []Table
}

type Table struct {
	service.AbstractResource
	types.Table
	Tags map[string]string
}

func NewTable(client AwsClient, table types.Table, tags map[string]string) Table {
	arn := helper.BuildArn(client.GetAccountID(), client.GetRegion(), "glue", fmt.Sprintf("table/%s", aws.ToString(table.DatabaseName)), table.Name)

	return Table{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(table.Name),
			ARN:       arn,
			CreatedAt: aws.ToTime(table.CreateTime),
			Type:      cfg.ResourceTypeGlueTable,
		},
		Table: table,
		Tags:  tags,
	}
}

func (e Table) GetName() string {
	return aws.ToString(e.Table.Name)
}

func (e Table) GetTags() map[string]string {
	return e.Tags
}

func (e Table) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
