package dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/imunhatep/awslib/service"
)

type TableList struct {
	Items []Table
}

type Table struct {
	service.AbstractResource
	*types.TableDescription
	Tags []types.Tag
}

func NewTable(client AwsClient, table *types.TableDescription, tags []types.Tag) Table {
	dbArn, _ := arn.Parse(aws.ToString(table.TableArn))

	return Table{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(table.TableId),
			ARN:       &dbArn,
			CreatedAt: aws.ToTime(table.CreationDateTime),
			Type:      cfg.ResourceTypeTable,
		},
		TableDescription: table,
		Tags:             tags,
	}
}

func (e Table) GetName() string {
	return aws.ToString(e.TableDescription.TableName)
}

func (e Table) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Table) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
