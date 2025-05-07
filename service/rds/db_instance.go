package rds

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/imunhatep/awslib/service"
)

func init() {
	gob.Register(DbInstance{})
}

type DbInstanceList struct {
	Items []DbInstance
}

type DbInstance struct {
	service.AbstractResource
	types.DBInstance
}

func NewDbInstance(client AwsClient, db types.DBInstance) DbInstance {
	dbArn, _ := arn.Parse(aws.ToString(db.DBInstanceArn))

	return DbInstance{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(db.DBInstanceIdentifier),
			ARN:       &dbArn,
			CreatedAt: aws.ToTime(db.InstanceCreateTime),
			Type:      cfg.ResourceTypeDBInstance,
		},
		DBInstance: db,
	}
}

func (e DbInstance) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	// if no name tag is found and DBInstanceIdentifier is not nil, return it
	if e.DBInstanceIdentifier != nil {
		return aws.ToString(e.DBInstanceIdentifier)
	}

	if e.ARN != nil {
		return e.ARN.Resource
	}

	// fallback to DB name
	return e.GetDbName()
}

func (e DbInstance) GetDbName() string {
	return aws.ToString(e.DBName)
}

func (e DbInstance) GetEngine() string {
	return aws.ToString(e.DBInstance.Engine)
}

func (e DbInstance) GetEngineVersion() string {
	return aws.ToString(e.DBInstance.EngineVersion)
}

func (e DbInstance) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.TagList {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e DbInstance) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
