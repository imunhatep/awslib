package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/imunhatep/awslib/service"
	"time"
)

type ClusterList struct {
	Items []Cluster
}

type Cluster struct {
	service.AbstractResource
	types.Cluster
}

func NewCluster(client AwsClient, cluster types.Cluster) Cluster {
	clusterArn, _ := arn.Parse(aws.ToString(cluster.ClusterArn))

	return Cluster{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(cluster.ClusterName),
			ARN:       &clusterArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeECSCluster,
		},
		Cluster: cluster,
	}
}

func (e Cluster) GetName() string {
	return aws.ToString(e.Cluster.ClusterName)
}

func (e Cluster) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Cluster.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Cluster) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
