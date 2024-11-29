package eks

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/imunhatep/awslib/service"
)

type ClusterList struct {
	Items []Cluster
}

type Cluster struct {
	service.AbstractResource
	*types.Cluster
}

func init() {
	gob.Register(Cluster{})
}

func NewCluster(client AwsClient, cluster *types.Cluster) Cluster {
	clusterArn, _ := arn.Parse(aws.ToString(cluster.Arn))

	return Cluster{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(cluster.Name),
			ARN:       &clusterArn,
			CreatedAt: aws.ToTime(cluster.CreatedAt),
			Type:      cfg.ResourceTypeEKSCluster,
		},
		Cluster: cluster,
	}
}

func (e Cluster) GetName() string {
	return aws.ToString(e.Cluster.Name)
}

func (e Cluster) GetTags() map[string]string {
	tags := make(map[string]string)
	for key, value := range e.Cluster.Tags {
		tags[key] = value
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
