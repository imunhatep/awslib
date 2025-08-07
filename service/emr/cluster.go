package emr

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/emr/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
)

type ClusterList struct {
	Items []Cluster
}

type Cluster struct {
	service.AbstractResource
	*types.Cluster
}

func NewCluster(client AwsClient, cluster *types.Cluster) Cluster {
	clusterArn, _ := arn.Parse(aws.ToString(cluster.ClusterArn))

	return Cluster{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(cluster.Id),
			ARN:       &clusterArn,
			CreatedAt: aws.ToTime(cluster.Status.Timeline.CreationDateTime),
			Type:      cfg.ResourceTypeEmrCluster,
		},
		Cluster: cluster,
	}
}

func (e Cluster) GetName() string {
	return aws.ToString(e.Cluster.Name)
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
