package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *EcsRepository) ListClustersAll() ([]Cluster, error) {
	return r.ListClustersByInput(&ecs.ListClustersInput{})
}

func (r *EcsRepository) ListClustersByInput(query *ecs.ListClustersInput) ([]Cluster, error) {
	log.Debug().
		Str("type", cfg.ResourceTypeToString(types.ResourceTypeECSCluster)).
		Msg("[EcsRepository.ListClustersByInput] searching for clusters")

	start := time.Now()
	var clusters []Cluster

	p := ecs.NewListClustersPaginator(r.client.ECS(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListClusters", types.ResourceTypeECSCluster)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListClusters", types.ResourceTypeECSCluster)).
					Inc()
			}

			return clusters, errors.New(err)
		}

		// list clusters
		ecsClusters, err := r.client.ECS().DescribeClusters(
			r.ctx,
			&ecs.DescribeClustersInput{Clusters: resp.ClusterArns},
		)

		for _, ecsCluster := range ecsClusters.Clusters {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequests.
					With(r.promLabels("DescribeClusters", types.ResourceTypeECSCluster)).
					Inc()
			}

			if err != nil {
				log.Error().Err(err).
					Str("cluster", aws.ToString(ecsCluster.ClusterArn)).
					Msg("[EcsRepository.DescribeResources] failed to fetch EMR CLuster details")

				if metrics.AwsMetricsEnabled {
					metrics.AwsApiRequestErrors.
						With(r.promLabels("DescribeClusters", types.ResourceTypeECSCluster)).
						Inc()
				}

				continue
			}

			cluster := NewCluster(r.client, ecsCluster)
			clusters = append(clusters, cluster)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListClusters", types.ResourceTypeECSCluster)).
			Add(float64(len(clusters)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListClustersByInput", types.ResourceTypeECSCluster)).
			Observe(time.Since(start).Seconds())
	}

	return clusters, nil
}
