package eks

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *EksRepository) ListClustersAll() ([]Cluster, error) {
	return r.ListClustersByInput(&eks.ListClustersInput{})
}

func (r *EksRepository) ListClustersByInput(query *eks.ListClustersInput) ([]Cluster, error) {
	log.Debug().
		Str("type", cfg.ResourceTypeToString(types.ResourceTypeECSCluster)).
		Msg("[EksRepository.ListClustersByInput] searching for clusters")

	start := time.Now()
	var clusters []Cluster

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("ListClusters", types.ResourceTypeEKSCluster)).
			Inc()
	}

	clustersOutput, err := r.client.EKS().ListClusters(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("ListClusters", types.ResourceTypeEKSCluster)).
				Inc()
		}

		return clusters, errors.New(err)
	}

	for _, clusterName := range clustersOutput.Clusters {

		eksCluster, err := r.client.EKS().DescribeCluster(r.ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
		if err != nil {
			log.Error().Err(err).
				Str("cluster", clusterName).
				Msg("[EksRepository.DescribeResources] failed to fetch EKS Cluster details")

			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeCluster", types.ResourceTypeEKSCluster)).
					Inc()
			}

			continue
		}

		cluster := NewCluster(r.client, eksCluster.Cluster)
		clusters = append(clusters, cluster)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListClusters", types.ResourceTypeEKSCluster)).
			Add(float64(len(clusters)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListClustersByInput", types.ResourceTypeEKSCluster)).
			Observe(time.Since(start).Seconds())
	}

	return clusters, nil
}
